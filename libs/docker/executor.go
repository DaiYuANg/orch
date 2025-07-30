package docker

import (
	docker "github.com/fsouza/go-dockerclient"
	"os"
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
func (e *Executor) runContainer() (*docker.Container, error) {
	imageName := "postgres:latest"
	// 拉取镜像
	err := e.client.PullImage(docker.PullImageOptions{
		OutputStream: os.Stdout,
		Registry:     imageName,
	}, docker.AuthConfiguration{})
	if err != nil {
		return nil, err
	}

	container, err := e.client.CreateContainer(docker.CreateContainerOptions{
		Name:       "",
		Platform:   "",
		Config:     nil,
		HostConfig: nil,
		Context:    nil,
	})
	if err != nil {
		return nil, err
	}
	return container, nil
}
