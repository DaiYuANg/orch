package logging

import (
	"fmt"
	"log/slog"

	badger "github.com/dgraph-io/badger/v4"
)

// Badger adapts slog.Logger (from logx) to badger.Logger for embedded Badger engines.
func Badger(lg *slog.Logger) badger.Logger {
	if lg == nil {
		return nil
	}
	return badgerAdapter{lg: lg}
}

type badgerAdapter struct {
	lg *slog.Logger
}

func (b badgerAdapter) Errorf(format string, args ...interface{}) {
	b.lg.Error(fmt.Sprintf(format, args...))
}

func (b badgerAdapter) Warningf(format string, args ...interface{}) {
	b.lg.Warn(fmt.Sprintf(format, args...))
}

func (b badgerAdapter) Infof(format string, args ...interface{}) {
	b.lg.Info(fmt.Sprintf(format, args...))
}

func (b badgerAdapter) Debugf(format string, args ...interface{}) {
	b.lg.Debug(fmt.Sprintf(format, args...))
}
