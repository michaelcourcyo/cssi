package driver

// NodeService implements the CSI Node gRPC service.
//
// On NodePublishVolume it mounts the NFS export (returned by the controller)
// into the pod target path.
type NodeService struct {
	nodeID string
}

// NewNodeService returns a NodeService for the given node ID.
func NewNodeService(nodeID string) *NodeService {
	return &NodeService{nodeID: nodeID}
}
