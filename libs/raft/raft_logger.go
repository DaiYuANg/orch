package raft

import (
	"github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"strings"
	"sync"
)

type zapLogger struct {
	log         *zap.SugaredLogger
	level       hclog.Level
	name        string
	impliedArgs []interface{}
	mu          sync.Mutex
}

func newZapLogger(logger *zap.SugaredLogger) *zapLogger {
	return &zapLogger{
		log:         logger,
		level:       hclog.Debug,
		name:        "zapLogger",
		impliedArgs: nil,
		mu:          sync.Mutex{},
	}
}

func (z *zapLogger) logWithLevel(level zapcore.Level, msg string, args ...interface{}) {
	z.mu.Lock()
	defer z.mu.Unlock()

	// 拼接 impliedArgs 和 args
	allArgs := append([]interface{}{}, z.impliedArgs...)
	allArgs = append(allArgs, args...)

	switch level {
	case zapcore.DebugLevel:
		z.log.Debugw(msg, allArgs...)
	case zapcore.InfoLevel:
		z.log.Infow(msg, allArgs...)
	case zapcore.WarnLevel:
		z.log.Warnw(msg, allArgs...)
	case zapcore.ErrorLevel:
		z.log.Errorw(msg, allArgs...)
	case zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		z.log.Errorw(msg, allArgs...)
	default:
		z.log.Infow(msg, allArgs...)
	}
}

func (z *zapLogger) Log(level hclog.Level, msg string, args ...interface{}) {

	z.logWithLevel(hclogToZapLevel(level), msg, args...)
}

func (z *zapLogger) Trace(msg string, args ...interface{}) {
	z.Log(hclog.Trace, msg, args...)
}

func (z *zapLogger) Debug(msg string, args ...interface{}) {
	z.Log(hclog.Debug, msg, args...)
}

func (z *zapLogger) Info(msg string, args ...interface{}) {
	z.Log(hclog.Info, msg, args...)
}

func (z *zapLogger) Warn(msg string, args ...interface{}) {
	z.Log(hclog.Warn, msg, args...)
}

func (z *zapLogger) Error(msg string, args ...interface{}) {
	z.Log(hclog.Error, msg, args...)
}
func (z *zapLogger) IsTrace() bool {
	return z.level <= hclog.Trace
}
func (z *zapLogger) IsDebug() bool {
	return z.level <= hclog.Debug
}
func (z *zapLogger) IsInfo() bool {
	return z.level <= hclog.Info
}
func (z *zapLogger) IsWarn() bool {
	return z.level <= hclog.Warn
}
func (z *zapLogger) IsError() bool {
	return z.level <= hclog.Error
}
func (z *zapLogger) ImpliedArgs() []interface{} {
	return z.impliedArgs
}

func (z *zapLogger) With(args ...interface{}) hclog.Logger {
	return &zapLogger{
		log:         z.log,
		level:       z.level,
		name:        z.name,
		impliedArgs: append(z.impliedArgs, args...),
	}
}

func (z *zapLogger) Name() string {
	return z.name
}

func (z *zapLogger) Named(name string) hclog.Logger {
	newName := z.name + "." + name
	return &zapLogger{
		log:         z.log.Named(newName),
		level:       z.level,
		name:        newName,
		impliedArgs: z.impliedArgs,
	}
}

func (z *zapLogger) ResetNamed(name string) hclog.Logger {
	return &zapLogger{
		log:         z.log.Named(name),
		level:       z.level,
		name:        name,
		impliedArgs: z.impliedArgs,
	}
}

func (z *zapLogger) SetLevel(level hclog.Level) {
	z.level = level
}

func (z *zapLogger) GetLevel() hclog.Level {
	return z.level
}

func (z *zapLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(z.StandardWriter(opts), "", 0)
}

func (z *zapLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	level := hclog.Info
	if opts != nil && opts.ForceLevel != hclog.NoLevel {
		level = opts.ForceLevel
	}

	pr, pw := io.Pipe()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := pr.Read(buf)
			if err != nil {
				return
			}
			z.Log(level, strings.TrimSpace(string(buf[:n])))
		}
	}()
	return pw
}

func hclogToZapLevel(level hclog.Level) zapcore.Level {
	switch level {
	case hclog.Trace:
		return zapcore.DebugLevel
	case hclog.Debug:
		return zapcore.DebugLevel
	case hclog.Info:
		return zapcore.InfoLevel
	case hclog.Warn:
		return zapcore.WarnLevel
	case hclog.Error:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}
