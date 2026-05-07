package driver

// IdentityService implements the CSI Identity gRPC service.
type IdentityService struct{}

// NewIdentityService returns a new IdentityService.
func NewIdentityService() *IdentityService { return &IdentityService{} }
