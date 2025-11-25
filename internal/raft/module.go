package raft

import (
	"os"
	"path/filepath"

	"github.com/DaiYuANg/warden/pkg"
	"github.com/adrg/xdg"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module("raft", fx.Provide(newService), fx.Invoke(lifecycle))

func newService(logger *zap.SugaredLogger) (*Service, error) {
	raftDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		logger.Errorf("mkdir error:%e", err)
	}
	logger.Infof("data path: %s", raftDir)
	nodeId, err := pkg.MachineID()

	if err != nil {
		return nil, err
	}

	return NewRaftBadgerService(nodeId, raftDir, logger)
}

func lifecycle(lc fx.Lifecycle, service *Service) {
	lc.Append(
		fx.StopHook(func() error {
			return service.Close()
		}),
	)
}
