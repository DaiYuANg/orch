package ingress

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/adrg/xdg"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistryForIngressTest(t *testing.T) *registry.Service {
	t.Helper()
	oldDataHome := xdg.DataHome
	xdg.DataHome = t.TempDir()
	t.Cleanup(func() {
		xdg.DataHome = oldDataHome
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := registry.NewService(logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service
}

func newTestIngress(registryService *registry.Service) *Ingress {
	return &Ingress{
		httpAddr:     ":0",
		logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		registry:     registryService,
		tcpListeners: make(map[int]*tcpListener),
		udpListeners: make(map[int]*udpListener),
		stopCh:       make(chan struct{}),
	}
}

func pickFreeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestNormalizeHostWithPort(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{input: "API.WARDEN.Local:8080", expected: "api.warden.local"},
		{input: "[::1]:9000", expected: "::1"},
		{input: " 10.0.0.2 ", expected: "10.0.0.2"},
	}

	lo.ForEach(cases, func(tc struct {
		input    string
		expected string
	}, _ int) {
		assert.Equal(t, tc.expected, normalizeHostWithPort(tc.input))
	})
}

func TestSyncStreamRoutesRegistersAndUnregistersTCP(t *testing.T) {
	registryService := newRegistryForIngressTest(t)
	ing := newTestIngress(registryService)
	t.Cleanup(func() {
		_ = ing.Stop()
	})

	backendListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = backendListener.Close()
	})

	backendPort := backendListener.Addr().(*net.TCPAddr).Port
	listenPort := pickFreeTCPPort(t)

	err = registryService.UpsertEndpoint(registry.ServiceEndpoint{
		ID:       "ep-1",
		Service:  "task-api",
		NodeID:   "node-1",
		NodeIP:   "127.0.0.1",
		Protocol: registry.RouteProtocolTCP,
		Ports: map[string]int{
			"tcp": backendPort,
		},
		Healthy: true,
	})
	require.NoError(t, err)

	route := registry.Route{
		ID:         "route-stream",
		OwnerID:    "dep-stream",
		Service:    "task-api",
		Protocol:   registry.RouteProtocolTCP,
		ListenPort: listenPort,
		TargetPort: backendPort,
		Enabled:    true,
	}
	err = registryService.UpsertRoute(route)
	require.NoError(t, err)

	ing.syncStreamRoutes(registry.RouteProtocolTCP)
	require.Contains(t, ing.tcpListeners, listenPort)
	assert.Equal(t, fmt.Sprintf("127.0.0.1:%d", backendPort), ing.tcpListeners[listenPort].backend)

	route.Enabled = false
	err = registryService.UpsertRoute(route)
	require.NoError(t, err)

	ing.syncStreamRoutes(registry.RouteProtocolTCP)
	assert.NotContains(t, ing.tcpListeners, listenPort)
}
