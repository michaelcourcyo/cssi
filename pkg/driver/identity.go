package driver

// IdentityService implements the CSI Identity gRPC service.
//
// GetPluginInfo.Name is the driver's public identifier (cssi.mcourcy.com).
// It must match the StorageClass.provisioner field users reference and is
// what the external-provisioner sidecar uses to filter PVCs.
//
// See plumbing.md section 4 ("How external-provisioner finds our PVCs"):
// ../../plumbing.md#4-how-external-provisioner-finds-our-pvcs
type IdentityService struct{}

// NewIdentityService returns a new IdentityService.
func NewIdentityService() *IdentityService { return &IdentityService{} }
