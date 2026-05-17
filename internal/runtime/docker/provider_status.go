package docker

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (p *Provider) Status(ctx context.Context, meta deployv1.Metadata, workloadName string) (runtimeinfo.Status, error) {
	cli, err := p.newDockerClient()
	if err != nil {
		return runtimeinfo.Status{}, err
	}
	defer p.closeDockerClient(cli)

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: workloadContainerFilters(meta, workloadName)})
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", p.runtime()).Wrapf(err, "docker list containers")
	}
	out := stoppedDockerStatus(workloadName, string(p.Kind()))
	if len(containers) == 0 {
		return out, nil
	}
	inspect, err := cli.ContainerInspect(ctx, containers[0].ID)
	if err != nil {
		return runtimeinfo.Status{}, oopsx.B("runtime", p.runtime()).Wrapf(err, "docker inspect container")
	}
	applyDockerInspectStatus(&out, inspect)
	return out, nil
}

func stoppedDockerStatus(workloadName, runtime string) runtimeinfo.Status {
	if runtime == "" {
		runtime = string(deployv1.RuntimeDocker)
	}
	return runtimeinfo.Status{Name: strings.TrimSpace(workloadName), Runtime: deployv1.RuntimeKind(runtime), Status: "stopped"}
}

func applyDockerInspectStatus(out *runtimeinfo.Status, inspect container.InspectResponse) {
	out.NativeID = inspect.ID
	out.UpdatedAt = time.Now().UTC()
	if inspect.State == nil {
		return
	}
	out.Status = dockerContainerStateStatus(inspect.State)
	out.Message = strings.TrimSpace(inspect.State.Error)
	if startedAt, ok := parseDockerStartedAt(inspect.State.StartedAt); ok {
		out.StartedAt = startedAt
	}
}

func dockerContainerStateStatus(state *container.State) string {
	if state.Running {
		return "running"
	}
	return strings.TrimSpace(state.Status)
}

func parseDockerStartedAt(value string) (time.Time, bool) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	return parsed, err == nil
}

func (p *Provider) Logs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	cli, err := p.newDockerClient()
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	defer p.closeDockerClient(cli)

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true, Filters: workloadContainerFilters(meta, workloadName)})
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", p.runtime()).Wrapf(err, "docker list containers")
	}
	if len(containers) == 0 {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", p.runtime()).Errorf("docker container for workload %q not found", workloadName)
	}
	content, err := p.readDockerLogs(ctx, cli, containers[0].ID, opts)
	if err != nil {
		return runtimeinfo.LogResult{}, err
	}
	return runtimeinfo.LogResult{
		Name:    strings.TrimSpace(workloadName),
		Runtime: p.Kind(),
		Source:  containers[0].ID,
		Content: content,
	}, nil
}

func (p *Provider) readDockerLogs(ctx context.Context, cli *client.Client, containerID string, opts runtimeinfo.LogOptions) (string, error) {
	reader, err := cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       strconv.Itoa(runtimeinfo.NormalizeTailLines(opts.Tail)),
	})
	if err != nil {
		return "", oopsx.B("runtime", p.runtime()).Wrapf(err, "docker logs")
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			p.logger.Warn("docker logs reader close", "error", closeErr)
		}
	}()
	return p.decodeDockerLogs(reader)
}

func (p *Provider) decodeDockerLogs(reader io.Reader) (string, error) {
	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
		return "", oopsx.B("runtime", p.runtime()).Wrapf(err, "docker logs decode")
	}
	return combineDockerLogs(stdout.String(), stderr.String()), nil
}

func combineDockerLogs(stdout, stderr string) string {
	if stderr == "" {
		return stdout
	}
	if stdout != "" && !strings.HasSuffix(stdout, "\n") {
		stdout += "\n"
	}
	return stdout + stderr
}

func (p *Provider) newDockerClient() (*client.Client, error) {
	if p.newClient == nil {
		return nil, oopsx.B("runtime", p.runtime()).Errorf("docker runtime client factory is not configured")
	}
	cli, err := p.newClient()
	if err != nil {
		return nil, oopsx.B("runtime", p.runtime()).Wrapf(err, "docker client")
	}
	return cli, nil
}

func defaultDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, oopsx.B("runtime", "docker").Wrapf(err, "docker client from env")
	}
	return cli, nil
}

func (p *Provider) closeDockerClient(cli *client.Client) {
	if closeErr := cli.Close(); closeErr != nil {
		p.logger.Warn("docker client close", "error", closeErr)
	}
}
