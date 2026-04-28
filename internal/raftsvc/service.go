package raftsvc

import (
	"context"
	"fmt"
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
		s.cfg.Raft.BadgerDir,
		filepath.Dir(s.cfg.Raft.BoltPath),
		s.cfg.Raft.SnapshotDir,
	)
	for _, dir := range raftDirs.Values() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("raft mkdir %q: %w", dir, err)
		}
	}

	opts := badger.DefaultOptions(s.cfg.Raft.BadgerDir)
	opts.Logger = logging.Badger(s.logger.With(slog.String("engine", "badger"), slog.String("use", "raft-log")))

	bgx, err := badgerx.Open(opts, badgerx.WithDBLogger(s.logger))
	if err != nil {
		return fmt.Errorf("open raft badger (storx): %w", err)
	}

	bbolt, err := bboltx.Open(s.cfg.Raft.BoltPath, 0o600, nil, bboltx.WithDBLogger(s.logger))
	if err != nil {
		_ = bgx.Close()
		return fmt.Errorf("open raft bolt (storx): %w", err)
	}

	logStore := newStorxBadgerLogStore(bgx)
	stable := newStorxBoltStableStore(bbolt)

	raftHC := logging.HCLogger(s.logger, "raft")

	snapStore, err := hraft.NewFileSnapshotStoreWithLogger(s.cfg.Raft.SnapshotDir, 3, raftHC)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return fmt.Errorf("open raft snapshots: %w", err)
	}

	localAddr, transport := hraft.NewInmemTransport("")

	hrCfg := hraft.DefaultConfig()
	hrCfg.LocalID = hraft.ServerID(s.cfg.Raft.NodeID)
	hrCfg.Logger = raftHC

	hasState, err := hraft.HasExistingState(logStore, stable, snapStore)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return fmt.Errorf("raft HasExistingState: %w", err)
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
			return fmt.Errorf("raft BootstrapCluster: %w", err)
		}
	}

	node, err := hraft.NewRaft(hrCfg, s.fsm, logStore, stable, snapStore, transport)
	if err != nil {
		_ = bbolt.Close()
		_ = bgx.Close()
		return fmt.Errorf("raft.NewRaft: %w", err)
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
		"badger_dir", s.cfg.Raft.BadgerDir,
		"bolt_path", s.cfg.Raft.BoltPath,
		"snapshot_dir", s.cfg.Raft.SnapshotDir,
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
