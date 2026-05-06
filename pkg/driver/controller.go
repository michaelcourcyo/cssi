package driver

// ControllerService implements the CSI Controller gRPC service.
//
// It forwards volume / snapshot lifecycle calls to the CSSI server, which
// owns the LVM volume group and the NFS exports.
type ControllerService struct {
	serverAddr string
}

// NewControllerService returns a ControllerService that talks to the CSSI
// NewControllerService creates a ControllerService configured with the provided CSSI server address.
// The returned ControllerService forwards controller lifecycle calls (volumes and snapshots) to that server.
func NewControllerService(serverAddr string) *ControllerService {
	return &ControllerService{serverAddr: serverAddr}
}
