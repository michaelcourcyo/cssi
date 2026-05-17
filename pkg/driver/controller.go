package driver

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	csi "github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/michaelcourcyo/cssi/pkg/client"
)

// ControllerService implements the CSI Controller gRPC service.
//
// It is reached by the external-provisioner sidecar over a Unix socket in
// response to PVC events. CreateVolume forwards the request to the CSSI
// server identified by the StorageClass parameters, which owns the LVM
// volume group and the NFS exports.
//
// CreateVolume must be idempotent on its Name input (the sidecar may
// retry). The CSSI server enforces the idempotency on its side; this
// service just passes Name through unchanged.
//
// See plumbing.md section 2 ("PVC -> PV provisioning workflow"):
// ../../plumbing.md#2-pvc---pv-provisioning-workflow
type ControllerService struct {
	csi.UnimplementedControllerServer
}

// NewControllerService returns a ControllerService. The CSSI server's
// address is read from the StorageClass parameters at CreateVolume time,
// so the constructor takes no arguments.
func NewControllerService() *ControllerService {
	return &ControllerService{}
}

// StorageClass parameter keys the driver reads.
const (
	// ParamServer is the host (or IP) of the CSSI server that this
	// StorageClass provisions against.
	ParamServer = "server"
	// ParamPort is the TCP port the CSSI server listens on.
	ParamPort = "port"
)

// CreateVolume is the CSI Controller CreateVolume RPC. It opens a gRPC
// connection to the CSSI server identified by the StorageClass parameters
// (req.Parameters) and asks it to provision a new volume.
//
// req.Name is the idempotency key; the sidecar passes "pvc-<PVC.UID>"
// here and retries with the same name on transient failures.
//
// Validation errors come back with codes.InvalidArgument. Conflicts
// (same name, different size or fs) come back with codes.AlreadyExists.
// Anything else surfaces as codes.Internal.
func (c *ControllerService) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	name := req.GetName()
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: Name is required (idempotency key)")
	}

	var size int64
	if r := req.GetCapacityRange(); r != nil {
		size = r.GetRequiredBytes()
	}
	if size <= 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: capacity_range.required_bytes must be > 0")
	}

	fsType := ""
	for _, vc := range req.GetVolumeCapabilities() {
		if m := vc.GetMount(); m != nil && m.GetFsType() != "" {
			fsType = m.GetFsType()
			break
		}
	}
	if fsType == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume: a mount volume capability with fs_type is required")
	}

	host, port, err := parseServerEndpoint(req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	cli, err := client.NewClient(ctx, host, port)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "client: %v", err)
	}
	defer cli.Close()

	res, err := cli.CreateVolume(ctx, name, size, fsType)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "cssi server CreateVolume: %v", err)
	}
	if !res.Success {
		// The CSSI protocol carries application-level failures in the
		// body as Success=false with a Reason. Conflict (same name,
		// different params) is the only such case today; map it to the
		// CSI-idiomatic AlreadyExists.
		return nil, status.Errorf(codes.AlreadyExists, "cssi server refused CreateVolume: %s", res.Reason)
	}
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      res.VolumeHandle,
			CapacityBytes: size,
			VolumeContext: req.GetParameters(),
		},
	}, nil
}

// parseServerEndpoint extracts the server host and port from a
// StorageClass parameter map, returning a descriptive error if either is
// missing or malformed.
func parseServerEndpoint(params map[string]string) (string, int, error) {
	host := params[ParamServer]
	if host == "" {
		return "", 0, errors.New("StorageClass parameter \"server\" is required")
	}
	rawPort := params[ParamPort]
	if rawPort == "" {
		return "", 0, errors.New("StorageClass parameter \"port\" is required")
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		return "", 0, fmt.Errorf("StorageClass parameter \"port\" is not an integer: %q", rawPort)
	}
	return host, port, nil
}
