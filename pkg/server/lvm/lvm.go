// Package lvm wraps the LVM userland tools (lvcreate, lvremove, lvs, ...) used
// by the CSSI server to manage Logical Volumes and snapshots.
package lvm

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Manager performs LVM operations against a single Volume Group.
type Manager struct {
	VG string

	mu      sync.Mutex
	volumes map[string]volumeRecord // keyed by sanitized LV name
}

// volumeRecord tracks what an LV was created with so a repeat
// CreateVolume call can be detected as either a benign retry (same name,
// same params) or a conflict (same name, different params).
type volumeRecord struct {
	sizeBytes int64
	fsType    string
	handle    string
}

// New creates a Manager bound to the named volume group.
func New(vg string) *Manager {
	return &Manager{VG: vg, volumes: map[string]volumeRecord{}}
}

// ErrConflict is returned by CreateVolume when an LV with the requested
// name already exists but has different parameters from the new request.
var ErrConflict = errors.New("lvm: volume exists with different parameters")

// CreateVolume carves a new Logical Volume of sizeBytes out of the
// manager's Volume Group and formats it with the given filesystem.
//
// The name parameter is the idempotency key: the LV is deterministically
// named "cssi-<name>" inside the VG, and:
//
//   - If no LV with that name exists, it is created and its handle is
//     returned.
//   - If an LV with that name already exists with matching size and
//     filesystem, the existing handle is returned (success — this is the
//     happy retry case).
//   - If an LV with that name exists but the recorded size or filesystem
//     differ from the new request, ErrConflict is returned.
//
// The shell-out to lvcreate / mkfs / exportfs is not implemented yet; this
// is a stub that records each created volume in an in-memory map so the
// idempotency semantics above can already be observed end-to-end.
func (m *Manager) CreateVolume(name string, sizeBytes int64, fsType string) (string, error) {
	if m.VG == "" {
		return "", errors.New("lvm: volume group is empty")
	}
	if name == "" {
		return "", errors.New("lvm: name is empty")
	}
	if sizeBytes <= 0 {
		return "", fmt.Errorf("lvm: size_bytes must be > 0, got %d", sizeBytes)
	}
	if fsType == "" {
		return "", errors.New("lvm: fs_type is empty")
	}

	lvName := "cssi-" + name
	handle := m.VG + "/" + lvName
	if strings.Contains(name, "/") {
		return "", errors.New("lvm: name must not contain '/'")
	}
	if strings.TrimSpace(name) != name {
		return "", errors.New("lvm: name must not have leading/trailing whitespace")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.volumes[lvName]; ok {
		if existing.sizeBytes != sizeBytes || existing.fsType != fsType {
			return "", fmt.Errorf("%w: name=%q have(size=%d fs=%q) want(size=%d fs=%q)",
				ErrConflict, name, existing.sizeBytes, existing.fsType, sizeBytes, fsType)
		}
		return existing.handle, nil
	}

	// TODO: lvcreate -n <lvName> -L <sizeBytes>B <m.VG>
	//       mkfs.<fsType> /dev/<m.VG>/<lvName>
	//       mount + exportfs
	m.volumes[lvName] = volumeRecord{
		sizeBytes: sizeBytes,
		fsType:    fsType,
		handle:    handle,
	}
	return handle, nil
}
