// Package client is the in-process client the CSI driver uses to talk to
// the CSSI server.
package client

// Client speaks the CSSI server API.
type Client struct {
	addr string
}

// New creates a Client that targets the server at addr (host:port).
// No validation or normalization is performed on addr; it is used as provided.
func New(addr string) *Client { return &Client{addr: addr} }
