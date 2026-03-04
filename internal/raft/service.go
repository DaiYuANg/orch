package raft

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/raft"
	"github.com/samber/lo"
	"go.etcd.io/bbolt"
)

var (
	ErrRaftDisabled = errors.New("raft is disabled")
	ErrNotLeader    = errors.New("raft node is not leader")
)

type ServiceConfig struct {
	Enable            bool
	NodeID            string
	BindAddr          string
	DataDir           string
	Bootstrap         bool
	Join              []string
	ApplyTimeout      time.Duration
	LeaderWaitTimeout time.Duration
}

type Service struct {
	enabled           bool
	raft              *Manager
	nodeID            string
	bindAddr          string
	join              []string
	applyTimeout      time.Duration
	leaderWaitTimeout time.Duration
	logger            *slog.Logger
}

type Server struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Suffrage string `json:"suffrage"`
}

type Status struct {
	Enabled bool     `json:"enabled"`
	NodeID  string   `json:"node_id"`
	Bind    string   `json:"bind"`
	Leader  string   `json:"leader"`
	Role    string   `json:"role"`
	Servers []Server `json:"servers"`
}

func NewRaftBadgerService(cfg ServiceConfig, logger *slog.Logger) (*Service, error) {
	service := &Service{
		enabled:           cfg.Enable,
		nodeID:            cfg.NodeID,
		bindAddr:          cfg.BindAddr,
		join:              compactJoin(cfg.Join),
		applyTimeout:      cfg.ApplyTimeout,
		leaderWaitTimeout: cfg.LeaderWaitTimeout,
		logger:            logger,
	}

	if !cfg.Enable {
		return service, nil
	}

	raftMgr, err := NewRaftManager(ManagerConfig{
		NodeID:    cfg.NodeID,
		BindAddr:  cfg.BindAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: cfg.Bootstrap,
		Join:      cfg.Join,
	}, logger)
	if err != nil {
		return nil, err
	}

	service.raft = raftMgr
	if err := service.waitForLeader(cfg.LeaderWaitTimeout); err != nil {
		logger.Warn("raft leader not ready yet", "error", err)
	}
	if err := service.joinPeers(); err != nil {
		logger.Warn("raft join peers failed", "error", err)
	}
	return service, nil
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Service) NodeID() string {
	if s == nil {
		return ""
	}
	return s.nodeID
}

func (s *Service) IsLeader() bool {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return false
	}
	return s.raft.raftNode.State() == raft.Leader
}

func (s *Service) Leader() string {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return ""
	}
	return string(s.raft.raftNode.Leader())
}

func (s *Service) State() string {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return "disabled"
	}
	return s.raft.raftNode.State().String()
}

func (s *Service) Status() (Status, error) {
	status := Status{
		Enabled: s.Enabled(),
		NodeID:  s.nodeID,
		Bind:    s.bindAddr,
		Leader:  s.Leader(),
		Role:    s.State(),
	}
	if !s.Enabled() || s.raft == nil {
		return status, nil
	}
	servers, err := s.ListServers()
	if err != nil {
		return Status{}, err
	}
	status.Servers = servers
	return status, nil
}

func (s *Service) MetadataDB() *bbolt.DB {
	if !s.Enabled() || s.raft == nil {
		return nil
	}
	return s.raft.MetadataDB()
}

func (s *Service) Read(bucket, key string) ([]byte, error) {
	if !s.Enabled() || s.raft == nil {
		return nil, ErrRaftDisabled
	}
	return s.raft.Read(bucket, key)
}

func (s *Service) ApplySet(bucket, key string, value []byte) error {
	if !s.Enabled() || s.raft == nil {
		return ErrRaftDisabled
	}
	if !s.IsLeader() {
		return ErrNotLeader
	}
	raw, err := encodeCommand(newSetCommand(bucket, key, value))
	if err != nil {
		return err
	}
	return s.raft.ApplyLog(raw, s.applyTimeout)
}

func (s *Service) ApplyDelete(bucket, key string) error {
	if !s.Enabled() || s.raft == nil {
		return ErrRaftDisabled
	}
	if !s.IsLeader() {
		return ErrNotLeader
	}
	raw, err := encodeCommand(newDeleteCommand(bucket, key))
	if err != nil {
		return err
	}
	return s.raft.ApplyLog(raw, s.applyTimeout)
}

func (s *Service) ListServers() ([]Server, error) {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return []Server{}, ErrRaftDisabled
	}

	configuration := s.raft.raftNode.GetConfiguration()
	if err := configuration.Error(); err != nil {
		return nil, err
	}

	servers := lo.Map(configuration.Configuration().Servers, func(item raft.Server, _ int) Server {
		return Server{
			ID:       string(item.ID),
			Address:  string(item.Address),
			Suffrage: item.Suffrage.String(),
		}
	})
	sort.SliceStable(servers, func(i, j int) bool {
		return servers[i].ID < servers[j].ID
	})
	return servers, nil
}

func (s *Service) AddVoter(id, address string) error {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return ErrRaftDisabled
	}
	if !s.IsLeader() {
		return ErrNotLeader
	}

	serverID := strings.TrimSpace(id)
	serverAddress := strings.TrimSpace(address)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	if serverAddress == "" {
		return fmt.Errorf("server address is required")
	}

	servers, err := s.ListServers()
	if err != nil {
		return err
	}
	if lo.ContainsBy(servers, func(item Server) bool {
		return item.ID == serverID || item.Address == serverAddress
	}) {
		return nil
	}

	future := s.raft.raftNode.AddVoter(raft.ServerID(serverID), raft.ServerAddress(serverAddress), 0, 0)
	return future.Error()
}

func (s *Service) RemoveServer(id string) error {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return ErrRaftDisabled
	}
	if !s.IsLeader() {
		return ErrNotLeader
	}

	serverID := strings.TrimSpace(id)
	if serverID == "" {
		return fmt.Errorf("server id is required")
	}
	future := s.raft.raftNode.RemoveServer(raft.ServerID(serverID), 0, 0)
	return future.Error()
}

func (s *Service) WaitForLeader(ctx context.Context) error {
	if !s.Enabled() {
		return ErrRaftDisabled
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		if leader := s.Leader(); strings.TrimSpace(leader) != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) Close() error {
	if !s.Enabled() || s.raft == nil || s.raft.raftNode == nil {
		return nil
	}
	err1 := s.raft.raftNode.Shutdown().Error()
	err2 := s.raft.fsm.close()
	return errors.Join(err1, err2)
}

func (s *Service) waitForLeader(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.WaitForLeader(ctx)
}

func (s *Service) joinPeers() error {
	if !s.Enabled() || s.raft == nil || len(s.join) == 0 {
		return nil
	}
	if !s.IsLeader() {
		return nil
	}

	servers, err := s.ListServers()
	if err != nil {
		return err
	}
	existing := lo.Associate(servers, func(item Server) (string, struct{}) {
		return item.Address, struct{}{}
	})

	for _, peer := range s.join {
		if peer == s.bindAddr {
			continue
		}
		if _, ok := existing[peer]; ok {
			continue
		}
		if err := s.AddVoter(peer, peer); err != nil {
			return fmt.Errorf("add raft voter %s: %w", peer, err)
		}
	}
	return nil
}

func compactJoin(peers []string) []string {
	items := lo.FilterMap(peers, func(peer string, _ int) (string, bool) {
		trimmed := strings.TrimSpace(peer)
		return trimmed, trimmed != ""
	})
	return lo.Uniq(items)
}
