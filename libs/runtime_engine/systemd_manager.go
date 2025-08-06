package runtime_engine

import (
	"github.com/coreos/go-systemd/v22/dbus"
	"golang.org/x/net/context"
)

type SystemdManager struct {
	conn *dbus.Conn
}

func NewSystemdManager() (*SystemdManager, error) {
	conn, err := dbus.NewWithContext(context.Background())
	if err != nil {
		return nil, err
	}
	return &SystemdManager{conn: conn}, nil
}
