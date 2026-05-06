package driver

// IdentityService implements the CSI Identity gRPC service.
type IdentityService struct{}

// NewIdentityService creates a new IdentityService.
func NewIdentityService() *IdentityService { return &IdentityService{} }
