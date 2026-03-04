package task

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/adrg/xdg"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRuntime struct {
	driver string

	mu            sync.Mutex
	containers    map[string]RuntimeRunSpec
	stopCalls     []string
	logByContaner map[string]string
}

func newFakeRuntime(driver string) *fakeRuntime {
	return &fakeRuntime{
		driver:        driver,
		containers:    make(map[string]RuntimeRunSpec),
		logByContaner: make(map[string]string),
	}
}

func (f *fakeRuntime) Driver() string {
	return f.driver
}

func (f *fakeRuntime) Ping(context.Context) error {
	return nil
}

func (f *fakeRuntime) Run(_ context.Context, spec RuntimeRunSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	containerID := fmt.Sprintf("ctr-%d", len(f.containers)+1)
	f.containers[containerID] = spec
	f.logByContaner[containerID] = "runtime log: " + spec.Name
	return containerID, nil
}

func (f *fakeRuntime) Stop(_ context.Context, containerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.containers, containerID)
	f.stopCalls = append(f.stopCalls, containerID)
	return nil
}

func (f *fakeRuntime) Status(_ context.Context, containerID string) (RuntimeStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.containers[containerID]; !ok {
		return RuntimeStatus{}, fmt.Errorf("container not found: %s", containerID)
	}
	return RuntimeStatus{
		ContainerID: containerID,
		Running:     true,
		State:       "running",
	}, nil
}

func (f *fakeRuntime) Logs(_ context.Context, containerID string, _ int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if logs, ok := f.logByContaner[containerID]; ok {
		return logs, nil
	}
	return "", fmt.Errorf("logs not found: %s", containerID)
}

func (f *fakeRuntime) List(context.Context, bool, map[string][]string) ([]RuntimeContainer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return lo.MapToSlice(f.containers, func(containerID string, spec RuntimeRunSpec) RuntimeContainer {
		return RuntimeContainer{
			ID:     containerID,
			Names:  []string{spec.Name},
			Image:  spec.Image,
			Labels: spec.Labels,
		}
	}), nil
}

func newRegistryForTaskTest(t *testing.T) *registry.Service {
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

func TestDeployAndStopLifecycle(t *testing.T) {
	ctx := context.Background()
	registryService := newRegistryForTaskTest(t)
	runtime := newFakeRuntime("mock")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewServiceWithRuntimeFactory(logger, registryService, func() (RuntimeExecutor, error) {
		return runtime, nil
	})

	workloadYAML := `
name: todo
units:
  - name: backend
    tasks:
      - name: api
        type: service
        driver: docker
        image: nginx:latest
        replicas: 1
        dns:
          resolver: "1.1.1.1, 8.8.8.8"
          domains:
            - svc.cluster.local
            - internal.local
        network:
          name: default
          port:
            http: 18080
`
	result, err := service.Deploy(ctx, DeployRequest{
		Filename: "todo.yaml",
		Content:  workloadYAML,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "todo", result.WorkloadName)
	assert.Equal(t, 1, result.Instances)

	detail, ok := service.GetDeployment(result.DeploymentID)
	require.True(t, ok)
	require.Len(t, detail.Instances, 1)
	instance := detail.Instances[0]
	assert.Equal(t, "mock", instance.Driver)
	assert.NotEmpty(t, instance.ContainerID)

	logs, err := service.Logs(ctx, instance.ID, 20)
	require.NoError(t, err)
	assert.Contains(t, logs, "runtime log")

	routes, err := registryService.ListRoutes(registry.RouteProtocol(""))
	require.NoError(t, err)
	assert.NotEmpty(t, routes)
	expectedService := buildServiceName("todo", "backend", "api")
	assert.Equal(t, expectedService+".warden.local", routes[0].Host)

	runtime.mu.Lock()
	specs := lo.Values(runtime.containers)
	runtime.mu.Unlock()
	require.Len(t, specs, 1)
	assert.Equal(t, []string{"1.1.1.1", "8.8.8.8"}, specs[0].DNSServers)
	assert.Equal(t, []string{"svc.cluster.local", "internal.local"}, specs[0].DNSSearch)

	err = service.StopDeployment(ctx, result.DeploymentID)
	require.NoError(t, err)

	updated, ok := service.GetDeployment(result.DeploymentID)
	require.True(t, ok)
	assert.Equal(t, DeploymentStatusStopped, updated.Deployment.Status)

	endpoints, err := registryService.ListEndpoints("", false)
	require.NoError(t, err)
	assert.Empty(t, endpoints)

	routes, err = registryService.ListRoutes(registry.RouteProtocol(""))
	require.NoError(t, err)
	assert.Empty(t, routes)
	assert.Contains(t, runtime.stopCalls, instance.ContainerID)
}
