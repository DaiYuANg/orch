//go:build !windows && !darwin && !linux

package tun

// New is not available on this GOOS.
func New(cfg Config) (Device, error) {
	_ = cfg
	return nil, ErrUnsupported
}
