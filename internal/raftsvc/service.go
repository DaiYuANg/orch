package raftsvc

import (
	"context"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arcgolabs/collectionx/list"
	dragonboat "github.com/lni/dragonboat/v4"
	dbconfig "github.com/lni/dragonboat/v4/config"
	sm "github.com/lni/dragonboat/v4/statemachine"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	controlShardID uint64 = 1
)

// Service owns the Dragonboat NodeHost used for replicated control-plane state.
type Service struct {
	logger  *slog.Logger
	cfg     config.Config
	localID nodeid.Local

	nh             *dragonboat.NodeHost
	fsm            *schedulingFSM
	deploySignalCh chan struct{}

	localReplicaID uint64
	localAddress   string

	mu struct {
		sync.RWMutex
		replicaToNode map[uint64]string
		addressToNode map[string]string
	}

	started atomic.Bool
}

// New constructs the service (Dragonboat starts in Start).
func New(cfg config.Config, logger *slog.Logger, local nodeid.Local) *Service {
	logging.InstallDragonboatLogger(logger)
	ch := make(chan struct{}, 1)
	fsm := &schedulingFSM{}
	s := &Service{
		logger:         logger,
		cfg:            cfg,
		localID:        local,
		fsm:            fsm,
		deploySignalCh: ch,
	}
	s.mu.replicaToNode = map[uint64]string{}
	s.mu.addressToNode = map[string]string{}
	fsm.setNotifyDeploy(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return s
}

func replicaIDForNodeID(id string) (uint64, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return 0, oopsx.B("raft").Errorf("raft node id is required")
	}
	h := fnv.New64a()
	_, _ = io.WriteString(h, id)
	n := h.Sum64()
	if n == 0 {
		n = 1
	}
	return n, nil
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

func validateConcreteRaftAddress(label, raw string) (string, error) {
	addr, err := validateRaftPeerAddress(label, raw)
	if err != nil {
		return "", err
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip != nil && ip.IsUnspecified() {
		return "", fmt.Errorf("%s must be a concrete host:port, got %q", label, raw)
	}
	return addr, nil
}

func resolveEphemeralTCPAddr(raw string) (string, error) {
	_, port, err := net.SplitHostPort(strings.TrimSpace(raw))
	if err != nil || port != "0" {
		return raw, nil
	}
	ln, err := net.Listen("tcp", raw)
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	if closeErr := ln.Close(); closeErr != nil {
		return "", closeErr
	}
	return addr, nil
}

func (s *Service) dragonboatDataDir() string {
	if dir := strings.TrimSpace(s.cfg.Raft.Data.Dir); dir != "" {
		return dir
	}
	return filepath.Join(config.DefaultDataRoot(), "dragonboat")
}

func (s *Service) dragonboatAddresses() (listenAddr string, raftAddr string, err error) {
	bindAddr := strings.TrimSpace(s.cfg.Raft.Bind)
	if bindAddr == "" {
		return "", "", oopsx.B("raft").Errorf("raft.bind is required")
	}
	bindAddr, err = validateRaftPeerAddress("raft.bind", bindAddr)
	if err != nil {
		return "", "", oopsx.B("raft").Wrapf(err, "validate raft bind")
	}
	listenAddr, err = resolveEphemeralTCPAddr(bindAddr)
	if err != nil {
		return "", "", oopsx.B("raft").Wrapf(err, "reserve raft bind address")
	}
	advertise := strings.TrimSpace(s.cfg.Raft.Advertise)
	if advertise == "" {
		raftAddr, err = validateConcreteRaftAddress("raft.bind", listenAddr)
	} else {
		raftAddr, err = validateConcreteRaftAddress("raft.advertise", advertise)
	}
	if err != nil {
		return "", "", oopsx.B("raft").Wrapf(err, "validate raft address")
	}
	return listenAddr, raftAddr, nil
}

func (s *Service) rememberMember(id string, replicaID uint64, address string) {
	id = strings.TrimSpace(id)
	address = strings.TrimSpace(address)
	if id == "" || replicaID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mu.replicaToNode[replicaID] = id
	if address != "" {
		s.mu.addressToNode[address] = id
	}
}

func (s *Service) forgetMember(replicaID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mu.replicaToNode, replicaID)
}

func (s *Service) nodeIDForMember(replicaID uint64, address string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.mu.replicaToNode[replicaID]; ok && strings.TrimSpace(id) != "" {
		return id
	}
	if id, ok := s.mu.addressToNode[strings.TrimSpace(address)]; ok && strings.TrimSpace(id) != "" {
		return id
	}
	return fmt.Sprintf("replica-%d", replicaID)
}

func (s *Service) bootstrapReplicaTargets(localReplicaID uint64, localAddr string) (map[uint64]dragonboat.Target, error) {
	targets := map[uint64]dragonboat.Target{}
	seenIDs := map[uint64]string{}
	localID := strings.TrimSpace(s.localID.String())
	if localID == "" {
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
	if configured, ok := peers[localID]; ok && configured != localAddr {
		s.logger.Warn("raft peer address for local node differs from transport advertise address; using transport address",
			"node_id", localID,
			"configured", configured,
			"transport", localAddr,
		)
	}
	peers[localID] = localAddr

	ids := make([]string, 0, len(peers))
	for id := range peers {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		replicaID, err := replicaIDForNodeID(id)
		if err != nil {
			return nil, err
		}
		if prev, exists := seenIDs[replicaID]; exists && prev != id {
			return nil, oopsx.B("raft").Errorf("raft node ids %q and %q resolve to the same dragonboat replica id %d", prev, id, replicaID)
		}
		seenIDs[replicaID] = id
		targets[replicaID] = dragonboat.Target(peers[id])
		s.rememberMember(id, replicaID, peers[id])
	}
	if _, ok := targets[localReplicaID]; !ok {
		targets[localReplicaID] = dragonboat.Target(localAddr)
		s.rememberMember(localID, localReplicaID, localAddr)
	}
	return targets, nil
}

// bootstrapServerList is kept as a small testable compatibility helper around
// the configured static peers used to bootstrap Dragonboat's initial members.
func (s *Service) bootstrapServerList(localID string, localAddr string) (*list.List[Member], error) {
	replicaID, err := replicaIDForNodeID(localID)
	if err != nil {
		return nil, err
	}
	targets, err := s.bootstrapReplicaTargets(replicaID, localAddr)
	if err != nil {
		return nil, err
	}
	members := list.NewListWithCapacity[Member](len(targets))
	for rid, target := range targets {
		addr := string(target)
		members.Add(Member{
			ID:       s.nodeIDForMember(rid, addr),
			Address:  addr,
			Suffrage: "Voter",
		})
	}
	members.Sort(func(a, b Member) int {
		return strings.Compare(a.ID, b.ID)
	})
	return members, nil
}

func (s *Service) dragonboatReplicaConfig(replicaID uint64) dbconfig.Config {
	return dbconfig.Config{
		ReplicaID:          replicaID,
		ShardID:            controlShardID,
		CheckQuorum:        true,
		PreVote:            true,
		HeartbeatRTT:       1,
		ElectionRTT:        10,
		SnapshotEntries:    10_000,
		CompactionOverhead: 1_000,
		WaitReady:          true,
	}
}

func (s *Service) startReplica(nh *dragonboat.NodeHost, replicaID uint64, raftAddr string) error {
	hasNodeInfo := nh.HasNodeInfo(controlShardID, replicaID)
	initialMembers := map[uint64]dragonboat.Target{}
	join := false
	if !hasNodeInfo {
		if s.cfg.Raft.Bootstrap {
			var err error
			initialMembers, err = s.bootstrapReplicaTargets(replicaID, raftAddr)
			if err != nil {
				return err
			}
		} else {
			join = true
		}
	}
	return nh.StartReplica(initialMembers, join, func(shardID, rid uint64) sm.IStateMachine {
		return s.fsm
	}, s.dragonboatReplicaConfig(replicaID))
}

// Start opens a Dragonboat NodeHost and starts the control-plane Raft shard replica.
func (s *Service) Start(_ context.Context) error {
	if s.started.Load() {
		return nil
	}

	replicaID, err := replicaIDForNodeID(s.localID.String())
	if err != nil {
		return err
	}
	listenAddr, raftAddr, err := s.dragonboatAddresses()
	if err != nil {
		return err
	}

	dataDir := s.dragonboatDataDir()
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return oopsx.B("raft").Wrapf(err, "dragonboat mkdir %q", dataDir)
	}

	nh, err := dragonboat.NewNodeHost(dbconfig.NodeHostConfig{
		WALDir:         filepath.Join(dataDir, "wal"),
		NodeHostDir:    filepath.Join(dataDir, "nodehost"),
		RTTMillisecond: 100,
		RaftAddress:    raftAddr,
		ListenAddress:  listenAddr,
	})
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "dragonboat.NewNodeHost")
	}

	s.localReplicaID = replicaID
	s.localAddress = raftAddr
	s.rememberMember(s.localID.String(), replicaID, raftAddr)

	if err := s.startReplica(nh, replicaID, raftAddr); err != nil {
		nh.Close()
		return oopsx.B("raft").Wrapf(err, "dragonboat StartReplica")
	}

	s.nh = nh
	s.started.Store(true)

	s.logger.Info("raft started",
		"engine", "dragonboat",
		"node_id", s.localID.String(),
		"replica_id", replicaID,
		"bind", listenAddr,
		"advertise", raftAddr,
		"configured_peers", len(s.cfg.Raft.Peers),
		"data_dir", dataDir,
	)
	return nil
}

