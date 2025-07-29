package raft

import (
	"github.com/DaiYuANg/warden/pkg"
	"github.com/DaiYuANg/warden/raft"
	"github.com/adrg/xdg"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
	"path/filepath"
)

var Module = fx.Module("raft", fx.Provide(newService), fx.Invoke(lifecycle))

func newService(logger *zap.SugaredLogger) (*store.RaftBadgerService, error) {
	raftDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		logger.Errorf("mkdir error:%e", err)
	}
	logger.Infof("data path: %s", raftDir)
	nodeId := pkg.GenerateNodeID()

	dbDir := filepath.Join(raftDir, "db")
	return store.NewRaftBadgerService(nodeId, raftDir, dbDir, logger)
}

func lifecycle(lc fx.Lifecycle, service *store.RaftBadgerService) {
	lc.Append(
		fx.StopHook(func() error {
			return service.Close()
		}),
	)
}
