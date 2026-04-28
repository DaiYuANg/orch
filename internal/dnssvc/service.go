package dnssvc

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/dnsx/dnsserver"
	"github.com/miekg/dns"

	"github.com/daiyuang/orch/internal/config"
)

type Service struct {
	logger          *slog.Logger
	cfg             config.DNSConfig
	store           *dnsserver.BboltStore
	server          *dnsserver.Server
	workloadRecords *mapping.ConcurrentMap[string, dnsserver.Record]
	started         atomic.Bool
}

func New(cfg config.Config, logger *slog.Logger) *Service {
	return &Service{
		logger:          logger,
		cfg:             cfg.DNS,
		workloadRecords: mapping.NewConcurrentMap[string, dnsserver.Record](),
	}
}

func (s *Service) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		s.logger.Info("dns service disabled by config")
		return nil
	}
	if s.started.Load() {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.cfg.DataPath), 0o755); err != nil {
		return err
	}

	store, err := dnsserver.OpenBboltStore(s.cfg.DataPath, s.logger)
	if err != nil {
		return err
	}

	server := dnsserver.NewServerWithRepository(
		dnsserver.Config{Listen: s.cfg.Listen},
		store,
		dnsserver.WithLogger(s.logger),
	)
	if err := server.Start(ctx); err != nil {
		_ = store.Close()
		return err
	}

	s.store = store
	s.server = server
	s.started.Store(true)
	zone := dnsZoneName(s.cfg)
	_ = s.store.SaveRecord(ctx, dnsserver.Record{
		Zone: zone,
		Name: zone,
		TTL:  60,
		Type: dns.TypeA,
		Data: "127.0.0.1",
	})
	s.logger.Info("dns service started", "listen", s.cfg.Listen, "udp", server.UDPAddr(), "tcp", server.TCPAddr())
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	if !s.started.Load() {
		return nil
	}

	if s.server != nil {
		if err := s.server.Stop(ctx); err != nil {
			return err
		}
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return err
		}
	}

	s.started.Store(false)
	s.logger.Info("dns service stopped")
	return nil
}
