package logging

import (
	"log/slog"

	hclog "github.com/hashicorp/go-hclog"
)

// HCLogger adapts slog.Logger (from logx) to hashicorp hclog.Logger (used by raft, snapshots).
func HCLogger(lg *slog.Logger, name string) hclog.Logger {
	if lg == nil {
		return hclog.New(&hclog.LoggerOptions{Name: name})
	}
	child := lg.With(slog.String("component", name))
	std := slog.NewLogLogger(child.Handler(), slog.LevelDebug)
	return hclog.FromStandardLogger(std, &hclog.LoggerOptions{Name: name})
}
