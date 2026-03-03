package task

import (
	tasksvc "github.com/DaiYuANg/warden/internal/task"
	"github.com/danielgtaylor/huma/v2"
)

func NewTaskEndpoint(service *tasksvc.Service) *Endpoint {
	return &Endpoint{service: service}
}

func (e Endpoint) Register(openapi huma.API) {
	tag := huma.OperationTags("task")
	huma.Post(openapi, "/tasks/deploy", e.submitTask, tag)
	huma.Get(openapi, "/tasks", e.listDeployment, tag)
	huma.Get(openapi, "/tasks/{id}", e.getDeployment, tag)
	huma.Post(openapi, "/tasks/{id}/stop", e.stopDeployment, tag)
	huma.Get(openapi, "/tasks/instances/{id}/logs", e.getInstanceLogs, tag)
}
