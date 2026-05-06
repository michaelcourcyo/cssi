# CSSI

**CSSI** is a CSI (Container Storage Interface) driver that provisions Kubernetes volumes backed by NFS exports, where each export is served from a dedicated **LVM Logical Volume (LV)** on a Linux server.

The combination of NFS (for shared filesystem semantics) with LVM (for block-level features) enables **consistent, space-efficient snapshots** of every volume — something a plain NFS export cannot provide.

## Why CSSI?

Most NFS-based CSI drivers serve volumes as subdirectories of a single export. This is simple, but it makes per-volume snapshotting impossible: NFS has no native snapshot concept, and a directory has no underlying block device to snapshot.

CSSI takes a different approach:

- One **Volume Group (VG)** is built from the raw devices attached to a Linux server.
- Each PVC requested through the driver becomes its **own LV** with its own filesystem, exported individually over NFS.
- Snapshots are taken at the **LVM layer**, giving point-in-time, crash-consistent copies of any volume on demand — independent of the workload using it.

This makes CSSI a good fit for backup integrations (e.g. CSI snapshot-aware tools) on top of an NFS-style shared-filesystem experience.

## Architecture

CSSI is split into two cooperating components:

```
   Kubernetes                                      Linux storage server
  ┌────────────┐    gRPC / HTTP    ┌───────────────────────────────────┐
  │ CSI driver │ ────────────────► │  CSSI server                      │
  │  (cssi)    │                   │  ├─ LVM (single VG over raw disks)│
  │            │ ◄──────────────── │  ├─ mkfs (per storage-class)      │
  └─────┬──────┘                   │  └─ NFS exports                   │
        │ NFS mount                └───────────────┬───────────────────┘
        ▼                                          │
    Pod / Workload  ◄────────────── NFS ───────────┘
```

### 1. Server component

Runs on a Linux host with one or more raw block devices.

- On startup, the raw devices are assembled into a **single Volume Group**.
- Exposes an API consumed by the CSI driver. For each volume request it:
  1. Creates an **LV** of the requested size in the VG.
  2. Formats the LV with the **filesystem type** specified by the storage class parameters (e.g. `ext4`, `xfs`).
  3. Mounts the new filesystem locally.
  4. Adds an **NFS export** for that mount and returns the export path to the caller.
- Handles deletion (unexport, unmount, `lvremove`) and snapshots (`lvcreate --snapshot`) symmetrically.

### 2. CSI driver

A standard Kubernetes CSI driver that implements the controller and node services.

- On `CreateVolume`, it calls the server component to provision the LV/filesystem/export and records the returned NFS endpoint.
- On `NodePublishVolume`, it mounts the NFS export into the pod's target path.
- On `CreateSnapshot` / `DeleteSnapshot` / `DeleteVolume`, it forwards the request to the server, which performs the LVM-level operation.

## Storage class parameters

The filesystem layout is driven by the `StorageClass`. Typical parameters include:

| Parameter      | Description                                                      |
|----------------|------------------------------------------------------------------|
| `fsType`       | Filesystem to create on the LV (`ext4`, `xfs`, ...).             |
| `mkfsOptions`  | Extra options passed to `mkfs`.                                  |
| `mountOptions` | NFS mount options applied on the node.                           |

(See the example manifests under `deploy/` for the full, current list.)

## Snapshots

Because every volume is its own LV, snapshots are simply LVM snapshots:

1. A `VolumeSnapshot` is created in Kubernetes.
2. The CSI driver asks the server to run `lvcreate --snapshot` against the source LV.
3. The snapshot can later be restored into a new PVC, which the server materializes as a new LV cloned from the snapshot and re-exports over NFS.

This gives consistent, near-instant snapshots without quiescing the workload at the filesystem level.

## Repository layout

> The project is in early development; this section will be expanded as components land.

- `server/` — CSSI server (LVM + NFS management daemon).
- `driver/` — CSI driver (controller + node plugin).
- `deploy/` — Example Kubernetes manifests (StorageClass, VolumeSnapshotClass, RBAC, DaemonSet, Deployment).

## Requirements

- A Linux host with one or more raw block devices dedicated to the CSSI VG.
- `lvm2`, `nfs-kernel-server`, and the relevant `mkfs.*` userland tools.
- A Kubernetes cluster (1.24+) with the `VolumeSnapshot` CRDs installed if snapshot support is desired.

## Status

Experimental. APIs, on-disk layout, and storage-class parameters may still change.
