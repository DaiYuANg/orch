package raftsvc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	hclog "github.com/hashicorp/go-hclog"

	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/bboltx"
	badger "github.com/dgraph-io/badger/v4"
	hraft "github.com/hashicorp/raft"

	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// Service owns hashicorp raft backed by storx (Badger logs + bbolt stable metadata).
type Service struct {
	logger  *slog.Logger
	cfg     config.Config
	localID nodeid.Local

	r              *hraft.Raft
	fsm            *schedulingFSM
	deploySignalCh chan struct{}
	badgerDB       *badgerx.DB
	bboltDB        *bboltx.DB
	logStore       *storxBadgerLogStore
	stable         *storxBoltStableStore
	transport      *hraft.InmemTransport

	started atomic.Bool
}

// New constructs the service (Raft starts in Start).
func New(cfg config.Config, logger *slog.Logger, local nodeid.Local) *Service {
	ch := make(chan struct{}, 1)
	fsm := &schedulingFSM{}
	s := &Service{
		logger:         logger,
		cfg:            cfg,
		localID:        local,
		fsm:            fsm,
		deploySignalCh: ch,
	}
	fsm.setNotifyDeploy(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return s
}

func warnRaftCleanup(logger *slog.Logger, cleaners ...func() error) {
	for _, fn := range cleaners {
		if fn == nil {
			continue
		}
		if err := fn(); err != nil {
			logger.Warn("raft startup cleanup", "error", err)
		}
	}
}

type raftOpenedStores struct {
	bgx       *badgerx.DB
	bbolt     *bboltx.DB
	logStore  *storxBadgerLogStore
	stable    *storxBoltStableStore
	snapStore hraft.SnapshotStore
	raftHC    hclog.Logger
}

func (s *Service) openRaftStores() (*raftOpenedStores, error) {
	raftDirs := list.NewList(
		s.cfg.Raft.Badger.Dir,
		filepath.Dir(s.cfg.Raft.Bolt.Path),
		s.cfg.Raft.Snapshot.Dir,
	)
	for _, dir := range raftDirs.Values() {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, oopsx.B("raft").Wrapf(err, "raft mkdir %q", dir)
		}
	}

	opts := badger.DefaultOptions(s.cfg.Raft.Badger.Dir)
	opts.Logger = logging.Badger(s.logger.With(slog.String("engine", "badger"), slog.String("use", "raft-log")))

	bgx, err := badgerx.Open(opts, badgerx.WithDBLogger(s.logger))
	if err != nil {
		return nil, oopsx.B("raft").Wrapf(err, "open raft badger (storx)")
	}

	bbolt, err := bboltx.Open(s.cfg.Raft.Bolt.Path, 0o600, nil, bboltx.WithDBLogger(s.logger))
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return bgx.Close() })
		return nil, oopsx.B("raft").Wrapf(err, "open raft bolt (storx)")
	}

	raftHC := logging.HCLogger(s.logger, "raft")
	st := &raftOpenedStores{
		bgx:      bgx,
		bbolt:    bbolt,
		logStore: newStorxBadgerLogStore(bgx),
		stable:   newStorxBoltStableStore(bbolt),
		raftHC:   raftHC,
	}
	st.snapStore, err = hraft.NewFileSnapshotStoreWithLogger(s.cfg.Raft.Snapshot.Dir, 3, raftHC)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
		return nil, oopsx.B("raft").Wrapf(err, "open raft snapshots")
	}
	return st, nil
}

func (s *Service) maybeBootstrapRaft(
	hrCfg *hraft.Config,
	st *raftOpenedStores,
	localAddr hraft.ServerAddress,
	transport hraft.Transport,
	hasState bool,
) error {
	if hasState {
		return nil
	}
	bootstrapServers := list.NewList(hraft.Server{
		Suffrage: hraft.Voter,
		ID:       hrCfg.LocalID,
		Address:  localAddr,
	})
	configuration := hraft.Configuration{
		Servers: bootstrapServers.Values(),
	}
	if bootstrapErr := hraft.BootstrapCluster(hrCfg, st.logStore, st.stable, st.snapStore, transport, configuration); bootstrapErr != nil {
		warnRaftCleanup(s.logger, func() error { return st.bbolt.Close() }, func() error { return st.bgx.Close() })
		return oopsx.B("raft").Wrapf(bootstrapErr, "raft BootstrapCluster")
	}
	return nil
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

	st, err := s.openRaftStores()
	if err != nil {
		return err
	}
	bgx, bbolt := st.bgx, st.bbolt
	logStore, stable, snapStore := st.logStore, st.stable, st.snapStore

	localAddr, transport := hraft.NewInmemTransport("")

	hrCfg := hraft.DefaultConfig()
	hrCfg.LocalID = hraft.ServerID(s.localID.String())
	hrCfg.Logger = st.raftHC

	hasState, err := hraft.HasExistingState(logStore, stable, snapStore)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
		return oopsx.B("raft").Wrapf(err, "raft HasExistingState")
	}

	if bootstrapErr := s.maybeBootstrapRaft(hrCfg, st, localAddr, transport, hasState); bootstrapErr != nil {
		return bootstrapErr
	}

	node, err := hraft.NewRaft(hrCfg, s.fsm, logStore, stable, snapStore, transport)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
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

func (s *Service) shutdownRaftInstance() {
	if s.r == nil {
		return
	}
	if err := s.r.Shutdown().Error(); err != nil {
		s.logger.Warn("raft shutdown", "error", err)
	}
	s.r = nil
}

func (s *Service) closeRaftStorageSilently() {
	if s.badgerDB != nil {
		if err := s.badgerDB.Close(); err != nil {
			s.logger.Warn("close raft badger", "error", err)
		}
		s.badgerDB = nil
	}
	if s.bboltDB != nil {
		if err := s.bboltDB.Close(); err != nil {
			s.logger.Warn("close raft bolt", "error", err)
		}
		s.bboltDB = nil
	}
}

func (s *Service) clearRaftReferences() {
	s.logStore = nil
	s.stable = nil
	s.transport = nil
}

// Stop shuts Raft down then closes embedded databases.
func (s *Service) Stop(_ context.Context) error {
	if !s.cfg.Raft.Enabled {
		return nil
	}
	s.shutdownRaftInstance()
	s.closeRaftStorageSilently()
	s.clearRaftReferences()
	if s.started.Load() {
		s.started.Store(false)
		s.logger.Info("raft stopped")
	}
	return nil
}
