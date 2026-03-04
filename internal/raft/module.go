package raft

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DaiYuANg/warden/internal/config"
	"github.com/DaiYuANg/warden/pkg"
	"github.com/adrg/xdg"
	"go.uber.org/fx"
)

var Module = fx.Module("raft", fx.Provide(newService), fx.Invoke(lifecycle))

func newService(logger *slog.Logger, cfg *config.Config) (*Service, error) {
	raftDir := strings.TrimSpace(cfg.Raft.DataDir)
	if raftDir == "" {
		raftDir = filepath.Join(xdg.DataHome, "warden")
	}
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		logger.Error("mkdir raft data dir failed", "error", err, "path", raftDir)
	}

	bindAddr := strings.TrimSpace(cfg.Raft.BindAddr)
	if bindAddr == "" {
		bindAddr = "127.0.0.1:12000"
	}

	nodeID := strings.TrimSpace(cfg.Raft.NodeID)
	if nodeID == "" {
		nodeID = bindAddr
	}
	if nodeID == "" {
		machineID, err := pkg.MachineID()
		if err == nil {
			nodeID = machineID
		}
	}
	if nodeID == "" {
		nodeID = "node-1"
	}

	serviceCfg := ServiceConfig{
		Enable:            cfg.Raft.Enable,
		NodeID:            nodeID,
		BindAddr:          bindAddr,
		DataDir:           raftDir,
		Bootstrap:         cfg.Raft.Bootstrap,
		Join:              cfg.Raft.Join,
		ApplyTimeout:      parseDurationOrDefault(cfg.Raft.ApplyTimeout, 3*time.Second),
		LeaderWaitTimeout: parseDurationOrDefault(cfg.Raft.LeaderWaitTimeout, 10*time.Second),
	}

	logger.Info("raft config",
		"enabled", serviceCfg.Enable,
		"node_id", serviceCfg.NodeID,
		"bind", serviceCfg.BindAddr,
		"bootstrap", serviceCfg.Bootstrap,
		"join", serviceCfg.Join,
		"data_dir", serviceCfg.DataDir,
	)
	return NewRaftBadgerService(serviceCfg, logger)
}

func parseDurationOrDefault(raw string, fallback time.Duration) time.Duration {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return fallback
	}
	return duration
}

func lifecycle(lc fx.Lifecycle, service *Service) {
	lc.Append(
		fx.StopHook(func() error {
			return service.Close()
		}),
	)
}
