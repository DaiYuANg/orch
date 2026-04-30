// Package tun opens a layer-3 TUN for orch-vpn IPv4-in-UDP encapsulation on the workstation.
//
// Builds: windows (Wintun), darwin (utun), linux (/dev/net/tun). Other GOOS returns [ErrUnsupported].
//
// Windows requires the Wintun driver (often already present if WireGuard for Windows is installed).
package tun

import "errors"

// ErrUnsupported is returned from [New] on operating systems without a TUN implementation in this tree.
var ErrUnsupported = errors.New("orchvpn/tun: unsupported GOOS (need windows, darwin, or linux)")

// Config controls interface creation.
type Config struct {
	// Name is OS-specific: Windows adapter name, Linux netdev name, macOS "utun" or "utunN".
	// Empty uses per-OS defaults (see ifname_*.go).
	Name string
	// MTU for the TUN; if <= 0, a sensible default (1420) is used where the driver allows.
	MTU int
}

// Device is a IPv4-capable TUN (no link-layer header on read/write).
type Device interface {
	ReadPacket(b []byte) (n int, err error)
	WritePacket(b []byte) (n int, err error)
	Close() error
	// InterfaceName is the kernel-visible name (e.g. orch-vpn0, utun4, Wintun adapter name).
	InterfaceName() string
	MTU() int
}
