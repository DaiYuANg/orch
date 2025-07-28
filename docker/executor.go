package docker

import (
	docker "github.com/fsouza/go-dockerclient"
)

type Executor struct {
	Runtime string
	Image   string
	Name    string
	client  *docker.Client
}

func NewExecutor() (*Executor, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &Executor{client: client}, nil
}
