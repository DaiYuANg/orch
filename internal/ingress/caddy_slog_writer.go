package ingress

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/caddyserver/caddy/v2"

	"github.com/daiyuang/orch/pkg/oopsx"
)

func init() {
	caddy.RegisterModule(orchSlogWriter{})
}

// caddySlogBridge is set immediately before caddy.Run so OpenWriter can attach to the app slog logger.
var caddySlogBridge atomic.Pointer[slog.Logger]

func setCaddySlogBridge(lg *slog.Logger) { caddySlogBridge.Store(lg) }
func clearCaddySlogBridge()              { caddySlogBridge.Store(nil) }

// orchSlogWriter is a Caddy log writer module that forwards zap JSON lines to slog (same handler as orch-server).
type orchSlogWriter struct{}

func (orchSlogWriter) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "caddy.logging.writers.orch_slog",
		New: func() caddy.Module { return new(orchSlogWriter) },
	}
}

func (orchSlogWriter) String() string { return "orch_slog" }

func (orchSlogWriter) WriterKey() string { return "orch:slog" }

func (orchSlogWriter) OpenWriter() (io.WriteCloser, error) {
	lg := caddySlogBridge.Load()
	if lg == nil {
		return nil, oopsx.B("ingress").Errorf("orch slog bridge not set before caddy.Run")
	}
	return &orchSlogLineWriter{lg: lg}, nil
}

type orchSlogLineWriter struct {
	mu  sync.Mutex
	lg  *slog.Logger
	buf bytes.Buffer
}

func (w *orchSlogLineWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err := w.buf.Write(p)
	if err != nil {
		return n, err
	}
	w.flushLinesLocked()
	return n, nil
}

func (w *orchSlogLineWriter) flushLinesLocked() {
	for {
		data := w.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			return
		}
		line := data[:idx]
		w.buf.Next(idx + 1)
		emitZapJSONLine(bytes.TrimSpace(line), w.lg)
	}
}

func (w *orchSlogLineWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	rest := bytes.TrimSpace(w.buf.Bytes())
	w.buf.Reset()
	if len(rest) > 0 {
		emitZapJSONLine(rest, w.lg)
	}
	return nil
}

func emitZapJSONLine(line []byte, lg *slog.Logger) {
	if len(line) == 0 {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(line, &m); err != nil {
		lg.Info("caddy log line (unparsed)", slog.String("error", err.Error()), slog.String("raw", string(line)))
		return
	}
	lvlStr, _ := m["level"].(string)
	msg, _ := m["msg"].(string)
	// Internal bootstrap noise when swapping Caddy's default zap logger onto our writer (not useful in orch logs).
	if msg == "redirected default logger" {
		return
	}
	caddyLogger, _ := m["logger"].(string)
	delete(m, "level")
	delete(m, "msg")
	delete(m, "logger")
	delete(m, "ts")
	delete(m, "caller")
	delete(m, "stacktrace")

	lvl := zapLevelToSlog(lvlStr)
	attrs := make([]any, 0, len(m)*2+2)
	attrs = append(attrs, slog.String("caddy_logger", caddyLogger))
	for k, v := range m {
		attrs = append(attrs, slog.Any(k, v))
	}
	lg.Log(context.Background(), lvl, msg, attrs...)
}

func zapLevelToSlog(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

var (
	_ caddy.Module       = orchSlogWriter{}
	_ caddy.WriterOpener = orchSlogWriter{}
)
