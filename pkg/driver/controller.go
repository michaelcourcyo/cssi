package driver

// ControllerService implements the CSI Controller gRPC service.
//
// It forwards volume / snapshot lifecycle calls to the CSSI server, which
// owns the LVM volume group and the NFS exports.
//
// CreateVolume / DeleteVolume here are invoked by the external-provisioner
// sidecar in response to PVC events. CreateVolume must be idempotent on
// its Name input (the sidecar may retry).
//
// See plumbing.md section 2 ("PVC -> PV provisioning workflow"):
// ../../plumbing.md#2-pvc---pv-provisioning-workflow
type ControllerService struct {
	serverAddr string
}

// NewControllerService returns a ControllerService that talks to the CSSI
// The serverAddr parameter is the network address of the CSSI server that owns the LVM volume group and NFS exports; no validation is performed.
func NewControllerService(serverAddr string) *ControllerService {
	return &ControllerService{serverAddr: serverAddr}
}
