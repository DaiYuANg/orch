package docker

import (
	docker "github.com/fsouza/go-dockerclient"
)

type Executor struct {
	client *docker.Client
}

func NewExecutor() (*Executor, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &Executor{client: client}, nil
}

func (e *Executor) ListImage() ([]docker.APIImages, error) {
	return e.client.ListImages(docker.ListImagesOptions{})
}
