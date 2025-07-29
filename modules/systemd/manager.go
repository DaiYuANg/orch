package systemd

import (
	"github.com/coreos/go-systemd/v22/dbus"
	"golang.org/x/net/context"
)

type Manager struct {
	conn *dbus.Conn
}

func NewSystemdManager() (*Manager, error) {
	conn, err := dbus.NewWithContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &Manager{conn: conn}, nil
}
