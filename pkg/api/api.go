// Package api defines the wire types shared between the CSI driver and the
// CSSI server. Once a transport is chosen (gRPC, HTTP+JSON, ...) this is
// where the request/response types — or the .proto definitions and their
// generated code — should live.
package api

// CreateVolumeRequest is what the CSI driver sends to the server when a
// PVC needs a backing LV + NFS export.
type CreateVolumeRequest struct {
	Name         string            // unique CSI volume name
	SizeBytes    int64             // requested capacity
	FSType       string            // filesystem to create (ext4, xfs, ...)
	MkfsOptions  []string          // extra options for mkfs
	StorageClass map[string]string // raw StorageClass parameters
}

// CreateVolumeResponse is the reply from the server once the LV exists,
// has a filesystem, and is exported.
type CreateVolumeResponse struct {
	VolumeID  string // server-side ID (e.g. LV name)
	NFSServer string // host or IP serving the export
	NFSPath   string // exported path
	SizeBytes int64  // actual provisioned size
}
