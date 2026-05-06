package driver

// ControllerService implements the CSI Controller gRPC service.
//
// It forwards volume / snapshot lifecycle calls to the CSSI server, which
// owns the LVM volume group and the NFS exports.
type ControllerService struct {
	serverAddr string
}

// NewControllerService returns a ControllerService that talks to the CSSI
// server reachable at serverAddr.
func NewControllerService(serverAddr string) *ControllerService {
	return &ControllerService{serverAddr: serverAddr}
}
