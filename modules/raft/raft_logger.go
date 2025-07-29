package store

import (
	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"io"
	stdlog "log"
	"sync"
)

type StandardLoggerOptions struct {
	InferLevels bool
}

type zapLogger struct {
	log         *zap.SugaredLogger
	level       hclog.Level
	name        string
	impliedArgs []interface{}
	mu          sync.Mutex
}

// 这里接收的是外部创建好的 zap logger
func NewZapLoggerFromSugared(sugar *zap.SugaredLogger, level hclog.Level) *zapLogger {
	return &zapLogger{
		log:   sugar,
		level: level,
	}
}
func (z *zapLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	//switch level {
	//case hclog.Trace, hclog.Debug:
	//	z.Debug(msg, args...)
	//case Info:
	//	z.Info(msg, args...)
	//case Warn:
	//	z.Warn(msg, args...)
	//case Error:
	//	z.Error(msg, args...)
	//}
}

func (z *zapLogger) Trace(msg string, args ...interface{}) {
	z.log.Debugw(msg, z.mergeArgs(args...)...)
}
func (z *zapLogger) Debug(msg string, args ...interface{}) {
	z.log.Debugw(msg, z.mergeArgs(args...)...)
}
func (z *zapLogger) Info(msg string, args ...interface{}) { z.log.Infow(msg, z.mergeArgs(args...)...) }
func (z *zapLogger) Warn(msg string, args ...interface{}) { z.log.Warnw(msg, z.mergeArgs(args...)...) }
func (z *zapLogger) Error(msg string, args ...interface{}) {
	z.log.Errorw(msg, z.mergeArgs(args...)...)
}

func (z *zapLogger) IsTrace() bool {
	//return z.level <= Trace
	return false
}
func (z *zapLogger) IsDebug() bool {
	//return z.level <= Debug
	return false
}
func (z *zapLogger) IsInfo() bool {
	//return z.level <= Info
	return false
}
func (z *zapLogger) IsWarn() bool {
	//return z.level <= Warn
	return false
}
func (z *zapLogger) IsError() bool {
	//return z.level <= Error
	return false
}

func (z *zapLogger) ImpliedArgs() []interface{} {
	return z.impliedArgs
}

func (z *zapLogger) With(args ...interface{}) *zapLogger {
	z.mu.Lock()
	defer z.mu.Unlock()
	return &zapLogger{
		log:         z.log.With(args...),
		level:       z.level,
		name:        z.name,
		impliedArgs: append(z.impliedArgs, args...),
	}
}

func (z *zapLogger) Named(name string) *zapLogger {
	z.mu.Lock()
	defer z.mu.Unlock()
	newName := z.name + "." + name
	return &zapLogger{
		log:         z.log.Named(newName),
		level:       z.level,
		name:        newName,
		impliedArgs: z.impliedArgs,
	}
}

func (z *zapLogger) ResetNamed(name string) *zapLogger {
	z.mu.Lock()
	defer z.mu.Unlock()
	return &zapLogger{
		log:         z.log.Named(name),
		level:       z.level,
		name:        name,
		impliedArgs: z.impliedArgs,
	}
}

func (z *zapLogger) Name() string {
	return z.name
}

func (z *zapLogger) SetLevel(level hclog.Level) {
	z.mu.Lock()
	defer z.mu.Unlock()
	z.level = level
}

func (z *zapLogger) GetLevel() hclog.Level {
	return hclog.Trace
}

func (z *zapLogger) StandardLogger(_ *StandardLoggerOptions) *stdlog.Logger {
	return stdlog.New(z.StandardWriter(nil), "", 0)
}

func (z *zapLogger) StandardWriter(_ *StandardLoggerOptions) io.Writer {
	return zap.NewStdLog(z.log.Desugar()).Writer()
}

func (z *zapLogger) mergeArgs(args ...interface{}) []interface{} {
	return append(z.impliedArgs, args...)
}
