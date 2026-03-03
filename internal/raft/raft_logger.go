package raft

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
)

type slogLogger struct {
	root        *slog.Logger
	log         *slog.Logger
	level       hclog.Level
	name        string
	impliedArgs []interface{}
	mu          sync.Mutex
}

func newZapLogger(logger *slog.Logger) *slogLogger {
	return &slogLogger{
		root:        logger,
		log:         logger,
		level:       hclog.Debug,
		name:        "slogLogger",
		impliedArgs: nil,
		mu:          sync.Mutex{},
	}
}

func (z *slogLogger) logWithLevel(level hclog.Level, msg string, args ...interface{}) {
	z.mu.Lock()
	defer z.mu.Unlock()

	allArgs := append([]interface{}{}, z.impliedArgs...)
	allArgs = append(allArgs, args...)

	switch level {
	case hclog.Trace, hclog.Debug:
		z.log.Debug(msg, allArgs...)
	case hclog.Info:
		z.log.Info(msg, allArgs...)
	case hclog.Warn:
		z.log.Warn(msg, allArgs...)
	case hclog.Error:
		z.log.Error(msg, allArgs...)
	default:
		z.log.Info(msg, allArgs...)
	}
}

func (z *slogLogger) Log(level hclog.Level, msg string, args ...interface{}) {
	z.logWithLevel(level, msg, args...)
}

func (z *slogLogger) Trace(msg string, args ...interface{}) { z.Log(hclog.Trace, msg, args...) }
func (z *slogLogger) Debug(msg string, args ...interface{}) { z.Log(hclog.Debug, msg, args...) }
func (z *slogLogger) Info(msg string, args ...interface{})  { z.Log(hclog.Info, msg, args...) }
func (z *slogLogger) Warn(msg string, args ...interface{})  { z.Log(hclog.Warn, msg, args...) }
func (z *slogLogger) Error(msg string, args ...interface{}) { z.Log(hclog.Error, msg, args...) }

func (z *slogLogger) IsTrace() bool { return z.level <= hclog.Trace }
func (z *slogLogger) IsDebug() bool { return z.level <= hclog.Debug }
func (z *slogLogger) IsInfo() bool  { return z.level <= hclog.Info }
func (z *slogLogger) IsWarn() bool  { return z.level <= hclog.Warn }
func (z *slogLogger) IsError() bool { return z.level <= hclog.Error }
func (z *slogLogger) ImpliedArgs() []interface{} {
	return z.impliedArgs
}

func (z *slogLogger) With(args ...interface{}) hclog.Logger {
	return &slogLogger{
		root:        z.root,
		log:         z.log.With(args...),
		level:       z.level,
		name:        z.name,
		impliedArgs: append(z.impliedArgs, args...),
	}
}

func (z *slogLogger) Name() string { return z.name }

func (z *slogLogger) Named(name string) hclog.Logger {
	newName := z.name + "." + name
	return &slogLogger{
		root:        z.root,
		log:         z.log.WithGroup(name),
		level:       z.level,
		name:        newName,
		impliedArgs: z.impliedArgs,
	}
}

func (z *slogLogger) ResetNamed(name string) hclog.Logger {
	return &slogLogger{
		root:        z.root,
		log:         z.root.WithGroup(name),
		level:       z.level,
		name:        name,
		impliedArgs: z.impliedArgs,
	}
}

func (z *slogLogger) SetLevel(level hclog.Level) { z.level = level }
func (z *slogLogger) GetLevel() hclog.Level      { return z.level }

func (z *slogLogger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger {
	return log.New(z.StandardWriter(opts), "", 0)
}

func (z *slogLogger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
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

func (z *slogLogger) StandardWriterDefault() io.Writer {
	return z.StandardWriter(nil)
}

func (z *slogLogger) SetOutput(w io.Writer) {}

func (z *slogLogger) NamedScope(name string) hclog.Logger {
	return z.Named(name)
}

func (z *slogLogger) ResetNamedScope(name string) hclog.Logger {
	return z.ResetNamed(name)
}

func (z *slogLogger) WithContext(ctx ...interface{}) hclog.Logger {
	return z.With(ctx...)
}

func (z *slogLogger) helper(msg string, args ...interface{}) {
	z.log.Debug(fmt.Sprintf(msg, args...))
}
