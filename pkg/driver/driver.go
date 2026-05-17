// Package driver implements the CSSI CSI driver.
//
// It is split across:
//   - identity.go   : CSI Identity service
//   - controller.go : CSI Controller service (CreateVolume, snapshots, ...)
//   - node.go       : CSI Node service (NodePublishVolume mounts the NFS export)
//
// The CSSI server's address and port are not configured globally on the
// driver. They are read from StorageClass.parameters on each
// CreateVolume call so that a single driver can front multiple servers.
//
// For the end-to-end Kubernetes plumbing (driver registration, the
// PVC -> PV workflow, and how the standard kubernetes-csi sidecars wire
// up to this driver), see ../../plumbing.md.
package driver

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"
)

// Config carries the runtime parameters for the CSI driver.
type Config struct {
	// Endpoint is the CSI gRPC socket, e.g. "unix:///csi/csi.sock".
	Endpoint string
	// NodeID is the identifier of the node this driver instance runs on.
	NodeID string
}

// Driver wires the CSI services together.
type Driver struct {
	cfg Config
}

// New returns a Driver configured with cfg.
func New(cfg Config) *Driver {
	return &Driver{cfg: cfg}
}

// Run parses cfg.Endpoint (which must be a unix:// URL), binds a Unix
// domain socket at that path, and serves the CSI gRPC plane until it
// exits.
func (d *Driver) Run() error {
	sock, err := parseUnixEndpoint(d.cfg.Endpoint)
	if err != nil {
		return err
	}
	// A leftover socket file from a previous (crashed) run would make
	// the bind fail. Remove it; net.Listen will recreate it.
	_ = os.Remove(sock)
	lis, err := net.Listen("unix", sock)
	if err != nil {
		return fmt.Errorf("listen %s: %w", sock, err)
	}
	return d.Serve(lis)
}

// Serve registers the CSI services on a fresh gRPC server bound to lis
// and blocks until it exits. Tests pass an ephemeral Unix-socket listener
// here so they can drive the driver over a real gRPC connection.
//
// Identity and Node are not registered yet (the CreateVolume slice is the
// first thing wired). They will be added as those services are filled in.
func (d *Driver) Serve(lis net.Listener) error {
	gs := grpc.NewServer()
	csi.RegisterControllerServer(gs, NewControllerService())
	log.Printf("cssi-driver listening on %s (node=%s)", lis.Addr(), d.cfg.NodeID)
	return gs.Serve(lis)
}

// parseUnixEndpoint expects a "unix:///absolute/path" URL and returns the
// path. The CSI spec only mandates unix sockets between the kubelet/
// sidecars and the driver; we don't support anything else.
func parseUnixEndpoint(ep string) (string, error) {
	if !strings.HasPrefix(ep, "unix://") {
		return "", fmt.Errorf("driver: only unix:// endpoints are supported (got %q)", ep)
	}
	p := strings.TrimPrefix(ep, "unix://")
	if p == "" {
		return "", fmt.Errorf("driver: unix endpoint missing path (got %q)", ep)
	}
	return p, nil
}
