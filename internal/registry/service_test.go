package registry

import (
	"io"
	"log/slog"
	"testing"

	"github.com/adrg/xdg"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRegistryServiceForTest(t *testing.T) *Service {
	t.Helper()
	oldDataHome := xdg.DataHome
	xdg.DataHome = t.TempDir()
	t.Cleanup(func() {
		xdg.DataHome = oldDataHome
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := NewService(logger)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = service.Close()
	})
	return service
}

func TestResolveHTTPBackendPrefersLongestPathPrefix(t *testing.T) {
	service := newRegistryServiceForTest(t)

	err := service.UpsertEndpoint(ServiceEndpoint{
		ID:       "ep-1",
		Service:  "api",
		NodeID:   "node-1",
		NodeIP:   "127.0.0.1",
		Protocol: RouteProtocolHTTP,
		Ports: map[string]int{
			"http": 18080,
		},
		Healthy: true,
	})
	require.NoError(t, err)

	err = service.UpsertRoute(Route{
		ID:         "route-root",
		OwnerID:    "dep-1",
		Service:    "api",
		Protocol:   RouteProtocolHTTP,
		Host:       "api.warden.local",
		PathPrefix: "/",
		Enabled:    true,
	})
	require.NoError(t, err)

	err = service.UpsertRoute(Route{
		ID:         "route-v1",
		OwnerID:    "dep-1",
		Service:    "api",
		Protocol:   RouteProtocolHTTP,
		Host:       "api.warden.local",
		PathPrefix: "/v1",
		Enabled:    true,
	})
	require.NoError(t, err)

	route, endpoint, backend, err := service.ResolveHTTPBackend("api.warden.local:80", "/v1/jobs")
	require.NoError(t, err)
	assert.Equal(t, "route-v1", route.ID)
	assert.Equal(t, "ep-1", endpoint.ID)
	assert.Equal(t, "127.0.0.1:18080", backend)
}

func TestDeleteRoutesByOwner(t *testing.T) {
	service := newRegistryServiceForTest(t)

	routes := []Route{
		{ID: "r-1", OwnerID: "dep-1", Service: "svc", Protocol: RouteProtocolTCP, ListenPort: 19001, Enabled: true},
		{ID: "r-2", OwnerID: "dep-1", Service: "svc", Protocol: RouteProtocolUDP, ListenPort: 19002, Enabled: true},
		{ID: "r-3", OwnerID: "dep-2", Service: "svc", Protocol: RouteProtocolHTTP, Host: "svc.warden.local", PathPrefix: "/", Enabled: true},
	}
	upsertErr := lo.Reduce(routes, func(agg error, route Route, _ int) error {
		if agg != nil {
			return agg
		}
		return service.UpsertRoute(route)
	}, error(nil))
	require.NoError(t, upsertErr)

	err := service.DeleteRoutesByOwner("dep-1")
	require.NoError(t, err)

	remaining, err := service.ListRoutes(RouteProtocol(""))
	require.NoError(t, err)
	require.Len(t, remaining, 1)
	assert.Equal(t, "r-3", remaining[0].ID)
}
