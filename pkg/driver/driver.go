// Package driver implements the CSSI CSI driver.
//
// It is split across:
//   - identity.go   : CSI Identity service
//   - controller.go : CSI Controller service (CreateVolume, snapshots, ...)
//   - node.go       : CSI Node service (NodePublishVolume mounts the NFS export)
//
// For the end-to-end Kubernetes plumbing (driver registration, the
// PVC -> PV workflow, and how the standard kubernetes-csi sidecars wire
// up to this driver), see ../../plumbing.md.
package driver

import (
	"errors"
	"log"
)

// Config carries the runtime parameters for the CSI driver.
type Config struct {
	// Endpoint is the CSI gRPC socket, e.g. "unix:///csi/csi.sock".
	Endpoint string
	// NodeID is the identifier of the node this driver instance runs on.
	NodeID string
	// ServerAddr is the address of the CSSI server (host:port).
	ServerAddr string
}

// Driver wires the CSI services together.
type Driver struct {
	cfg Config
}

// New returns a Driver configured with cfg.
func New(cfg Config) *Driver {
	return &Driver{cfg: cfg}
}

// Run starts the gRPC server and blocks until it exits.
func (d *Driver) Run() error {
	log.Printf("cssi-driver starting: endpoint=%s node=%s server=%s",
		d.cfg.Endpoint, d.cfg.NodeID, d.cfg.ServerAddr)
	// TODO: register Identity, Controller, and Node services on a gRPC server
	// listening at d.cfg.Endpoint, and Serve().
	return errors.New("cssi-driver: gRPC server not implemented")
}
