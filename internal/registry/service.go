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
	"go.etcd.io/bbolt"
)

type Service struct {
	logger *slog.Logger

	db           *bbolt.DB
	endpointRepo *raft.Repository[ServiceEndpoint]
	routeRepo    *raft.Repository[Route]

	mu         sync.Mutex
	roundRobin map[string]uint64
}

func NewService(logger *slog.Logger) (*Service, error) {
	dataDir := filepath.Join(xdg.DataHome, "warden")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir registry data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "registry.db")
	db, err := bbolt.Open(dbPath, 0o700, nil)
	if err != nil {
		return nil, fmt.Errorf("open registry db: %w", err)
	}

	return &Service{
		logger:       logger,
		db:           db,
		endpointRepo: raft.NewRepository[ServiceEndpoint](db, "service_endpoints"),
		routeRepo:    raft.NewRepository[Route](db, "service_routes"),
		roundRobin:   make(map[string]uint64),
	}, nil
}

func (s *Service) Close() error {
	if s.db == nil {
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
	return s.endpointRepo.Set(endpoint.ID, endpoint)
}

func (s *Service) DeleteEndpoint(endpointID string) error {
	err := s.endpointRepo.Delete(endpointID)
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
	return s.endpointRepo.Set(endpointID, current)
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
	uniq := map[string]struct{}{}
	ips := make([]string, 0, len(endpoints))
	for _, endpoint := range endpoints {
		ip := strings.TrimSpace(endpoint.NodeIP)
		if ip == "" {
			continue
		}
		if _, exists := uniq[ip]; exists {
			continue
		}
		uniq[ip] = struct{}{}
		ips = append(ips, ip)
	}
	return ips, nil
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
	return s.routeRepo.Set(route.ID, route)
}

func (s *Service) DeleteRoute(routeID string) error {
	err := s.routeRepo.Delete(routeID)
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
	for _, route := range routes {
		if route.OwnerID != ownerID {
			continue
		}
		if deleteErr := s.DeleteRoute(route.ID); deleteErr != nil {
			return deleteErr
		}
	}
	return nil
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

	var selected Route
	matched := false
	longestPath := -1
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		if !hostMatch(route.Host, host) {
			continue
		}
		prefix := ensurePath(route.PathPrefix)
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		if len(prefix) > longestPath {
			longestPath = len(prefix)
			selected = route
			matched = true
		}
	}
	if !matched {
		return Route{}, ServiceEndpoint{}, "", fmt.Errorf("no http route matched host=%s path=%s", host, path)
	}
	endpoint, backend, err := s.resolveBackendForRoute(selected)
	if err != nil {
		return Route{}, ServiceEndpoint{}, "", err
	}
	return selected, endpoint, backend, nil
}

func (s *Service) ResolveStreamBackend(protocol RouteProtocol, listenPort int) (Route, ServiceEndpoint, string, error) {
	routes, err := s.ListRoutes(protocol)
	if err != nil {
		return Route{}, ServiceEndpoint{}, "", err
	}
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		if route.ListenPort != listenPort {
			continue
		}
		endpoint, backend, resolveErr := s.resolveBackendForRoute(route)
		if resolveErr != nil {
			return Route{}, ServiceEndpoint{}, "", resolveErr
		}
		return route, endpoint, backend, nil
	}
	return Route{}, ServiceEndpoint{}, "", fmt.Errorf("no %s route for listen port %d", protocol, listenPort)
}

func (s *Service) resolveBackendForRoute(route Route) (ServiceEndpoint, string, error) {
	endpoints, err := s.ListEndpoints(route.Service, true)
	if err != nil {
		return ServiceEndpoint{}, "", err
	}

	candidates := make([]ServiceEndpoint, 0, len(endpoints))
	ports := make([]int, 0, len(endpoints))
	for _, endpoint := range endpoints {
		port := selectPort(route, endpoint)
		if port <= 0 {
			continue
		}
		candidates = append(candidates, endpoint)
		ports = append(ports, port)
	}
	if len(candidates) == 0 {
		return ServiceEndpoint{}, "", fmt.Errorf("no healthy endpoint for service=%s", route.Service)
	}

	idx := s.nextIndex(route.ID, len(candidates))
	endpoint := candidates[idx]
	backend := fmt.Sprintf("%s:%d", endpoint.NodeIP, ports[idx])
	return endpoint, backend, nil
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
	for _, port := range endpoint.Ports {
		if port > 0 {
			return port
		}
	}
	return 0
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

func isBucketNotFound(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "bucket") && strings.Contains(msg, "not found")
}
