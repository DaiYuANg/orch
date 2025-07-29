package executor

// Executor 定义了不同运行时应该实现的基础操作
type Executor interface {
	// Start 启动运行时（例如，容器、服务等）
	Start() error
	// Stop 停止运行时
	Stop() error
	// Status 获取运行时的状态（例如，正在运行、停止等）
	Status() (string, error)
}
