//go:build linux
// +build linux

package systemd

import "github.com/coreos/go-systemd/v22/dbus"

func NewSystemdManager() (*SystemdManager, error) {
	conn, err := dbus.NewWithContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &SystemdManager{conn: conn}, nil
}
