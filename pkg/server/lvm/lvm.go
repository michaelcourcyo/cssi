// Package lvm wraps the LVM userland tools (lvcreate, lvremove, lvs, ...) used
// by the CSSI server to manage Logical Volumes and snapshots.
package lvm

// Manager performs LVM operations against a single Volume Group.
type Manager struct {
	VG string
}

// New returns a Manager bound to the named Volume Group.
func New(vg string) *Manager { return &Manager{VG: vg} }
