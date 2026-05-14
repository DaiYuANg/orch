package gossipsvc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
	"github.com/hashicorp/memberlist"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/nodeid"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type Service struct {
	logger *slog.Logger
	cfg    config.Config
	local  nodeid.Local
	raft   *raftsvc.Service

	members *mapping.ShardedConcurrentMap[string, Node]
	events  chan memberlist.NodeEvent
	ml      *memberlist.Memberlist

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started atomic.Bool
}

func New(cfg config.Config, logger *slog.Logger, local nodeid.Local, raft *raftsvc.Service) *Service {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{
		logger:  logger,
		cfg:     cfg,
		local:   local,
		raft:    raft,
		members: mapping.NewShardedConcurrentMap[string, Node](0, mapping.HashString),
		events:  make(chan memberlist.NodeEvent, 256),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if s == nil || !s.cfg.Gossip.Enabled {
		return nil
	}
	if s.started.Load() {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return oopsx.B("gossip").Wrapf(err, "start context")
	}
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.cancel = cancel
	s.wg.Add(1)
	go s.eventLoop(runCtx)

	cfg, err := s.memberlistConfig()
	if err != nil {
		cancel()
		s.wg.Wait()
		return err
	}
	ml, err := memberlist.Create(cfg)
	if err != nil {
		cancel()
		s.wg.Wait()
		return oopsx.B("gossip").Wrapf(err, "create memberlist")
	}
	s.ml = ml
	s.started.Store(true)
	s.refreshMembers()

	s.wg.Add(1)
	go s.reconcileLoop(runCtx)

	s.logger.Info("gossip started",
		"node_id", s.local.String(),
		"bind", s.cfg.Gossip.Bind,
		"advertise", s.cfg.Gossip.Advertise,
		"seeds", normalizeSeeds(s.cfg.Gossip.Seeds).Len(),
		"auto_join_raft", s.cfg.Gossip.AutoJoinRaft,
	)
	return nil
}

func (s *Service) Stop(context.Context) error {
	if s == nil || !s.started.Load() {
		return nil
	}
	if s.ml != nil {
		if err := s.ml.Leave(2 * time.Second); err != nil {
			s.logger.Warn("gossip leave failed", "error", err)
		}
		if err := s.ml.Shutdown(); err != nil {
			s.logger.Warn("gossip shutdown failed", "error", err)
		}
		s.ml = nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	s.started.Store(false)
	s.logger.Info("gossip stopped")
	return nil
}

func (s *Service) Members() *list.List[Node] {
	if s == nil {
		return list.NewList[Node]()
	}
	s.refreshMembers()
	out := list.NewListWithCapacity[Node](s.members.Len())
	s.members.Range(func(_ string, node Node) bool {
		out.Add(node)
		return true
	})
	out.Sort(func(a, b Node) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func (s *Service) memberlistConfig() (*memberlist.Config, error) {
	if strings.TrimSpace(s.local.String()) == "" {
		return nil, oopsx.B("gossip").Errorf("local node id is required")
	}
	bindHost, bindPort, err := splitGossipAddress("gossip.bind", s.cfg.Gossip.Bind)
	if err != nil {
		return nil, err
	}
	cfg := memberlist.DefaultLANConfig()
	cfg.Name = s.local.String()
	cfg.BindAddr = bindHost
	cfg.BindPort = bindPort
	cfg.LogOutput = io.Discard
	cfg.Delegate = delegate{service: s}
	cfg.Events = &memberlist.ChannelEventDelegate{Ch: s.events}
	if advertise := strings.TrimSpace(s.cfg.Gossip.Advertise); advertise != "" {
		advertiseHost, advertisePort, splitErr := splitGossipAddress("gossip.advertise", advertise)
		if splitErr != nil {
			return nil, splitErr
		}
		cfg.AdvertiseAddr = advertiseHost
		cfg.AdvertisePort = advertisePort
	}
	secret, err := gossipSecretKey(s.cfg.Gossip.SecretKey)
	if err != nil {
		return nil, err
	}
	cfg.SecretKey = secret
	return cfg, nil
}

func splitGossipAddress(label, raw string) (string, int, error) {
	addr := strings.TrimSpace(raw)
	if addr == "" {
		return "", 0, oopsx.B("gossip").Errorf("%s is required", label)
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, oopsx.B("gossip").Wrapf(err, "parse %s", label)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 0 || port > 65535 {
		return "", 0, oopsx.B("gossip").Errorf("%s port must be between 0 and 65535", label)
	}
	if strings.TrimSpace(host) == "" {
		host = "0.0.0.0"
	}
	return host, port, nil
}

func gossipSecretKey(raw string) ([]byte, error) {
	key := []byte(strings.TrimSpace(raw))
	if len(key) == 0 {
		return nil, nil
	}
	switch len(key) {
	case 16, 24, 32:
		return key, nil
	default:
		return nil, oopsx.B("gossip").Errorf("gossip.secret_key must be 16, 24, or 32 bytes")
	}
}

func normalizeSeeds(seeds []string) *list.List[string] {
	out := list.NewList[string]()
	for _, seed := range seeds {
		if value := strings.TrimSpace(seed); value != "" {
			out.Add(value)
		}
	}
	out.Sort(strings.Compare)
	return out
}
