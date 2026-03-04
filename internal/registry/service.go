package registry

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/adrg/xdg"
	"github.com/goccy/go-json"
	"github.com/samber/lo"
	"github.com/samber/mo"
	"go.etcd.io/bbolt"
)

const (
	endpointBucket = "service_endpoints"
	routeBucket    = "service_routes"
)

type Service struct {
	logger *slog.Logger

	raft *raft.Service

	db           *bbolt.DB
	endpointRepo *raft.Repository[ServiceEndpoint]
	routeRepo    *raft.Repository[Route]
	ownsDB       bool

	mu         sync.Mutex
	roundRobin map[string]uint64
}

func NewService(logger *slog.Logger) (*Service, error) {
	return newService(logger, nil)
}

func NewServiceWithRaft(logger *slog.Logger, raftService *raft.Service) (*Service, error) {
	return newService(logger, raftService)
}

func newService(logger *slog.Logger, raftService *raft.Service) (*Service, error) {
	dataDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir registry data dir: %w", err)
	}

	ownsDB := true
	db := (*bbolt.DB)(nil)
	if raftService != nil && raftService.Enabled() && raftService.MetadataDB() != nil {
		db = raftService.MetadataDB()
		ownsDB = false
	}
	if db == nil {
		dbPath := filepath.Join(dataDir, "registry.db")
		var err error
		db, err = bbolt.Open(dbPath, 0o700, nil)
		if err != nil {
			return nil, fmt.Errorf("open registry db: %w", err)
		}
	}

	return &Service{
		logger:       logger,
		raft:         raftService,
		db:           db,
		endpointRepo: raft.NewRepository[ServiceEndpoint](db, endpointBucket),
		routeRepo:    raft.NewRepository[Route](db, routeBucket),
		ownsDB:       ownsDB,
		roundRobin:   make(map[string]uint64),
	}, nil
}

func (s *Service) Close() error {
	if s.db == nil || !s.ownsDB {
		return nil
	}
	return s.db.Close()
}

func (s *Service) UpsertEndpoint(endpoint ServiceEndpoint) error {
	if strings.TrimSpace(endpoint.ID) == "" {
		return fmt.Errorf("endpoint id is required")
	}
	if strings.TrimSpace(endpoint.Service) == "" {
		return fmt.Errorf("endpoint service is required")
	}
	if strings.TrimSpace(endpoint.NodeIP) == "" {
		return fmt.Errorf("endpoint node ip is required")
	}
	if endpoint.Protocol == "" {
		endpoint.Protocol = RouteProtocolHTTP
	}
	now := time.Now()
	if endpoint.CreatedAt.IsZero() {
		endpoint.CreatedAt = now
	}
	endpoint.UpdatedAt = now
	if !s.shouldUseRaftWrite() {
		return s.endpointRepo.Set(endpoint.ID, endpoint)
	}
	return s.raftSet(endpointBucket, endpoint.ID, endpoint)
}

func (s *Service) DeleteEndpoint(endpointID string) error {
	var err error
	if s.shouldUseRaftWrite() {
		err = s.raftDelete(endpointBucket, endpointID)
	} else {
		err = s.endpointRepo.Delete(endpointID)
	}
	if isBucketNotFound(err) {
		return nil
	}
	return err
}

func (s *Service) SetEndpointHealth(endpointID string, healthy bool) error {
	if strings.TrimSpace(endpointID) == "" {
		return fmt.Errorf("endpoint id is required")
	}
	current, err := s.endpointRepo.Get(endpointID)
	if err != nil {
		return err
	}
	current.Healthy = healthy
	current.UpdatedAt = time.Now()
	if !s.shouldUseRaftWrite() {
		return s.endpointRepo.Set(endpointID, current)
	}
	return s.raftSet(endpointBucket, endpointID, current)
}

