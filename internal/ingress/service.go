package ingress

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type Service struct {
	logger        *slog.Logger
	cfg           config.IngressConfig
	raft          *raftsvc.Service
	dns           *dnssvc.Service
	dataRoot      string
	mu            sync.Mutex
	apps          []*fiber.App
	live          *atomic.Pointer[liveRoutes]
	refreshCancel context.CancelFunc
	refreshWG     sync.WaitGroup
	started       bool // guarded by mu
}

func New(cfg config.Config, logger *slog.Logger, raft *raftsvc.Service, dns *dnssvc.Service) *Service {
	return &Service{
		logger:   logger,
		cfg:      cfg.Ingress,
		raft:     raft,
		dns:      dns,
		dataRoot: config.DefaultDataRoot(),
	}
}

func (s *Service) refreshRoutes() {
	if !s.cfg.Enabled || s.raft == nil || s.dns == nil || s.live == nil {
		return
	}
	routes := CompileIngressRoutesFromDeploy(s.raft.ListDesiredDeployApps(), s.dns, s.logger)
	compiled, err := buildFiberRoutes(routes)
	if err != nil {
		s.logger.Warn("ingress routes compile failed", "error", err)
		return
	}
	s.live.Store(&liveRoutes{compiled: compiled})
	log := s.logger.With(slog.String("component", "ingress"))
	log.Info("ingress routes refreshed", "routes", len(compiled))
}

func (s *Service) Start(_ context.Context) error {
	if !s.cfg.Enabled {
		s.logger.Info("ingress disabled by config")
		return nil
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}

	var tlsConf *tls.Config
	if s.cfg.TLS.Enabled {
		m, err := newAutocertManager(s.cfg.TLS, s.cfg.TLSAutocertDomains(), s.dataRoot)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		tlsConf = serverTLSConfig(m)
		if strings.TrimSpace(s.cfg.TLS.Email) == "" {
			s.logger.Warn("ingress autocert: ingress.tls.email is empty (Let's Encrypt recommends a contact email)")
		}
		s.logger.Info("ingress autocert",
			"domains", s.cfg.TLSAutocertDomains(),
			"tls_listen", s.cfg.TLSListenAddrs(),
			"staging", s.cfg.TLS.Staging,
		)
	}

	plainAddrs := s.cfg.PlainListenAddrs()
	tlsAddrs := s.cfg.TLSListenAddrs()
	listeners := make([]net.Listener, 0, len(plainAddrs)+len(tlsAddrs))

	for _, addr := range plainAddrs {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			s.mu.Unlock()
			return oopsx.B("ingress").Wrapf(err, "listen %s", addr)
		}
		listeners = append(listeners, ln)
	}
	for _, addr := range tlsAddrs {
		if tlsConf == nil {
			continue
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			s.mu.Unlock()
			return oopsx.B("ingress").Wrapf(err, "listen tls %s", addr)
		}
		listeners = append(listeners, tls.NewListener(ln, tlsConf))
	}

	if len(listeners) == 0 {
		s.mu.Unlock()
		return oopsx.B("ingress").Errorf("no ingress listeners (configure ingress.listen and/or ingress.tls)")
	}

	log := s.logger.With(slog.String("component", "ingress"), slog.String("engine", "fiber"))

	live := &atomic.Pointer[liveRoutes]{}
	s.live = live

	apps := make([]*fiber.App, 0, len(listeners))
	for range listeners {
		app, err := newIngressFiberApp(log, live)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			s.mu.Unlock()
			return err
		}
		apps = append(apps, app)
	}

	for i, ln := range listeners {
		ln, app := ln, apps[i]
		go func() {
			if listenErr := app.Listener(ln); listenErr != nil {
				log.Error("ingress listener stopped", "error", listenErr)
			}
		}()
	}

	s.apps = apps
	s.started = true

	refreshCtx, refreshCancel := context.WithCancel(context.Background())
	s.refreshCancel = refreshCancel
	ch := s.raft.DeployReconcileSignals()
	s.refreshWG.Add(1)
	go func() {
		defer s.refreshWG.Done()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-refreshCtx.Done():
				return
			case <-ch:
				s.refreshRoutes()
			case <-ticker.C:
				s.refreshRoutes()
			}
		}
	}()

	s.mu.Unlock()

	s.refreshRoutes()

	s.logger.Info("ingress started",
		"plain_listen", plainAddrs,
		"tls_listen", tlsAddrs,
	)
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	apps := s.apps
	s.apps = nil
	s.started = false
	cancel := s.refreshCancel
	s.refreshCancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
		s.refreshWG.Wait()
	}

	s.mu.Lock()
	s.live = nil
	s.mu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, app := range apps {
		wg.Add(1)
		go func(a *fiber.App) {
			defer wg.Done()
			if err := a.ShutdownWithContext(shutdownCtx); err != nil {
				s.logger.Warn("ingress shutdown", "error", err)
			}
		}(app)
	}
	wg.Wait()

	s.logger.Info("ingress stopped")
	return nil
}
