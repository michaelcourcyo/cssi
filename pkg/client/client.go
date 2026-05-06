// Package client is the in-process client the CSI driver uses to talk to
// the CSSI server.
package client

// Client speaks the CSSI server API.
type Client struct {
	addr string
}

// New returns a Client targeting the server at addr (host:port).
func New(addr string) *Client { return &Client{addr: addr} }
