package runtime_engine

import (
	"os"

	docker "github.com/fsouza/go-dockerclient"
)

type DockerExecutor struct {
	client *docker.Client
}

func NewExecutor() (*DockerExecutor, error) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return &DockerExecutor{client: client}, nil
}

func (e *DockerExecutor) ListImage() ([]docker.APIImages, error) {
	return e.client.ListImages(docker.ListImagesOptions{})
}
func (e *DockerExecutor) runContainer() (*docker.Container, error) {
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
