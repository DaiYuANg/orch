package runtime_engine

func NewContainerdExecutor(socket string) (*ContainerdExecutor, error) {
	client, err := containerd.New(socket)
	if err != nil {
		return nil, err
	}
	return &ContainerdExecutor{
		client: client,
	}, nil
}