func (s *Service) ListEndpoints(service string, healthyOnly bool) ([]ServiceEndpoint, error) {
	items := make([]ServiceEndpoint, 0)
	err := s.endpointRepo.ForEach(func(_ string, endpoint ServiceEndpoint) error {
		if service != "" && endpoint.Service != service {
			return nil
		}
		if healthyOnly && !endpoint.Healthy {
			return nil
		}
		items = append(items, endpoint)
		return nil
	})
	if isBucketNotFound(err) {
		return []ServiceEndpoint{}, nil
	}
	return items, err
}

func (s *Service) ResolveServiceIPs(service string) ([]string, error) {
	endpoints, err := s.ListEndpoints(service, true)
	if err != nil {
		return nil, err
	}
	ips := lo.Map(endpoints, func(endpoint ServiceEndpoint, _ int) string {
		return strings.TrimSpace(endpoint.NodeIP)
	})
	ips = lo.Filter(ips, func(ip string, _ int) bool {
		return ip != ""
	})
	return lo.Uniq(ips), nil
}

func (s *Service) UpsertRoute(route Route) error {
	if strings.TrimSpace(route.ID) == "" {
		return fmt.Errorf("route id is required")
	}
	if strings.TrimSpace(route.Service) == "" {
		return fmt.Errorf("route service is required")
	}
	if route.Protocol == "" {
		return fmt.Errorf("route protocol is required")
	}
	now := time.Now()
	if route.CreatedAt.IsZero() {
		route.CreatedAt = now
	}
	route.UpdatedAt = now
	if !s.shouldUseRaftWrite() {
		return s.routeRepo.Set(route.ID, route)
	}
	return s.raftSet(routeBucket, route.ID, route)
}

func (s *Service) DeleteRoute(routeID string) error {
	var err error
	if s.shouldUseRaftWrite() {
		err = s.raftDelete(routeBucket, routeID)
	} else {
		err = s.routeRepo.Delete(routeID)
	}
	if isBucketNotFound(err) {
		return nil
	}
	return err
}

func (s *Service) DeleteRoutesByOwner(ownerID string) error {
	if strings.TrimSpace(ownerID) == "" {
		return nil
	}
	routes, err := s.ListRoutes("")
	if err != nil {
		return err
	}
	targets := lo.Filter(routes, func(route Route, _ int) bool {
		return route.OwnerID == ownerID
	})
	return lo.Reduce(targets, func(agg error, route Route, _ int) error {
		if agg != nil {
			return agg
		}
		return s.DeleteRoute(route.ID)
	}, error(nil))
}

func (s *Service) ListRoutes(protocol RouteProtocol) ([]Route, error) {
	items := make([]Route, 0)
	err := s.routeRepo.ForEach(func(_ string, route Route) error {
		if protocol != "" && route.Protocol != protocol {
			return nil
		}
		items = append(items, route)
		return nil
	})
	if isBucketNotFound(err) {
		return []Route{}, nil
	}
	return items, err
}

func (s *Service) ResolveHTTPBackend(host, path string) (Route, ServiceEndpoint, string, error) {
	routes, err := s.ListRoutes(RouteProtocolHTTP)
	if err != nil {
		return Route{}, ServiceEndpoint{}, "", err
	}
	host = normalizeHost(host)
	path = ensurePath(path)

	type routeMatch struct {
		route   Route
		matched bool
		longest int
	}

	match := lo.Reduce(routes, func(acc routeMatch, route Route, _ int) routeMatch {
		if !route.Enabled || !hostMatch(route.Host, host) {
			return acc
		}
		prefix := ensurePath(route.PathPrefix)
		if !strings.HasPrefix(path, prefix) || len(prefix) <= acc.longest {
			return acc
		}
		return routeMatch{
			route:   route,
			matched: true,
			longest: len(prefix),
		}
	}, routeMatch{longest: -1})

	if !match.matched {
		return Route{}, ServiceEndpoint{}, "", fmt.Errorf("no http route matched host=%s path=%s", host, path)
	}
	endpoint, backend, err := s.resolveBackendForRoute(match.route)
	if err != nil {
		return Route{}, ServiceEndpoint{}, "", err
	}
	return match.route, endpoint, backend, nil
}

