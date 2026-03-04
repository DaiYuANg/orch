package task

import (
	"context"

	dockerrt "github.com/DaiYuANg/warden/internal/runtime_engine/docker"
	godocker "github.com/fsouza/go-dockerclient"
	"github.com/samber/lo"
)

type RuntimeRunSpec struct {
	Name       string
	Image      string
	Cmd        []string
	Env        map[string]string
	Labels     map[string]string
	Ports      map[string]int
	DNSServers []string
	DNSSearch  []string
}

type RuntimeStatus struct {
	ContainerID string
	Name        string
	Running     bool
	State       string
	ExitCode    int
	Error       string
}

type RuntimeContainer struct {
	ID     string
	Names  []string
	Image  string
	Labels map[string]string
}

type RuntimeExecutor interface {
	Driver() string
	Ping(ctx context.Context) error
	Run(ctx context.Context, spec RuntimeRunSpec) (string, error)
	Stop(ctx context.Context, containerID string) error
	Status(ctx context.Context, containerID string) (RuntimeStatus, error)
	Logs(ctx context.Context, containerID string, tail int) (string, error)
	List(ctx context.Context, all bool, filters map[string][]string) ([]RuntimeContainer, error)
}

type RuntimeFactory func() (RuntimeExecutor, error)

type dockerRuntimeExecutor struct {
	exec *dockerrt.Executor
}

func newDockerRuntimeExecutor() (RuntimeExecutor, error) {
	exec, err := dockerrt.NewExecutor()
	if err != nil {
		return nil, err
	}
	return &dockerRuntimeExecutor{exec: exec}, nil
}

func (d *dockerRuntimeExecutor) Driver() string {
	return "docker"
}

func (d *dockerRuntimeExecutor) Ping(ctx context.Context) error {
	return d.exec.Ping(ctx)
}

func (d *dockerRuntimeExecutor) Run(ctx context.Context, spec RuntimeRunSpec) (string, error) {
	return d.exec.RunContainer(ctx, dockerrt.RunSpec{
		Name:       spec.Name,
		Image:      spec.Image,
		Cmd:        spec.Cmd,
		Env:        spec.Env,
		Labels:     spec.Labels,
		Ports:      spec.Ports,
		DNSServers: spec.DNSServers,
		DNSSearch:  spec.DNSSearch,
	})
}

func (d *dockerRuntimeExecutor) Stop(ctx context.Context, containerID string) error {
	return d.exec.StopAndRemove(ctx, containerID)
}

func (d *dockerRuntimeExecutor) Status(ctx context.Context, containerID string) (RuntimeStatus, error) {
	status, err := d.exec.Status(ctx, containerID)
	if err != nil {
		return RuntimeStatus{}, err
	}
	return RuntimeStatus{
		ContainerID: status.ContainerID,
		Name:        status.Name,
		Running:     status.Running,
		State:       status.State,
		ExitCode:    status.ExitCode,
		Error:       status.Error,
	}, nil
}

func (d *dockerRuntimeExecutor) Logs(ctx context.Context, containerID string, tail int) (string, error) {
	return d.exec.Logs(ctx, containerID, tail)
}

func (d *dockerRuntimeExecutor) List(ctx context.Context, all bool, filters map[string][]string) ([]RuntimeContainer, error) {
	items, err := d.exec.ListContainers(ctx, all, filters)
	if err != nil {
		return nil, err
	}
	return lo.Map(items, func(item godocker.APIContainers, _ int) RuntimeContainer {
		return RuntimeContainer{
			ID:     item.ID,
			Names:  item.Names,
			Image:  item.Image,
			Labels: item.Labels,
		}
	}), nil
}
