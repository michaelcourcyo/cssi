// Package server implements the CSSI host-side daemon.
//
// The server owns one LVM Volume Group built from raw devices on a Linux
// host. For each provisioning request from the CSI driver it:
//
//  1. Creates a Logical Volume of the requested size in the VG.
//  2. Formats it with the requested filesystem (ext4, xfs, ...).
//  3. Mounts the filesystem under ExportRoot.
//  4. Adds an NFS export and returns its endpoint to the caller.
package server

import (
	"errors"
	"log"
)

// Config carries the runtime parameters for the CSSI server.
type Config struct {
	// ListenAddr is the address the API listens on.
	ListenAddr string
	// VGName is the LVM Volume Group the server manages.
	VGName string
	// ExportRoot is the directory under which provisioned LV filesystems
	// are mounted and exported.
	ExportRoot string
}

// Server is the CSSI storage server.
type Server struct {
	cfg Config
}

// New returns a Server configured with cfg.
func New(cfg Config) *Server {
	return &Server{cfg: cfg}
}

// Run starts the API listener and blocks until it exits.
func (s *Server) Run() error {
	log.Printf("cssi-server starting: listen=%s vg=%s exportRoot=%s",
		s.cfg.ListenAddr, s.cfg.VGName, s.cfg.ExportRoot)
	// TODO: open API listener, wire LVM and NFS managers, and Serve().
	return errors.New("cssi-server: API listener not implemented")
}
