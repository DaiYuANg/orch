package task

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	internalconfig "github.com/DaiYuANg/warden/internal/config"
	internaldns "github.com/DaiYuANg/warden/internal/dns"
	internalingress "github.com/DaiYuANg/warden/internal/ingress"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

type dnsResponseRecorder struct {
	msg *dns.Msg
}

func (r *dnsResponseRecorder) LocalAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func (r *dnsResponseRecorder) RemoteAddr() net.Addr {
	return &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0}
}

func (r *dnsResponseRecorder) WriteMsg(msg *dns.Msg) error {
	r.msg = msg
	return nil
}

func (r *dnsResponseRecorder) Write(_ []byte) (int, error) {
	return 0, nil
}

func (r *dnsResponseRecorder) Close() error {
	return nil
}

func (r *dnsResponseRecorder) TsigStatus() error {
	return nil
}

func (r *dnsResponseRecorder) TsigTimersOnly(bool) {}

func (r *dnsResponseRecorder) Hijack() {}

func pickFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

func TestDeployDNSIngressFlow(t *testing.T) {
	ctx := context.Background()
	registryService := newRegistryForTaskTest(t)
	runtime := newFakeRuntime("mock")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	taskService := NewServiceWithRuntimeFactory(logger, registryService, func() (RuntimeExecutor, error) {
		return runtime, nil
	})
	taskService.nodeIP = "127.0.0.1"

	dnsServer, err := internaldns.NewDNSServer(logger, registryService)
	require.NoError(t, err)
	defer func() {
		_ = dnsServer.Shutdown()
	}()

	backendPort := pickFreePort(t)
	ingressPort := pickFreePort(t)
	backendAddr := fmt.Sprintf("127.0.0.1:%d", backendPort)
	ingressAddr := fmt.Sprintf("127.0.0.1:%d", ingressPort)

	backendServer := &http.Server{
		Addr: backendAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("backend-ok"))
		}),
	}
	go func() {
		_ = backendServer.ListenAndServe()
	}()
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = backendServer.Shutdown(stopCtx)
	}()

	app := fx.New(
		fx.Supply(
			&internalconfig.Config{
				Network: internalconfig.Network{
					IngressHTTPListen: ingressAddr,
				},
			},
			logger,
			registryService,
		),
		internalingress.Module,
	)
	startCtx, startCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer startCancel()
	require.NoError(t, app.Start(startCtx))
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Stop(stopCtx)
	}()

	workloadYAML := fmt.Sprintf(`
name: todo
units:
  - name: backend
    tasks:
      - name: api
        type: service
        driver: docker
        image: nginx:latest
        replicas: 1
        network:
          name: default
          port:
            http: %d
`, backendPort)

	result, err := taskService.Deploy(ctx, DeployRequest{
		Filename: "todo.yaml",
		Content:  workloadYAML,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.DeploymentID)

	serviceName := buildServiceName("todo", "backend", "api")
	host := serviceName + ".warden.local"

	dnsQuery := new(dns.Msg)
	dnsQuery.SetQuestion(dns.Fqdn(host), dns.TypeA)
	recorder := &dnsResponseRecorder{}
	dnsServer.ServeDNS(recorder, dnsQuery)
	require.NotNil(t, recorder.msg)
	require.NotEmpty(t, recorder.msg.Answer)
	answer, ok := recorder.msg.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "127.0.0.1", answer.A.String())

	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	var body string
	for attempt := 0; attempt < 30; attempt++ {
		req, reqErr := http.NewRequest(http.MethodGet, "http://"+ingressAddr+"/", nil)
		require.NoError(t, reqErr)
		req.Host = host
		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = doErr
			time.Sleep(100 * time.Millisecond)
			continue
		}
		bytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		body = string(bytes)
		if readErr == nil && resp.StatusCode == http.StatusOK && strings.Contains(body, "backend-ok") {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("unexpected ingress response status=%d body=%s", resp.StatusCode, body)
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, lastErr)
	assert.Contains(t, body, "backend-ok")
}
