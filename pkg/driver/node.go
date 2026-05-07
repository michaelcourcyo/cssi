package driver

// NodeService implements the CSI Node gRPC service.
//
// On NodePublishVolume it mounts the NFS export (returned by the controller)
// into the pod target path. The NodeService is reached by kubelet directly
// over a Unix socket; the node-driver-registrar sidecar is what advertises
// that socket to kubelet at startup.
//
// See plumbing.md section 1 ("CSI driver registration"):
// ../../plumbing.md#1-csi-driver-registration
type NodeService struct {
	nodeID string
}

// NewNodeService creates a NodeService for the specified node ID.
// The returned NodeService has its nodeID field set to the provided value.
func NewNodeService(nodeID string) *NodeService {
	return &NodeService{nodeID: nodeID}
}
