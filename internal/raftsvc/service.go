package raftsvc

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

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
	transport      hraft.Transport

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
	var mkdirErr error
	raftDirs.Range(func(_ int, dir string) bool {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			mkdirErr = oopsx.B("raft").Wrapf(err, "raft mkdir %q", dir)
			return false
		}
		return true
	})
	if mkdirErr != nil {
		return nil, mkdirErr
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

func (s *Service) openRaftTransport(st *raftOpenedStores) (*hraft.NetworkTransport, hraft.ServerAddress, error) {
	bindAddr := strings.TrimSpace(s.cfg.Raft.Bind)
	if bindAddr == "" {
		return nil, "", oopsx.B("raft").Errorf("raft.bind is required")
	}

	var advertise net.Addr
	if raw := strings.TrimSpace(s.cfg.Raft.Advertise); raw != "" {
		addr, err := net.ResolveTCPAddr("tcp", raw)
		if err != nil {
			return nil, "", oopsx.B("raft").Wrapf(err, "resolve raft.advertise %q", raw)
		}
		if addr.IP == nil || addr.IP.IsUnspecified() {
			return nil, "", oopsx.B("raft").Errorf("raft.advertise must be a concrete host:port, got %q", raw)
		}
		advertise = addr
	}

	transport, err := hraft.NewTCPTransportWithLogger(bindAddr, advertise, 3, 10*time.Second, st.raftHC)
	if err != nil {
		return nil, "", oopsx.B("raft").Wrapf(err, "create raft tcp transport (set raft.advertise when raft.bind is 0.0.0.0/:port)")
	}
	return transport, transport.LocalAddr(), nil
}

func validateRaftPeerAddress(label, raw string) (string, error) {
	addr := strings.TrimSpace(raw)
	if addr == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("%s must be host:port: %w", label, err)
	}
	if strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("%s must include a host", label)
	}
	if strings.TrimSpace(port) == "" {
		return "", fmt.Errorf("%s must include a port", label)
	}
	return addr, nil
}

func (s *Service) bootstrapServerList(localID hraft.ServerID, localAddr hraft.ServerAddress) (*list.List[hraft.Server], error) {
	localIDString := strings.TrimSpace(string(localID))
	if localIDString == "" {
		return nil, oopsx.B("raft").Errorf("raft local id is required")
	}
	peers := map[string]string{}
	for rawID, rawAddr := range s.cfg.Raft.Peers {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		addr, err := validateRaftPeerAddress("raft.peers."+id, rawAddr)
		if err != nil {
			return nil, oopsx.B("raft").Wrapf(err, "validate raft peer")
		}
		peers[id] = addr
	}
	if configured, ok := peers[localIDString]; ok && configured != string(localAddr) {
		s.logger.Warn("raft peer address for local node differs from transport advertise address; using transport address",
			"node_id", localIDString,
			"configured", configured,
			"transport", localAddr,
		)
	}
	peers[localIDString] = string(localAddr)

	ids := make([]string, 0, len(peers))
	for id := range peers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	servers := list.NewListWithCapacity[hraft.Server](len(ids))
	for _, id := range ids {
		servers.Add(hraft.Server{
			Suffrage: hraft.Voter,
			ID:       hraft.ServerID(id),
			Address:  hraft.ServerAddress(peers[id]),
		})
	}
	return servers, nil
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
	if !s.cfg.Raft.Bootstrap {
		s.logger.Info("raft bootstrap skipped by config", "node_id", hrCfg.LocalID, "local_addr", localAddr)
		return nil
	}
	bootstrapServers, err := s.bootstrapServerList(hrCfg.LocalID, localAddr)
	if err != nil {
		return err
	}
	configuration := hraft.Configuration{
		Servers: bootstrapServers.Values(),
	}
	if bootstrapErr := hraft.BootstrapCluster(hrCfg, st.logStore, st.stable, st.snapStore, transport, configuration); bootstrapErr != nil {
		return oopsx.B("raft").Wrapf(bootstrapErr, "raft BootstrapCluster")
	}
	return nil
}

// Start opens storx engines and, if needed, bootstraps the configured voter set
// before constructing the Raft instance.
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

	transport, localAddr, err := s.openRaftTransport(st)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
		return err
	}

	hrCfg := hraft.DefaultConfig()
	hrCfg.LocalID = hraft.ServerID(s.localID.String())
	hrCfg.Logger = st.raftHC

	hasState, err := hraft.HasExistingState(logStore, stable, snapStore)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return transport.Close() }, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
		return oopsx.B("raft").Wrapf(err, "raft HasExistingState")
	}

	if bootstrapErr := s.maybeBootstrapRaft(hrCfg, st, localAddr, transport, hasState); bootstrapErr != nil {
		warnRaftCleanup(s.logger, func() error { return transport.Close() }, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
		return bootstrapErr
	}

	node, err := hraft.NewRaft(hrCfg, s.fsm, logStore, stable, snapStore, transport)
	if err != nil {
		warnRaftCleanup(s.logger, func() error { return transport.Close() }, func() error { return bbolt.Close() }, func() error { return bgx.Close() })
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
		"bind", s.cfg.Raft.Bind,
		"advertise", localAddr,
		"configured_peers", len(s.cfg.Raft.Peers),
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

func (s *Service) closeRaftTransportSilently() {
	if s.transport == nil {
		return
	}
	if closer, ok := s.transport.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			s.logger.Warn("close raft transport", "error", err)
		}
	}
	s.transport = nil
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
}

// Stop shuts Raft down then closes embedded databases.
func (s *Service) Stop(_ context.Context) error {
	if !s.cfg.Raft.Enabled {
		return nil
	}
	s.shutdownRaftInstance()
	s.closeRaftTransportSilently()
	s.closeRaftStorageSilently()
	s.clearRaftReferences()
	if s.started.Load() {
		s.started.Store(false)
		s.logger.Info("raft stopped")
	}
	return nil
}
