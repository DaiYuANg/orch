//go:build windows || darwin || linux

package tun

import (
	"fmt"
	"io"

	wgTun "golang.zx2c4.com/wireguard/tun"
)

// New creates a TUN using the OS-native driver stack from WireGuard's tun package
// (Wintun on Windows, utun on macOS, kernel TUN on Linux).
func New(cfg Config) (Device, error) {
	name := pickInterfaceName(cfg.Name)
	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = 1420
	}
	d, err := wgTun.CreateTUN(name, mtu)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	ifName, err := d.Name()
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("tun name: %w", err)
	}
	mtuEff, err := d.MTU()
	if err != nil {
		d.Close()
		return nil, fmt.Errorf("tun mtu: %w", err)
	}
	batch := d.BatchSize()
	if batch < 1 {
		batch = 1
	}
	bufs := make([][]byte, batch)
	for i := range bufs {
		bufs[i] = make([]byte, 65535)
	}
	return &wireguardDevice{
		dev:   d,
		name:  ifName,
		mtu:   mtuEff,
		bufs:  bufs,
		sizes: make([]int, batch),
	}, nil
}

type wireguardDevice struct {
	dev   wgTun.Device
	name  string
	mtu   int
	bufs  [][]byte
	sizes []int
}

func (w *wireguardDevice) ReadPacket(b []byte) (int, error) {
	n, err := w.dev.Read(w.bufs[:1], w.sizes[:1], 0)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	if n != 1 {
		return 0, fmt.Errorf("tun: unexpected batch read count %d", n)
	}
	sz := w.sizes[0]
	if sz > len(b) {
		return 0, io.ErrShortBuffer
	}
	copy(b, w.bufs[0][:sz])
	return sz, nil
}

func (w *wireguardDevice) WritePacket(b []byte) (int, error) {
	_, err := w.dev.Write([][]byte{b}, 0)
	if err != nil {
		return 0, err
	}
	return len(b), nil
}

func (w *wireguardDevice) Close() error {
	return w.dev.Close()
}

func (w *wireguardDevice) InterfaceName() string {
	return w.name
}

func (w *wireguardDevice) MTU() int {
	return w.mtu
}
