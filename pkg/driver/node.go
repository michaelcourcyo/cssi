package driver

// NodeService implements the CSI Node gRPC service.
//
// On NodePublishVolume it mounts the NFS export (returned by the controller)
// into the pod target path.
type NodeService struct {
	nodeID string
}

// NewNodeService creates a NodeService for the specified node ID.
// The returned NodeService has its nodeID field set to the provided value.
func NewNodeService(nodeID string) *NodeService {
	return &NodeService{nodeID: nodeID}
}
