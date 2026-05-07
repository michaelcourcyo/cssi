# CSSI Plumbing

Notes on how a CSI driver fits into Kubernetes. This document tracks the
end-to-end workflows the CSSI driver participates in, so we can refer back
to them as the implementation grows.

## Contents

- [1. CSI driver registration](#1-csi-driver-registration)
- [2. PVC -> PV provisioning workflow](#2-pvc---pv-provisioning-workflow)
- [3. The standard sidecars](#3-the-standard-sidecars)
- [4. How `external-provisioner` finds our PVCs](#4-how-external-provisioner-finds-our-pvcs)
- [5. Implications for the CSSI codebase](#5-implications-for-the-cssi-codebase)

---

## 1. CSI driver registration

A CSI driver does not register itself directly with the Kubernetes API.
Registration happens through a sidecar pattern with the kubelet on each
node, plus an optional cluster-level `CSIDriver` object.

### 1.1 The two planes

A CSI driver runs in two places:

- **Controller plane** (Deployment / StatefulSet): handles cluster-wide
  operations - `CreateVolume`, `DeleteVolume`, `ControllerPublishVolume`,
  snapshots, resize.
- **Node plane** (DaemonSet): runs on every node and handles
  `NodeStageVolume` / `NodePublishVolume` (mounting into pods).

### 1.2 Node registration via kubelet

On each node, the CSI driver pod runs alongside the
`node-driver-registrar` sidecar (from `kubernetes-csi`). This sidecar:

1. Calls the driver's `GetPluginInfo` over a Unix socket to learn the
   driver name (e.g. `cssi.mcourcy.com`).
2. Registers with kubelet via the plugin registration socket at
   `/var/lib/kubelet/plugins_registry/`.
3. Kubelet then knows: "for driver `cssi.mcourcy.com`, talk to the gRPC
   socket at `/var/lib/kubelet/plugins/cssi.mcourcy.com/csi.sock`."

This is what makes kubelet route Node-level CSI calls to the driver's
gRPC server.

### 1.3 Cluster-level `CSIDriver` object

A `CSIDriver` resource is also typically shipped:

```yaml
apiVersion: storage.k8s.io/v1
kind: CSIDriver
metadata:
  name: cssi.mcourcy.com
spec:
  attachRequired: false
  podInfoOnMount: true
  volumeLifecycleModes: [Persistent]
```

This tells Kubernetes whether the driver needs an attach step, whether
pod info should be passed on mount, etc.

### 1.4 Registration diagram

```text
[ kube-apiserver ]
       |
       | (watches PVC, VolumeAttachment, ...)
       |
[ controller pod ]                       [ node pod (DaemonSet) ]
  |- external-provisioner --gRPC--+        |- node-driver-registrar --> kubelet
  |- external-attacher ----gRPC---+        |- cssi driver (gRPC unix socket)
  |- cssi driver (gRPC) <---------+                   ^
                                                      |
                                              kubelet calls Node* RPCs
```

---

## 2. PVC -> PV provisioning workflow

End-to-end flow when a user creates a PVC referencing a StorageClass with
provisioner `cssi.mcourcy.com`.

### 2.1 Setup

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: cssi-fast
provisioner: cssi.mcourcy.com
parameters:
  type: ssd
reclaimPolicy: Delete
volumeBindingMode: Immediate     # or WaitForFirstConsumer
```

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-data
spec:
  storageClassName: cssi-fast
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 10Gi
```

### 2.2 Step-by-step

**1. PVC lands in etcd.** `kubectl apply` -> apiserver validates ->
PVC stored with `status.phase=Pending`. No PV yet.

**2. PV controller (`kube-controller-manager`) sees the new PVC.**
It looks for an existing matching PV. None exists. It reads the
StorageClass and finds `provisioner: cssi.mcourcy.com`. Since this is
not an in-tree provisioner, the PV controller does not call our driver.
Instead it stamps an annotation on the PVC:

```yaml
volume.kubernetes.io/storage-provisioner: cssi.mcourcy.com
```

(Older form: `volume.beta.kubernetes.io/storage-provisioner`. Both are
recognized.) This annotation is the signal that an external provisioner
must handle the PVC.

**3. `external-provisioner` sidecar picks it up.** Running next to the
CSI driver in the controller Deployment, it watches PVCs filtered by
that annotation. When it sees `my-data`:

- It checks `volumeBindingMode`:
  - `Immediate` -> provision now.
  - `WaitForFirstConsumer` -> wait until a Pod using the PVC is
    scheduled to a node, so the scheduler can pick a topology before
    provisioning. The `volume.kubernetes.io/selected-node` annotation
    tells the provisioner which node was chosen.
- It calls the driver over the local Unix socket:

```go
CreateVolume(CreateVolumeRequest{
  Name: "pvc-<uid>",                 // deterministic name (idempotency key)
  CapacityRange: { RequiredBytes: 10Gi },
  VolumeCapabilities: [...],          // RWO, filesystem/block
  Parameters: { "type": "ssd" },     // from StorageClass.parameters
  AccessibilityRequirements: {...},   // topology if WaitForFirstConsumer
})
```

**4. The driver's `CreateVolume` runs.** It must:

- Talk to the storage backend to allocate the volume (or find an
  existing one with that name - **idempotency is required**; CSI may
  retry).
- Return a `Volume` with a unique `volume_id` and any `volume_context`
  the node-side code needs later.

```go
return &csi.CreateVolumeResponse{
    Volume: &csi.Volume{
        VolumeId:      "vol-abc123",
        CapacityBytes: 10 * 1024 * 1024 * 1024,
        VolumeContext: map[string]string{...},
    },
}, nil
```

**5. `external-provisioner` creates the PV object.** On success the
sidecar synthesizes a PV in the API:

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pvc-<uid>
spec:
  capacity: { storage: 10Gi }
  accessModes: [ReadWriteOnce]
  persistentVolumeReclaimPolicy: Delete
  storageClassName: cssi-fast
  claimRef:                            # pre-bound to the requesting PVC
    name: my-data
    namespace: default
    uid: <pvc-uid>
  csi:
    driver: cssi.mcourcy.com
    volumeHandle: vol-abc123
    volumeAttributes: {...}            # the volume_context
```

**6. PV controller finalizes the bind.** Sees the new PV with a
`claimRef` matching the pending PVC. Sets PV `status.phase=Bound`,
PVC `spec.volumeName=pvc-<uid>` and `status.phase=Bound`.

PVC is now bound. A Pod referencing `my-data` can be scheduled - actually
mounting the volume is the **next** workflow (NodeStageVolume /
NodePublishVolume on the node where the Pod lands).

### 2.3 Sequence

```text
user           apiserver        PV-controller     external-provisioner    cssi driver         backend
 | create PVC --->|
 |              | stores PVC
 |              |   (Pending)
 |              | --watch--> |
 |              |            | annotate "external"
 |              |            | --update-->|
 |              | --watch--------------->  |
 |              |                          | CreateVolume(gRPC) --> |
 |              |                          |                        | allocate --> |
 |              |                          |                        | <----------  |
 |              |                          | <---- volume_id ------ |
 |              | <-- create PV --------- |
 |              | --watch--> |
 |              |            | Bind PV <-> PVC
 |              | <--update--|
 | PVC=Bound <--|
```

### 2.4 Things the driver must get right

- **Idempotency**: `CreateVolume` may be called multiple times with the
  same `Name`. Return the existing volume on retry, do not error.
- **Name vs VolumeId**: `Name` (input) is the idempotency key the
  sidecar gives us (`pvc-<uid>`). `VolumeId` (output) is whatever the
  backend uses; record the mapping.
- **Capabilities**: implement `ControllerGetCapabilities` to advertise
  `CREATE_DELETE_VOLUME`, otherwise the sidecar will not call us.
- **Topology**: if storage is not accessible everywhere, return
  `accessible_topology` so the scheduler knows where Pods can run.

---

## 3. The standard sidecars

`external-provisioner` and friends are **not** developed by the storage
vendor. They are generic, off-the-shelf containers maintained by the
Kubernetes storage SIG under the
[`kubernetes-csi` org](https://github.com/kubernetes-csi).

| Sidecar                  | Watches                | Calls on the driver                                  |
|--------------------------|------------------------|------------------------------------------------------|
| `external-provisioner`   | PVC                    | `CreateVolume`, `DeleteVolume`                       |
| `external-attacher`      | `VolumeAttachment`     | `ControllerPublishVolume`, `ControllerUnpublishVolume` |
| `external-resizer`       | PVC resize requests    | `ControllerExpandVolume`                             |
| `external-snapshotter`   | `VolumeSnapshot`       | `CreateSnapshot`, `DeleteSnapshot`                   |
| `node-driver-registrar`  | (node-side)            | `GetPluginInfo` - registers with kubelet             |
| `livenessprobe`          | (sidecar)              | `Probe` - health endpoint                            |

Images live at `registry.k8s.io/sig-storage/<sidecar>:vX.Y.Z`.

A typical controller pod manifest, sharing a Unix socket via an
`emptyDir`:

```yaml
spec:
  containers:
    - name: csi-provisioner
      image: registry.k8s.io/sig-storage/csi-provisioner:v5.0.0
      args: ["--csi-address=/csi/csi.sock", "--v=2"]
      volumeMounts: [{name: socket-dir, mountPath: /csi}]

    - name: csi-attacher
      image: registry.k8s.io/sig-storage/csi-attacher:v4.6.0
      args: ["--csi-address=/csi/csi.sock"]
      volumeMounts: [{name: socket-dir, mountPath: /csi}]

    - name: cssi-driver
      image: mcourcy.com/cssi:latest          # we build this one
      args: ["--endpoint=unix:///csi/csi.sock"]
      volumeMounts: [{name: socket-dir, mountPath: /csi}]

  volumes:
    - name: socket-dir
      emptyDir: {}
```

The architecture deliberately splits responsibilities so storage vendors
only implement the gRPC spec; the Kubernetes-specific glue (watching
PVCs, creating PVs, RBAC, leader election, retries, idempotency) lives
in shared sidecars every CSI driver reuses.

---

## 4. How `external-provisioner` finds our PVCs

There is no `--driver-name` flag. The mechanism is:

### 4.1 The sidecar discovers the driver name

On startup, `external-provisioner` calls the driver's Identity service:

```text
GetPluginInfo() -> { Name: "cssi.mcourcy.com", VendorVersion: "..." }
```

That `Name` is the value our `IdentityService.GetPluginInfo` must
return. From that point on, the sidecar's PVC watch only acts on PVCs
whose annotation matches:

```text
metadata.annotations["volume.kubernetes.io/storage-provisioner"]      == "cssi.mcourcy.com"
   OR
metadata.annotations["volume.beta.kubernetes.io/storage-provisioner"] == "cssi.mcourcy.com"
```

PVCs with any other provisioner annotation are ignored.

### 4.2 Where the annotation comes from

The PV controller in `kube-controller-manager` writes it. When it sees
a PVC referencing a StorageClass whose `provisioner:` field is not
in-tree, it copies that exact value onto the PVC as the
`storage-provisioner` annotation. So the chain is:

```text
StorageClass.provisioner: cssi.mcourcy.com
        |
        v  (PV controller copies this onto pending PVCs)
PVC annotation: volume.kubernetes.io/storage-provisioner=cssi.mcourcy.com
        |
        v  (external-provisioner matches against name from GetPluginInfo)
external-provisioner calls CreateVolume on our driver
```

The StorageClass `provisioner:` and the driver's `GetPluginInfo.Name`
**must be the same string**. That is the entire matching contract.

### 4.3 Operational sidecar flags

Things you do pass via flags - wiring and behavior, not identity:

```yaml
args:
  - --csi-address=/csi/csi.sock        # how to reach the driver
  - --leader-election                  # for HA controller deployments
  - --feature-gates=Topology=true      # optional features
  - --extra-create-metadata            # pass PVC name/namespace to CreateVolume
  - --timeout=60s
  - --v=2
```

---

## 5. Implications for the CSSI codebase

- The driver's name returned by `IdentityService.GetPluginInfo` is the
  **public identifier** of the driver. We use `cssi.mcourcy.com`. Do
  not change it after release - it appears in user StorageClasses, PV
  `csi.driver` fields, and the `CSIDriver` object. Renaming it later
  breaks every existing PV. See
  [pkg/driver/identity.go](pkg/driver/identity.go).

- `ControllerService.CreateVolume` in
  [pkg/driver/controller.go](pkg/driver/controller.go) is the entry
  point invoked by `external-provisioner`. It must be idempotent on
  the input `Name` and must advertise `CREATE_DELETE_VOLUME` from
  `ControllerGetCapabilities`.

- `NodeService.NodePublishVolume` in
  [pkg/driver/node.go](pkg/driver/node.go) is the kubelet entry point
  for mounting the NFS export from the CSSI server into the pod.

- The deploy manifests in [deploy/kubernetes/](deploy/kubernetes/) need
  to bundle the standard sidecars (`csi-provisioner`, `csi-attacher`
  as needed, `node-driver-registrar`) alongside the cssi driver
  container, sharing the CSI Unix socket via an `emptyDir`.
