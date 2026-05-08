package ingress

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	valeruntime "github.com/arcgolabs/vale/runtime"

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
	servers       []*http.Server
	gateway       *valeruntime.Gateway
	routeCount    atomic.Int64
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
	if !s.cfg.Enabled || s.raft == nil || s.dns == nil {
		return
	}
	apps := s.raft.ListDesiredDeployApps()
	routes := CompileIngressRoutesFromDeploy(apps, s.dns, s.logger)
	snapshot, routeCount, err := buildValeSnapshot(routes)
	if err != nil {
		s.logger.Warn("ingress routes compile failed", "error", err)
		return
	}
	s.mu.Lock()
	gateway := s.gateway
	if gateway != nil {
		gateway.Swap(snapshot)
		s.routeCount.Store(int64(routeCount))
	}
	s.mu.Unlock()
	log := s.logger.With(slog.String("component", "ingress"))
	log.Info("ingress routes refreshed", "routes", routeCount)
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
	domains := s.cfg.TLSAutocertDomainList()
	if s.cfg.TLS.Enabled {
		m, err := newAutocertManager(s.cfg.TLS, domains, s.dataRoot)
		if err != nil {
			s.mu.Unlock()
			return err
		}
		tlsConf = serverTLSConfig(m)
		if strings.TrimSpace(s.cfg.TLS.Email) == "" {
			s.logger.Warn("ingress autocert: ingress.tls.email is empty (Let's Encrypt recommends a contact email)")
		}
		s.logger.Info("ingress autocert",
			"domains", domains.Values(),
			"tls_listen", s.cfg.TLSListenAddrList().Values(),
			"staging", s.cfg.TLS.Staging,
		)
	}

	plainAddrs := s.cfg.PlainListenAddrList()
	tlsAddrs := s.cfg.TLSListenAddrList()
	listeners := make([]net.Listener, 0, plainAddrs.Len()+tlsAddrs.Len())

	var listenErr error
	plainAddrs.Range(func(_ int, addr string) bool {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			listenErr = oopsx.B("ingress").Wrapf(err, "listen %s", addr)
			return false
		}
		listeners = append(listeners, ln)
		return true
	})
	if listenErr != nil {
		s.mu.Unlock()
		return listenErr
	}
	tlsAddrs.Range(func(_ int, addr string) bool {
		if tlsConf == nil {
			return true
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			listenErr = oopsx.B("ingress").Wrapf(err, "listen tls %s", addr)
			return false
		}
		listeners = append(listeners, tls.NewListener(ln, tlsConf))
		return true
	})
	if listenErr != nil {
		s.mu.Unlock()
		return listenErr
	}

	if len(listeners) == 0 {
		s.mu.Unlock()
		return oopsx.B("ingress").Errorf("no ingress listeners (configure ingress.listen and/or ingress.tls)")
	}

	log := s.logger.With(slog.String("component", "ingress"), slog.String("engine", "vale"))

	snapshot, _, err := buildValeSnapshot(nil)
	if err != nil {
		for _, l := range listeners {
			_ = l.Close()
		}
		s.mu.Unlock()
		return err
	}
	gateway := valeruntime.NewGateway(snapshot, log, true, valeruntime.NewNoopMetrics())
	s.gateway = gateway

	servers := make([]*http.Server, 0, len(listeners))
	for range listeners {
		servers = append(servers, newIngressHTTPServer(log, gateway, s.currentRouteCount))
	}

	for i, ln := range listeners {
		ln, server := ln, servers[i]
		go func() {
			if listenErr := server.Serve(ln); listenErr != nil && listenErr != http.ErrServerClosed {
				log.Error("ingress listener stopped", "error", listenErr)
			}
		}()
	}

	s.servers = servers
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
		"plain_listen", plainAddrs.Values(),
		"tls_listen", tlsAddrs.Values(),
	)
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	servers := s.servers
	s.servers = nil
	s.gateway = nil
	s.routeCount.Store(0)
	s.started = false
	cancel := s.refreshCancel
	s.refreshCancel = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
		s.refreshWG.Wait()
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, server := range servers {
		wg.Add(1)
		go func(srv *http.Server) {
			defer wg.Done()
			if err := srv.Shutdown(shutdownCtx); err != nil {
				s.logger.Warn("ingress shutdown", "error", err)
			}
		}(server)
	}
	wg.Wait()

	s.logger.Info("ingress stopped")
	return nil
}

func (s *Service) currentRouteCount() int {
	return int(s.routeCount.Load())
}
