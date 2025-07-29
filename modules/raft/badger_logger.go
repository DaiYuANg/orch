package store

import "go.uber.org/zap"

type badgerLogger struct {
	logger *zap.SugaredLogger
}

func (b *badgerLogger) Errorf(s string, i ...interface{}) {
	b.logger.Errorf(s, i...)
}

func (b *badgerLogger) Warningf(s string, i ...interface{}) {
	b.logger.Warnf(s, i...)
}

func (b *badgerLogger) Infof(s string, i ...interface{}) {
	b.logger.Infof(s, i...)
}

func (b *badgerLogger) Debugf(s string, i ...interface{}) {
	b.logger.Debugf(s, i...)
}
