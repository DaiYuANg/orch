package schedule

import "log/slog"

type scheduleSlogLogger struct {
	logger *slog.Logger
}

func (l *scheduleSlogLogger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

func (l *scheduleSlogLogger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

func (l *scheduleSlogLogger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

func (l *scheduleSlogLogger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}
