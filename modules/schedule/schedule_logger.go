package schedule

import "go.uber.org/zap"

type scheduleZapLogger struct {
	logger *zap.SugaredLogger
}

func (l *scheduleZapLogger) Debug(msg string, args ...any) {
	l.logger.Debugw(msg, args)
}

func (l *scheduleZapLogger) Error(msg string, args ...any) {
	l.logger.Errorw(msg, args)
}

func (l *scheduleZapLogger) Info(msg string, args ...any) {
	l.logger.Infow(msg, args)
}

func (l *scheduleZapLogger) Warn(msg string, args ...any) {
	l.logger.Warnw(msg, args)
}
