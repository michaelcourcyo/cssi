package driver_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/michaelcourcyo/cssi/pkg/driver"
	"github.com/michaelcourcyo/cssi/pkg/server"
)

// This test exercises the CreateVolume call all the way from the
// external-provisioner sidecar down to the LVM manager, over two real
// gRPC hops:
//
//	externalProvisionerMock                      (CSI gRPC client)
//	  --[ unix socket ]--->
//	    cssi-driver (driver.ControllerService)   (real, CSI gRPC server)
//	      --[ TCP loopback ]--->
//	        cssi-server (storage service)        (real, CSSI gRPC server)
//	          --[ Go call ]--->
//	            fakeLVMManager                   (mock)
//
// Two pieces are mocked:
//
//   - externalProvisionerMock plays the role of the external-provisioner
//     CSI sidecar. It is a real csi.ControllerClient dialed against the
//     driver's Unix socket — same wire protocol the sidecar uses in
//     production.
//
//   - fakeLVMManager stands in for *lvm.Manager. It mirrors the real
//     manager's idempotency contract so the chain can be observed
//     end-to-end. The two implementations are intentionally separate so
//     they can diverge as the real LVM stack grows beyond what the test
//     needs to assert on.

// externalProvisionerMock is the test stand-in for the external-provisioner
// sidecar. Internally it is a CSI Controller gRPC client.
type externalProvisionerMock struct {
	conn   *grpc.ClientConn
	client csi.ControllerClient
}

func newExternalProvisionerMock(t *testing.T, socketPath string) *externalProvisionerMock {
	t.Helper()
	// "unix:" + absolute path gives the grpc-go built-in unix resolver
	// an absolute path to dial.
	target := "unix:" + socketPath
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial driver socket: %v", err)
	}
	return &externalProvisionerMock{
		conn:   conn,
		client: csi.NewControllerClient(conn),
	}
}

// ProvisionVolume mirrors what the sidecar does on a PVC event: issue a
// CSI CreateVolume RPC against the driver.
func (m *externalProvisionerMock) ProvisionVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	return m.client.CreateVolume(ctx, req)
}

func (m *externalProvisionerMock) Close() error {
	return m.conn.Close()
}

// fakeLVMRecord is the test-side equivalent of lvm.volumeRecord.
type fakeLVMRecord struct {
	sizeBytes int64
	fsType    string
	handle    string
}

// fakeLVMManager mimics lvm.Manager for tests. It records every call and
// applies the same idempotency rules as the real manager, but does no
// shell-out and stores its state in memory.
type fakeLVMManager struct {
	vg string

	mu      sync.Mutex
	calls   int
	volumes map[string]fakeLVMRecord
}

func newFakeLVMManager(vg string) *fakeLVMManager {
	return &fakeLVMManager{vg: vg, volumes: map[string]fakeLVMRecord{}}
}

var errFakeConflict = errors.New("fakelvm: volume exists with different parameters")

func (m *fakeLVMManager) CreateVolume(name string, sizeBytes int64, fsType string) (string, error) {
	if m.vg == "" {
		return "", errors.New("fakelvm: volume group is empty")
	}
	if name == "" {
		return "", errors.New("fakelvm: name is empty")
	}
	if sizeBytes <= 0 {
		return "", fmt.Errorf("fakelvm: size_bytes must be > 0, got %d", sizeBytes)
	}
	if fsType == "" {
		return "", errors.New("fakelvm: fs_type is empty")
	}

	lvName := "cssi-" + name
	handle := m.vg + "/" + lvName

	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	if existing, ok := m.volumes[lvName]; ok {
		if existing.sizeBytes != sizeBytes || existing.fsType != fsType {
			return "", fmt.Errorf("%w: name=%q have(size=%d fs=%q) want(size=%d fs=%q)",
				errFakeConflict, name, existing.sizeBytes, existing.fsType, sizeBytes, fsType)
		}
		return existing.handle, nil
	}

	m.volumes[lvName] = fakeLVMRecord{
		sizeBytes: sizeBytes,
		fsType:    fsType,
		handle:    handle,
	}
	return handle, nil
}

func (m *fakeLVMManager) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// startTestCSSIServer brings up a real CSSI gRPC server on a loopback TCP
// port backed by the given LVM manager and returns the host/port the
// driver should target plus a function to stop the server.
func startTestCSSIServer(t *testing.T, mgr server.LVMManager, vg string) (host string, port int, stop func()) {
	t.Helper()

	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cssi-server listen: %v", err)
	}

	srv := server.NewWithLVM(server.Config{VGName: vg}, mgr)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	hostStr, portStr, err := net.SplitHostPort(lis.Addr().String())
	if err != nil {
		_ = lis.Close()
		t.Fatalf("split host port: %v", err)
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		_ = lis.Close()
		t.Fatalf("parse port: %v", err)
	}

	stop = func() {
		_ = lis.Close()
		select {
		case <-serveErr:
		case <-time.After(2 * time.Second):
			t.Errorf("cssi-server did not exit within 2s")
		}
	}
	return hostStr, p, stop
}

