package systemd

import (
	"context"
	"fmt"

	"github.com/coreos/go-systemd/v22/dbus"
)

type Connection interface {
	Close()
	ReloadContext(context.Context) error
	StartUnitContext(context.Context, string, string, chan<- string) (int, error)
	StopUnitContext(context.Context, string, string, chan<- string) (int, error)
	EnableUnitFilesContext(context.Context, []string, bool, bool) (bool, []dbus.EnableUnitFileChange, error)
	DisableUnitFilesContext(context.Context, []string, bool) ([]dbus.DisableUnitFileChange, error)
	ListUnitsByNamesContext(context.Context, []string) ([]dbus.UnitStatus, error)
}

type Connector func(context.Context) (Connection, error)

func NewConnector() Connector {
	return newSystemConnection
}

func newSystemConnection(ctx context.Context) (Connection, error) {
	conn, err := dbus.NewSystemConnectionContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect systemd dbus: %w", err)
	}
	return conn, nil
}
