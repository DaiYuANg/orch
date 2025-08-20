package raft

import (
	"os"
	"path/filepath"

	"github.com/DaiYuANg/warden/libs/pkg"
	"github.com/DaiYuANg/warden/libs/raft"
	"github.com/adrg/xdg"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("raft", fx.Provide(newService), fx.Invoke(lifecycle))

func newService(logger *zap.SugaredLogger) (*raft.Service, error) {
	raftDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		logger.Errorf("mkdir error:%e", err)
	}
	logger.Infof("data path: %s", raftDir)
	nodeId, err := pkg.MachineID()

	if err != nil {
		return nil, err
	}

	return raft.NewRaftBadgerService(nodeId, raftDir, logger)
}

func lifecycle(lc fx.Lifecycle, service *raft.Service) {
	lc.Append(
		fx.StopHook(func() error {
			return service.Close()
		}),
	)
}
