package task

import "golang.org/x/net/context"

type Endpoint struct {
}

type Input struct {
	Body struct {
		DeployFile []byte
	}
}

type Output struct {
	Body struct {
		Message string `json:"message"`
	}
}

func (e Endpoint) submitTask(context context.Context, Input *Input) (*Output, error) {

	return &Output{}, nil
}
