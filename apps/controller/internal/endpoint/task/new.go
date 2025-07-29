package task

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/go-playground/validator/v10"
)

func NewTaskEndpoint(validator *validator.Validate) *Endpoint {
	return &Endpoint{}
}

func (e Endpoint) Register(openapi huma.API) {
	huma.Get[Input, Output](openapi, "/dsl", e.submitTask)
}
