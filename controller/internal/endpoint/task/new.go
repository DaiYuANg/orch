package task

import "github.com/danielgtaylor/huma/v2"

func NewTaskEndpoint() *Endpoint {
	return &Endpoint{}
}

func (e Endpoint) Register(openapi huma.API) {
	huma.Get[Input, Output](openapi, "/task", e.submitTask)
}