// startTestDriver brings up a real cssi-driver gRPC server on a Unix
// socket inside a tempdir owned by the test, and returns the socket path
// plus a function to stop the server.
func startTestDriver(t *testing.T) (socketPath string, stop func()) {
	t.Helper()

	// t.TempDir() embeds the (long) test name in the path, which on
	// macOS routinely overshoots the 104-byte sun_path limit for Unix
	// sockets. Use a short os.MkdirTemp dir instead.
	dir, err := os.MkdirTemp("", "cssi")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	socketPath = filepath.Join(dir, "csi.sock")
	var lc net.ListenConfig
	lis, err := lc.Listen(t.Context(), "unix", socketPath)
	if err != nil {
		t.Fatalf("driver listen on %s: %v", socketPath, err)
	}

	d := driver.New(driver.Config{NodeID: "test-node"})
	serveErr := make(chan error, 1)
	go func() { serveErr <- d.Serve(lis) }()

	stop = func() {
		_ = lis.Close()
		select {
		case <-serveErr:
		case <-time.After(2 * time.Second):
			t.Errorf("cssi-driver did not exit within 2s")
		}
	}
	return socketPath, stop
}

// buildCreateVolumeRequest assembles a CSI CreateVolumeRequest the way the
// external-provisioner would: idempotency name, capacity, a single mount
// VolumeCapability with the requested fs_type, and the StorageClass
// parameters used to reach the CSSI server.
func buildCreateVolumeRequest(name string, sizeBytes int64, fsType, host string, port int) *csi.CreateVolumeRequest {
	return &csi.CreateVolumeRequest{
		Name:          name,
		CapacityRange: &csi.CapacityRange{RequiredBytes: sizeBytes},
		VolumeCapabilities: []*csi.VolumeCapability{{
			AccessType: &csi.VolumeCapability_Mount{
				Mount: &csi.VolumeCapability_MountVolume{FsType: fsType},
			},
			AccessMode: &csi.VolumeCapability_AccessMode{
				Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			},
		}},
		Parameters: map[string]string{
			driver.ParamServer: host,
			driver.ParamPort:   strconv.Itoa(port),
		},
	}
}

func TestCreateVolume_EndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const vg = "test-vg"
	fake := newFakeLVMManager(vg)
	cssiHost, cssiPort, stopCSSI := startTestCSSIServer(t, fake, vg)
	defer stopCSSI()

	socketPath, stopDriver := startTestDriver(t)
	defer stopDriver()

	provisioner := newExternalProvisionerMock(t, socketPath)
	defer provisioner.Close()

	req := buildCreateVolumeRequest("pvc-abc123", 1<<30, "ext4", cssiHost, cssiPort)

	resp, err := provisioner.ProvisionVolume(ctx, req)
	if err != nil {
		t.Fatalf("ProvisionVolume: %v", err)
	}
	if resp.GetVolume() == nil {
		t.Fatalf("response.Volume is nil")
	}
	wantHandle := vg + "/cssi-" + req.Name
	if got := resp.GetVolume().GetVolumeId(); got != wantHandle {
		t.Errorf("VolumeId = %q, want %q", got, wantHandle)
	}
	if got := resp.GetVolume().GetCapacityBytes(); got != req.CapacityRange.RequiredBytes {
		t.Errorf("CapacityBytes = %d, want %d", got, req.CapacityRange.RequiredBytes)
	}
	if got := fake.callCount(); got != 1 {
		t.Errorf("fake LVM call count = %d, want 1", got)
	}
}

func TestCreateVolume_EndToEnd_RetryIsIdempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const vg = "test-vg"
	fake := newFakeLVMManager(vg)
	cssiHost, cssiPort, stopCSSI := startTestCSSIServer(t, fake, vg)
	defer stopCSSI()

	socketPath, stopDriver := startTestDriver(t)
	defer stopDriver()

	provisioner := newExternalProvisionerMock(t, socketPath)
	defer provisioner.Close()

	req := buildCreateVolumeRequest("pvc-retry", 2<<30, "xfs", cssiHost, cssiPort)

	first, err := provisioner.ProvisionVolume(ctx, req)
	if err != nil {
		t.Fatalf("first ProvisionVolume: %v", err)
	}
	second, err := provisioner.ProvisionVolume(ctx, req)
	if err != nil {
		t.Fatalf("retry ProvisionVolume: %v", err)
	}
	if first.GetVolume().GetVolumeId() != second.GetVolume().GetVolumeId() {
		t.Errorf("retry returned different VolumeId: first=%q second=%q",
			first.GetVolume().GetVolumeId(), second.GetVolume().GetVolumeId())
	}
	if got := fake.callCount(); got != 2 {
		t.Errorf("fake LVM call count = %d, want 2 (both retries reach LVM)", got)
	}
}

func TestCreateVolume_EndToEnd_ConflictingParamsFail(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const vg = "test-vg"
	fake := newFakeLVMManager(vg)
	cssiHost, cssiPort, stopCSSI := startTestCSSIServer(t, fake, vg)
	defer stopCSSI()

	socketPath, stopDriver := startTestDriver(t)
	defer stopDriver()

	provisioner := newExternalProvisionerMock(t, socketPath)
	defer provisioner.Close()

	base := buildCreateVolumeRequest("pvc-conflict", 1<<30, "ext4", cssiHost, cssiPort)
	if _, err := provisioner.ProvisionVolume(ctx, base); err != nil {
		t.Fatalf("initial ProvisionVolume: %v", err)
	}

	// Same name, different size: the CSSI server reports Success=false
	// on the wire, which the driver surfaces as AlreadyExists.
	conflicting := buildCreateVolumeRequest(base.Name, 4<<30, "ext4", cssiHost, cssiPort)
	_, err := provisioner.ProvisionVolume(ctx, conflicting)
	if err == nil {
		t.Fatalf("expected conflict error, got nil")
	}
	if got, want := status.Code(err), codes.AlreadyExists; got != want {
		t.Errorf("conflict error code = %s, want %s (err=%v)", got, want, err)
	}
}
