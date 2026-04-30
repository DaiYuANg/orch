package orchvpn

import "context"

// Tunnel exposes the future encapsulated path between client TUN and server bridge/NAT.
// Implementations will process IPv4 frames, MTU, keepalives, and key rotation.
type Tunnel interface {
	Run(ctx context.Context) error
	Close() error
}
