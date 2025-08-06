package runtime_engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type Executor interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Status(ctx context.Context) (string, error)
}

type LanguageRuntimeExecutor struct {
	Cmd         *exec.Cmd
	Interpreter string   // 如 "java", "node", "python"
	Args        []string // 如 ["-jar", "app.jar"]
	WorkDir     string   // 脚本所在路径
	Env         []string // 环境变量（可选）
}

func (e *LanguageRuntimeExecutor) Start(ctx context.Context) error {
	e.Cmd = exec.CommandContext(ctx, e.Interpreter, e.Args...)
	e.Cmd.Dir = e.WorkDir
	e.Cmd.Env = append(os.Environ(), e.Env...)

	// 可选：连接 stdout/stderr
	e.Cmd.Stdout = os.Stdout
	e.Cmd.Stderr = os.Stderr

	return e.Cmd.Start()
}

func (e *LanguageRuntimeExecutor) Stop(ctx context.Context) error {
	if e.Cmd == nil || e.Cmd.Process == nil {
		return fmt.Errorf("no running process")
	}
	return e.Cmd.Process.Kill()
}
