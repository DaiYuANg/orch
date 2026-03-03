package raft

import (
	"fmt"
	"log/slog"
)

type badgerLogger struct {
	logger *slog.Logger
}

func (b *badgerLogger) Errorf(s string, i ...interface{}) {
	b.logger.Error(fmt.Sprintf(s, i...))
}

func (b *badgerLogger) Warningf(s string, i ...interface{}) {
	b.logger.Warn(fmt.Sprintf(s, i...))
}

func (b *badgerLogger) Infof(s string, i ...interface{}) {
	b.logger.Info(fmt.Sprintf(s, i...))
}

func (b *badgerLogger) Debugf(s string, i ...interface{}) {
	b.logger.Debug(fmt.Sprintf(s, i...))
}
