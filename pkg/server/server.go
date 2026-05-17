// Package server implements the CSSI host-side daemon.
//
// The server owns one LVM Volume Group built from raw devices on a Linux
// host. For each provisioning request from the CSI driver it:
//
//  1. Creates a Logical Volume of the requested size in the VG.
//  2. Formats it with the requested filesystem (ext4, xfs, ...).
//  3. Mounts the filesystem and adds an NFS export for it.
//  4. Returns a volume handle so the driver can mount the volume later.
//
// The wire protocol is gRPC; see [proto/cssi/v1/cssi.proto] and the
// generated package at [pkg/api/v1].
package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"

	"google.golang.org/grpc"

	cssiv1 "github.com/michaelcourcyo/cssi/pkg/api/v1"
	"github.com/michaelcourcyo/cssi/pkg/server/lvm"
)

// Config carries the runtime parameters for the CSSI server.
//
// Per the design, the server is configured with exactly two inputs: the
// port it listens on, and the name of the LVM Volume Group it manages.
type Config struct {
	// Port is the TCP port the gRPC API listens on.
	Port int
	// VGName is the LVM Volume Group the server manages.
	VGName string
}

// LVMManager is the subset of the LVM stack the gRPC service depends on.
//
// It is satisfied by *lvm.Manager in production. Tests substitute a fake
// so the storage server can be exercised end-to-end without touching the
// host's volume group.
type LVMManager interface {
	CreateVolume(name string, sizeBytes int64, fsType string) (string, error)
}

// Server is the CSSI storage server.
type Server struct {
	cfg Config
	lvm LVMManager
}

// New creates a Server configured with the provided Config. The Server is
// backed by a real *lvm.Manager bound to cfg.VGName.
func New(cfg Config) *Server {
	return &Server{
		cfg: cfg,
		lvm: lvm.New(cfg.VGName),
	}
}

// NewWithLVM creates a Server with an explicit LVMManager. Intended for
// tests that want to substitute the real LVM stack with a fake.
func NewWithLVM(cfg Config, mgr LVMManager) *Server {
	return &Server{cfg: cfg, lvm: mgr}
}

// Run starts the gRPC API listener on cfg.Port and blocks until it exits.
func (s *Server) Run() error {
	addr := ":" + strconv.Itoa(s.cfg.Port)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	return s.Serve(lis)
}

// Serve registers the storage service on a fresh gRPC server bound to lis
// and blocks until it exits. Tests pass an ephemeral listener so they can
// drive the server over a real loopback connection.
func (s *Server) Serve(lis net.Listener) error {
	gs := grpc.NewServer()
	cssiv1.RegisterStorageServer(gs, &storageService{lvm: s.lvm})

	log.Printf("cssi-server listening on %s (vg=%s)", lis.Addr(), s.cfg.VGName)
	return gs.Serve(lis)
}

// storageService is the gRPC implementation of cssi.v1.Storage backed by
// an LVMManager.
type storageService struct {
	cssiv1.UnimplementedStorageServer
	lvm LVMManager
}

// CreateVolume carves an LV out of the configured VG, formats it, and
// returns a handle.
//
// On any failure the RPC still returns nil error and success=false with a
// human-readable reason, matching the design that puts the success bit on
// the message rather than relying on gRPC status codes.
func (s *storageService) CreateVolume(ctx context.Context, req *cssiv1.CreateVolumeRequest) (*cssiv1.CreateVolumeResponse, error) {
	handle, err := s.lvm.CreateVolume(req.GetName(), req.GetSizeBytes(), req.GetFsType())
	if err != nil {
		return &cssiv1.CreateVolumeResponse{
			Success: false,
			Reason:  err.Error(),
		}, nil
	}
	return &cssiv1.CreateVolumeResponse{
		Success:      true,
		VolumeHandle: handle,
	}, nil
}
