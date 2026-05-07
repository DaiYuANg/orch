package ingress

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/arcgolabs/collectionx/list"
	velaruntime "github.com/arcgolabs/vela/runtime"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func testHTTPHandler(t *testing.T, routes *list.List[config.IngressRoute]) http.Handler {
	t.Helper()
	snapshot, routeCount, err := buildVelaSnapshot(routes)
	if err != nil {
		t.Fatal(err)
	}
	gateway := velaruntime.NewGateway(snapshot, slog.Default(), false, velaruntime.NewNoopMetrics())
	return newIngressHTTPHandler(gateway, func() int { return routeCount })
}

func TestNewIngressHTTPHandler_proxyPathRewrite(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	t.Cleanup(upstream.Close)

	handler := testHTTPHandler(t, list.NewList(
		config.IngressRoute{PathPrefix: "/api", Upstream: upstream.URL},
	))

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/api/v1/hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if string(body) != "/v1/hello" {
		t.Fatalf("upstream saw path: got %q want %q", body, "/v1/hello")
	}
}

func TestNewIngressHTTPHandler_noRouteNotFound(t *testing.T) {
	t.Parallel()

	handler := testHTTPHandler(t, list.NewList(
		config.IngressRoute{PathPrefix: "/api", Upstream: "http://127.0.0.1:1"},
	))
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/other", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestNewIngressHTTPHandler_pathPrefixBoundary(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("upstream should not receive %s", r.URL.Path)
	}))
	t.Cleanup(upstream.Close)

	handler := testHTTPHandler(t, list.NewList(
		config.IngressRoute{PathPrefix: "/api", Upstream: upstream.URL},
	))
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/api2", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestNewIngressHTTPHandler_placeholderNoRoutes(t *testing.T) {
	t.Parallel()
	handler := testHTTPHandler(t, nil)
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	req.Host = "127.0.0.1"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
}

func TestNewIngressHTTPHandler_roundRobinDistributes(t *testing.T) {
	t.Parallel()

	var hitsA, hitsB int
	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsA++
		_, _ = w.Write([]byte("a"))
	}))
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitsB++
		_, _ = w.Write([]byte("b"))
	}))
	t.Cleanup(srvA.Close)
	t.Cleanup(srvB.Close)

	handler := testHTTPHandler(t, list.NewList(
		config.IngressRoute{
			PathPrefix: "/p",
			Upstreams:  []string{srvA.URL, srvB.URL},
			LB:         "round_robin",
		},
	))

	for range 8 {
		req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1/p/x", nil)
		req.Host = "127.0.0.1"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		resp := rec.Result()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status: %d", resp.StatusCode)
		}
		resp.Body.Close()
	}

	if hitsA < 1 || hitsB < 1 {
		t.Fatalf("expected both upstreams to receive traffic, got hitsA=%d hitsB=%d", hitsA, hitsB)
	}
	if hitsA+hitsB != 8 {
		t.Fatalf("hitsA+hitsB=%d want 8", hitsA+hitsB)
	}
}

func TestIngressRouteUpstreamEndpoints(t *testing.T) {
	t.Parallel()
	r := config.IngressRoute{Upstream: "http://a"}
	if got := r.UpstreamEndpoints(); got.Len() != 1 {
		t.Fatalf("got %#v", got)
	} else if first, _ := got.Get(0); first != "http://a" {
		t.Fatalf("got %#v", got.Values())
	}
	r2 := config.IngressRoute{Upstreams: []string{"http://a", "http://b"}, Upstream: "http://ignored"}
	got := r2.UpstreamEndpoints()
	if got.Len() != 2 {
		t.Fatalf("got %#v", got.Values())
	}
}

func TestIngressRouteLBPolicy(t *testing.T) {
	t.Parallel()
	if got := (&config.IngressRoute{}).LBPolicy(); got != "round_robin" {
		t.Fatal(got)
	}
	if got := (&config.IngressRoute{LB: "ROUND_ROBIN"}).LBPolicy(); got != "round_robin" {
		t.Fatal(got)
	}
}

// mapDNS mimics dnssvc workloadRecordKey lookup (lowercase namespace/workload).
type mapDNS map[string]string

func (m mapDNS) LookupWorkloadIPv4(namespace, workloadName string) (string, bool) {
	if m == nil {
		return "", false
	}
	ns := strings.TrimSpace(namespace)
	if ns == "" {
		ns = "default"
	}
	key := strings.ToLower(ns) + "/" + strings.ToLower(strings.TrimSpace(workloadName))
	ip, ok := m[key]
	return ip, ok
}

func TestCompileIngressRoutesFromDeploy(t *testing.T) {
	t.Parallel()
	apps := list.NewList(deployv1.App{
		Metadata: deployv1.Metadata{Name: "a", Namespace: "ns"},
		Workloads: []deployv1.Workload{{
			Name: "web",
			Endpoints: []deployv1.Endpoint{{
				Name: "http", Port: 8080, Protocol: deployv1.ProtoHTTP,
			}},
		}},
		Ingresses: []deployv1.Ingress{{
			Routes: []deployv1.IngressRoute{{
				Path:    "/api",
				Backend: deployv1.EndpointRef{Workload: "web", Endpoint: "http"},
			}},
		}},
	})
	dns := mapDNS{"ns/web": "10.0.0.2"}
	got := CompileIngressRoutesFromDeploy(apps, dns, nil)
	route, ok := got.Get(0)
	if got.Len() != 1 || !ok || route.PathPrefix != "/api" || route.Upstream != "http://10.0.0.2:8080" {
		t.Fatalf("got %#v", got.Values())
	}
}
