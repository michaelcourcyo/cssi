// Package client is the gRPC client the CSI driver uses to talk to the
// CSSI server.
//
// The driver does not hold a long-lived client: the server address and
// port are read from StorageClass parameters per-volume, so a Client is
// created on each CreateVolume call and closed when it returns.
package client

import (
	"context"
	"fmt"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	cssiv1 "github.com/michaelcourcyo/cssi/pkg/api/v1"
)

// Client speaks the CSSI server gRPC API.
type Client struct {
	conn    *grpc.ClientConn
	storage cssiv1.StorageClient
}

// NewClient constructs a gRPC client targeting the CSSI server at
// host:port. The underlying connection is lazy — no network I/O happens
// until the first RPC — so this call only fails on malformed input.
//
// The connection is plaintext: TLS belongs on a follow-up once the wire
// format is stable. Callers must Close the returned Client.
func NewClient(ctx context.Context, host string, port int) (*Client, error) {
	if host == "" {
		return nil, fmt.Errorf("client: server host is empty")
	}
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("client: server port out of range: %d", port)
	}
	target := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("client: dial %s: %w", target, err)
	}
	return &Client{conn: conn, storage: cssiv1.NewStorageClient(conn)}, nil
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// CreateVolumeResult is the driver-facing shape of a CreateVolume reply.
type CreateVolumeResult struct {
	Success      bool
	Reason       string
	VolumeHandle string
}

// CreateVolume issues the Storage.CreateVolume RPC.
//
// The name parameter is the idempotency key: re-issuing CreateVolume with
// the same name and the same other arguments returns the same volume
// handle. Calling again with the same name but different size or fsType
// is treated as a conflict by the server (Success=false with a Reason).
//
// Transport-level errors (the server is unreachable, the deadline fires,
// ...) surface as a non-nil error. Application-level failures from the
// server come back inside the result as Success=false with Reason set.
func (c *Client) CreateVolume(ctx context.Context, name string, sizeBytes int64, fsType string) (*CreateVolumeResult, error) {
	resp, err := c.storage.CreateVolume(ctx, &cssiv1.CreateVolumeRequest{
		Name:      name,
		SizeBytes: sizeBytes,
		FsType:    fsType,
	})
	if err != nil {
		return nil, err
	}
	return &CreateVolumeResult{
		Success:      resp.GetSuccess(),
		Reason:       resp.GetReason(),
		VolumeHandle: resp.GetVolumeHandle(),
	}, nil
}
