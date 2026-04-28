package raftsvc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/bboltx"
	badger "github.com/dgraph-io/badger/v4"
	hraft "github.com/hashicorp/raft"

	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// Service owns hashicorp raft backed by storx (Badger logs + bbolt stable metadata).
type Service struct {
	logger *slog.Logger
	cfg    config.Config

	r         *hraft.Raft
	fsm       *schedulingFSM
	badgerDB  *badgerx.DB
	bboltDB   *bboltx.DB
	logStore  *storxBadgerLogStore
	stable    *storxBoltStableStore
	transport *hraft.InmemTransport

	started atomic.Bool
}

// New constructs the service (Raft starts in Start).
func New(cfg config.Config, logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
		cfg:    cfg,
		fsm:    &schedulingFSM{},
	}
}

// Start opens storx engines and, if needed, bootstraps a single-node cluster before
// constructing the Raft instance.
func (s *Service) Start(_ context.Context) error {
	if !s.cfg.Raft.Enabled {
		s.logger.Info("raft disabled by config")
		return nil
	}
	if s.started.Load() {
		return nil
	}

	raftDirs := list.NewList(
		s.cfg.Raft.Badger.Dir,
		filepath.Dir(s.cfg.Raft.Bolt.Path),
		s.cfg.Raft.Snapshot.Dir,
	)
	for _, dir := range raftDirs.Values() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return oopsx.B("raft").Wrapf(err, "raft mkdir %q", dir)
		}
	}

	opts := badger.DefaultOptions(s.cfg.Raft.Badger.Dir)
	opts.Logger = logging.Badger(s.logger.With(slog.String("engine", "badger"), slog.String("use", "raft-log")))

	bgx, err := badgerx.Open(opts, badgerx.WithDBLogger(s.logger))
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "open raft badger (storx)")
	}

	bbolt, err := bboltx.Open(s.cfg.Raft.Bolt.Path, 0o600, nil, bboltx.WithDBLogger(s.logger))
	if err != nil {
		_ = bgx.Close()
		return oopsx.B("raft").Wrapf(err, "open raft bolt (storx)")
	}

	logStore := newStorxBadgerLogStore(bgx)
	stable := newStorxBoltStableStore(bbolt)

	raftHC := logging.HCLogger(s.logger, "raft")

	snapStore, err := hraft.NewFileSnapshotStoreWithLogger(s.cfg.Raft.Snapshot.Dir, 3, raftHC)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return oopsx.B("raft").Wrapf(err, "open raft snapshots")
	}

	localAddr, transport := hraft.NewInmemTransport("")

	hrCfg := hraft.DefaultConfig()
	hrCfg.LocalID = hraft.ServerID(s.cfg.Raft.Node.ID)
	hrCfg.Logger = raftHC

	hasState, err := hraft.HasExistingState(logStore, stable, snapStore)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return oopsx.B("raft").Wrapf(err, "raft HasExistingState")
	}

	if !hasState {
		bootstrapServers := list.NewList(hraft.Server{
			Suffrage: hraft.Voter,
			ID:       hrCfg.LocalID,
			Address:  localAddr,
		})
		configuration := hraft.Configuration{
			Servers: bootstrapServers.Values(),
		}
		if err := hraft.BootstrapCluster(hrCfg, logStore, stable, snapStore, transport, configuration); err != nil {
			_ = bbolt.Close()
			_ = bgx.Close()
			return oopsx.B("raft").Wrapf(err, "raft BootstrapCluster")
		}
	}

	node, err := hraft.NewRaft(hrCfg, s.fsm, logStore, stable, snapStore, transport)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return oopsx.B("raft").Wrapf(err, "raft.NewRaft")
	}

	s.badgerDB = bgx
	s.bboltDB = bbolt
	s.logStore = logStore
	s.stable = stable
	s.transport = transport
	s.r = node
	s.started.Store(true)

	s.logger.Info("raft started",
		"node_id", hrCfg.LocalID,
		"badger_dir", s.cfg.Raft.Badger.Dir,
		"bolt_path", s.cfg.Raft.Bolt.Path,
		"snapshot_dir", s.cfg.Raft.Snapshot.Dir,
	)
	return nil
}

// Stop shuts Raft down then closes embedded databases.
func (s *Service) Stop(_ context.Context) error {
	if !s.cfg.Raft.Enabled {
		return nil
	}
	if s.r != nil {
		_ = s.r.Shutdown().Error()
		s.r = nil
	}
	if s.badgerDB != nil {
		_ = s.badgerDB.Close()
		s.badgerDB = nil
	}
	if s.bboltDB != nil {
		_ = s.bboltDB.Close()
		s.bboltDB = nil
	}
	s.logStore = nil
	s.stable = nil
	s.transport = nil
	if s.started.Load() {
		s.started.Store(false)
		s.logger.Info("raft stopped")
	}
	return nil
}
