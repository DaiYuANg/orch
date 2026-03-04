package task

import (
	"context"
	"fmt"

	"github.com/DaiYuANg/warden/internal/http/model"
	tasksvc "github.com/DaiYuANg/warden/internal/task"
)

type Endpoint struct {
	service *tasksvc.Service
}

type deployInput struct {
	Body struct {
		Filename string `json:"filename,omitempty" doc:"optional source file name, used to auto detect format"`
		Format   string `json:"format,omitempty" enum:"yaml,hcl" doc:"optional, if empty will detect by filename"`
		Content  string `json:"content" doc:"dsl content"`
	}
}

type deploymentPathInput struct {
	ID string `path:"id"`
}

type instanceLogsPathInput struct {
	ID   string `path:"id"`
	Tail int    `query:"tail" default:"200"`
}

type internalRunInput struct {
	Body tasksvc.InternalRunRequest
}

type internalStopInput struct {
	Body tasksvc.InternalStopRequest
}

type internalLogsPathInput struct {
	ContainerID string `path:"container_id"`
	Driver      string `query:"driver,omitempty"`
	Tail        int    `query:"tail" default:"200"`
}

type internalStatusPathInput struct {
	ContainerID string `path:"container_id"`
	Driver      string `query:"driver,omitempty"`
}

func (e *Endpoint) submitTask(ctx context.Context, input *deployInput) (*struct {
	Body model.Response[tasksvc.DeployResult]
}, error) {
	result, err := e.service.Deploy(ctx, tasksvc.DeployRequest{
		Filename: input.Body.Filename,
		Format:   input.Body.Format,
		Content:  input.Body.Content,
	})
	if err != nil {
		return nil, err
	}
	return model.WrapResponse(*result), nil
}

func (e *Endpoint) getDeployment(ctx context.Context, input *deploymentPathInput) (*struct {
	Body model.Response[tasksvc.DeploymentDetail]
}, error) {
	detail, ok := e.service.GetDeployment(input.ID)
	if !ok {
		return nil, fmt.Errorf("deployment not found: %s", input.ID)
	}
	return model.WrapResponse(detail), nil
}

func (e *Endpoint) listDeployment(ctx context.Context, input *struct{}) (*struct {
	Body model.Response[[]tasksvc.DeploymentInfo]
}, error) {
	return model.WrapResponse(e.service.ListDeployments()), nil
}

func (e *Endpoint) stopDeployment(ctx context.Context, input *deploymentPathInput) (*struct {
	Body model.Response[struct {
		Stopped bool `json:"stopped"`
	}]
}, error) {
	if err := e.service.StopDeployment(ctx, input.ID); err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Stopped bool `json:"stopped"`
	}{
		Stopped: true,
	}), nil
}

func (e *Endpoint) getInstanceLogs(ctx context.Context, input *instanceLogsPathInput) (*struct {
	Body model.Response[struct {
		Logs string `json:"logs"`
	}]
}, error) {
	logs, err := e.service.Logs(ctx, input.ID, input.Tail)
	if err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Logs string `json:"logs"`
	}{
		Logs: logs,
	}), nil
}

func (e *Endpoint) internalRun(ctx context.Context, input *internalRunInput) (*struct {
	Body model.Response[tasksvc.InternalRunResult]
}, error) {
	result, err := e.service.InternalRun(ctx, input.Body)
	if err != nil {
		return nil, err
	}
	return model.WrapResponse(*result), nil
}

func (e *Endpoint) internalStop(ctx context.Context, input *internalStopInput) (*struct {
	Body model.Response[struct {
		Stopped bool `json:"stopped"`
	}]
}, error) {
	if err := e.service.InternalStop(ctx, input.Body); err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Stopped bool `json:"stopped"`
	}{
		Stopped: true,
	}), nil
}

func (e *Endpoint) internalLogs(ctx context.Context, input *internalLogsPathInput) (*struct {
	Body model.Response[struct {
		Logs string `json:"logs"`
	}]
}, error) {
	logs, err := e.service.InternalLogs(ctx, input.ContainerID, input.Driver, input.Tail)
	if err != nil {
		return nil, err
	}
	return model.WrapResponse(struct {
		Logs string `json:"logs"`
	}{
		Logs: logs,
	}), nil
}

func (e *Endpoint) internalStatus(ctx context.Context, input *internalStatusPathInput) (*struct {
	Body model.Response[tasksvc.RuntimeStatus]
}, error) {
	status, err := e.service.InternalStatus(ctx, input.ContainerID, input.Driver)
	if err != nil {
		return nil, err
	}
	return model.WrapResponse(status), nil
}
