// Package lvm wraps the LVM userland tools (lvcreate, lvremove, lvs, ...) used
// by the CSSI server to manage Logical Volumes and snapshots.
package lvm

// Manager performs LVM operations against a single Volume Group.
type Manager struct {
	VG string
}

// New creates a Manager bound to the named volume group.
// The returned Manager's VG field is set to the provided vg.
func New(vg string) *Manager { return &Manager{VG: vg} }