func (s *Service) isLocalLeader() bool {
	if s == nil || s.nh == nil || s.localReplicaID == 0 {
		return false
	}
	leaderID, _, ready, err := s.nh.GetLeaderID(controlShardID)
	return err == nil && ready && leaderID == s.localReplicaID
}

func (s *Service) applyCommand(data []byte, timeout time.Duration, notLeaderMessage string) error {
	if s == nil {
		return oopsx.B("raft").Errorf("nil service")
	}
	if s.nh == nil {
		// Tests can exercise the FSM without opening a Dragonboat NodeHost.
		// Production startup always calls Start before serving control-plane APIs.
		s.fsm.applyCommandPayload(data)
		return nil
	}
	if !s.isLocalLeader() {
		return s.notLeaderError(notLeaderMessage)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := s.nh.SyncPropose(ctx, s.nh.GetNoOPSession(controlShardID), data)
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "dragonboat propose")
	}
	return nil
}

func (s *Service) notLeaderError(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "raft local node is not leader"
	}
	if s == nil || s.nh == nil {
		return oopsx.B("raft").Errorf("%s: raft is not ready", message)
	}
	leaderID, _, ready, err := s.nh.GetLeaderID(controlShardID)
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "%s: read raft leader", message)
	}
	if !ready || leaderID == 0 {
		return oopsx.B("raft").Errorf("%s: raft leader is not ready", message)
	}
	return oopsx.B("raft").Errorf("%s: local node is follower, leader=%s", message, s.nodeIDForMember(leaderID, ""))
}

func withDefaultDeadline(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

// Stop shuts Dragonboat down.
func (s *Service) Stop(_ context.Context) error {
	if s.nh != nil {
		s.nh.Close()
		s.nh = nil
	}
	if s.started.Load() {
		s.started.Store(false)
		s.logger.Info("raft stopped")
	}
	return nil
}