func (s *Service) ResolveStreamBackend(protocol RouteProtocol, listenPort int) (Route, ServiceEndpoint, string, error) {
	routes, err := s.ListRoutes(protocol)
	if err != nil {
		return Route{}, ServiceEndpoint{}, "", err
	}
	route, found := lo.Find(routes, func(route Route) bool {
		return route.Enabled && route.ListenPort == listenPort
	})
	if !found {
		return Route{}, ServiceEndpoint{}, "", fmt.Errorf("no %s route for listen port %d", protocol, listenPort)
	}
	endpoint, backend, resolveErr := s.resolveBackendForRoute(route)
	if resolveErr != nil {
		return Route{}, ServiceEndpoint{}, "", resolveErr
	}
	return route, endpoint, backend, nil
}

func (s *Service) resolveBackendForRoute(route Route) (ServiceEndpoint, string, error) {
	endpoints, err := s.ListEndpoints(route.Service, true)
	if err != nil {
		return ServiceEndpoint{}, "", err
	}

	type backendCandidate struct {
		endpoint ServiceEndpoint
		port     int
	}
	candidates := lo.FilterMap(endpoints, func(endpoint ServiceEndpoint, _ int) (backendCandidate, bool) {
		port := selectPort(route, endpoint)
		if port <= 0 {
			return backendCandidate{}, false
		}
		return backendCandidate{endpoint: endpoint, port: port}, true
	})
	if len(candidates) == 0 {
		return ServiceEndpoint{}, "", fmt.Errorf("no healthy endpoint for service=%s", route.Service)
	}

	idx := s.nextIndex(route.ID, len(candidates))
	candidate := candidates[idx]
	backend := fmt.Sprintf("%s:%d", candidate.endpoint.NodeIP, candidate.port)
	return candidate.endpoint, backend, nil
}

func (s *Service) nextIndex(key string, size int) int {
	if size <= 1 {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.roundRobin[key]++
	return int(s.roundRobin[key] % uint64(size))
}

func selectPort(route Route, endpoint ServiceEndpoint) int {
	if route.TargetPort > 0 {
		return route.TargetPort
	}
	if route.PortName != "" {
		if port, ok := endpoint.Ports[route.PortName]; ok {
			return port
		}
	}
	if port, ok := endpoint.Ports["http"]; ok {
		return port
	}
	return mo.TupleToOption(lo.Find(lo.Values(endpoint.Ports), func(port int) bool {
		return port > 0
	})).OrElse(0)
}

func normalizeHost(host string) string {
	raw := strings.TrimSpace(strings.ToLower(host))
	if raw == "" {
		return raw
	}
	if idx := strings.Index(raw, ":"); idx >= 0 {
		return raw[:idx]
	}
	return raw
}

func hostMatch(pattern, host string) bool {
	pattern = normalizeHost(pattern)
	host = normalizeHost(host)
	if pattern == "" || pattern == "*" {
		return true
	}
	return pattern == host
}

func ensurePath(path string) string {
	p := strings.TrimSpace(path)
	if p == "" {
		return "/"
	}
	if strings.HasPrefix(p, "/") {
		return p
	}
	return "/" + p
}

func (s *Service) shouldUseRaftWrite() bool {
	return s.raft != nil && s.raft.Enabled()
}

func (s *Service) raftSet(bucket, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := s.raft.ApplySet(bucket, key, raw); err != nil {
		return fmt.Errorf("raft set %s/%s: %w", bucket, key, err)
	}
	return nil
}

func (s *Service) raftDelete(bucket, key string) error {
	if err := s.raft.ApplyDelete(bucket, key); err != nil {
		return fmt.Errorf("raft delete %s/%s: %w", bucket, key, err)
	}
	return nil
}

func isBucketNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "bucket") && strings.Contains(msg, "not found")
}
