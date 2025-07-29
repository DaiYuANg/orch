package raft

import (
	"github.com/DaiYuANg/warden/raft"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"os"
	"path/filepath"
)

var Module = fx.Module("raft", fx.Provide(newService), fx.Invoke(lifecycle))

func newService(logger *zap.SugaredLogger) (*store.RaftBadgerService, error) {
	getwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	raftDir := filepath.Join(getwd, "warnden")
	return store.NewRaftBadgerService("node1", raftDir, "db", logger)
}

func lifecycle(lc fx.Lifecycle, service *store.RaftBadgerService) {
	lc.Append(
		fx.StopHook(func() {
			service.Close()
		}),
	)
}
