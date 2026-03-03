package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	godocker "github.com/fsouza/go-dockerclient"
)

type Executor struct {
	client *godocker.Client
}

type RunSpec struct {
	Name   string
	Image  string
	Cmd    []string
	Env    map[string]string
	Labels map[string]string
	Ports  map[string]int
}

type RuntimeStatus struct {
	ContainerID string
	Name        string
	Running     bool
	State       string
	ExitCode    int
	Error       string
	StartedAt   time.Time
	FinishedAt  time.Time
	Health      string
}

func NewExecutor() (*Executor, error) {
	client, err := godocker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &Executor{client: client}, nil
}

func (e *Executor) Ping(ctx context.Context) error {
	return e.client.PingWithContext(ctx)
}

func (e *Executor) RunContainer(ctx context.Context, spec RunSpec) (string, error) {
	if strings.TrimSpace(spec.Image) == "" {
		return "", fmt.Errorf("docker run requires image")
	}
	if err := e.pullImage(ctx, spec.Image); err != nil {
		return "", err
	}

	exposedPorts, portBindings := buildPortBindings(spec.Ports)
	container, err := e.client.CreateContainer(godocker.CreateContainerOptions{
		Name: strings.TrimSpace(spec.Name),
		Config: &godocker.Config{
			Image:        spec.Image,
			Cmd:          spec.Cmd,
			Env:          mapToEnv(spec.Env),
			Labels:       spec.Labels,
			ExposedPorts: exposedPorts,
		},
		HostConfig: &godocker.HostConfig{
			PortBindings: portBindings,
		},
		Context: ctx,
	})
	if err != nil {
		return "", fmt.Errorf("create docker container: %w", err)
	}

	if err := e.client.StartContainerWithContext(container.ID, nil, ctx); err != nil {
		return "", fmt.Errorf("start docker container: %w", err)
	}

	return container.ID, nil
}

func (e *Executor) StopAndRemove(ctx context.Context, containerID string) error {
	id := strings.TrimSpace(containerID)
	if id == "" {
		return fmt.Errorf("container id is empty")
	}

	_ = e.client.StopContainerWithContext(id, 10, ctx)

	err := e.client.RemoveContainer(godocker.RemoveContainerOptions{
		ID:            id,
		Force:         true,
		RemoveVolumes: true,
		Context:       ctx,
	})
	if err != nil && !isNoSuchContainer(err) {
		return fmt.Errorf("remove docker container: %w", err)
	}
	return nil
}

func (e *Executor) Status(ctx context.Context, containerID string) (RuntimeStatus, error) {
	c, err := e.client.InspectContainerWithContext(strings.TrimSpace(containerID), ctx)
	if err != nil {
		return RuntimeStatus{}, err
	}
	status := RuntimeStatus{
		ContainerID: c.ID,
		Name:        strings.TrimPrefix(c.Name, "/"),
		Running:     c.State.Running,
		State:       c.State.StateString(),
		ExitCode:    c.State.ExitCode,
		Error:       c.State.Error,
		StartedAt:   c.State.StartedAt,
		FinishedAt:  c.State.FinishedAt,
		Health:      c.State.Health.Status,
	}
	return status, nil
}

func (e *Executor) Logs(ctx context.Context, containerID string, tail int) (string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	tailString := "200"
	if tail > 0 {
		tailString = strconv.Itoa(tail)
	}
	if err := e.client.Logs(godocker.LogsOptions{
		Context:      ctx,
		Container:    strings.TrimSpace(containerID),
		OutputStream: &stdout,
		ErrorStream:  &stderr,
		Stdout:       true,
		Stderr:       true,
		Tail:         tailString,
	}); err != nil {
		return "", fmt.Errorf("docker logs: %w", err)
	}

	logText := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	return logText, nil
}

func (e *Executor) ListContainers(ctx context.Context, all bool, filters map[string][]string) ([]godocker.APIContainers, error) {
	items, err := e.client.ListContainers(godocker.ListContainersOptions{
		All:     all,
		Filters: filters,
		Context: ctx,
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func buildPortBindings(ports map[string]int) (map[godocker.Port]struct{}, map[godocker.Port][]godocker.PortBinding) {
	if len(ports) == 0 {
		return nil, nil
	}
	exposed := make(map[godocker.Port]struct{}, len(ports))
	bindings := make(map[godocker.Port][]godocker.PortBinding, len(ports))
	for _, port := range ports {
		if port <= 0 {
			continue
		}
		dockerPort := godocker.Port(fmt.Sprintf("%d/tcp", port))
		exposed[dockerPort] = struct{}{}
		bindings[dockerPort] = []godocker.PortBinding{
			{
				HostIP:   "0.0.0.0",
				HostPort: strconv.Itoa(port),
			},
		}
	}
	if len(exposed) == 0 {
		return nil, nil
	}
	return exposed, bindings
}

func mapToEnv(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	items := make([]string, 0, len(env))
	for k, v := range env {
		items = append(items, fmt.Sprintf("%s=%s", k, v))
	}
	return items
}

func (e *Executor) pullImage(ctx context.Context, image string) error {
	if strings.TrimSpace(image) == "" {
		return fmt.Errorf("image is required")
	}
	err := e.client.PullImage(godocker.PullImageOptions{
		Repository: image,
		Context:    ctx,
	}, godocker.AuthConfiguration{})
	if err != nil && !strings.Contains(err.Error(), "already being pulled") {
		return fmt.Errorf("pull image %s: %w", image, err)
	}
	return nil
}

func isNoSuchContainer(err error) bool {
	if err == nil {
		return false
	}
	var noSuch *godocker.NoSuchContainer
	if strings.Contains(err.Error(), "No such container") {
		return true
	}
	return errors.As(err, &noSuch)
}
