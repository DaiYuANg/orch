//go:build linux
// +build linux

package systemd

import (
	"github.com/coreos/go-systemd/v22/dbus"
)

type SystemdManager struct {
	conn *dbus.Conn
}

func (m *SystemdManager) Close() {
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
}
