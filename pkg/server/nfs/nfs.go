// Package nfs manages the NFS exports backing CSSI volumes.
//
// It edits /etc/exports (or a CSSI-owned drop-in) and runs `exportfs` to
// publish each LV's mount point.
package nfs

// Exporter manages NFS exports rooted under a single directory.
type Exporter struct {
	Root string
}

// New creates an Exporter rooted at the provided directory.
// The Exporter places NFS exports under the specified root.
func New(root string) *Exporter { return &Exporter{Root: root} }
