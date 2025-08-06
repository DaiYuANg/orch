package runtime_engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type RuntimeInfo struct {
	Name          string // 比如 python, java
	Executable    string // 可执行路径
	DefaultArgs   []string
	EnvDetectFunc func() (string, error)
}

var supportedRuntimes = map[string]*RuntimeInfo{
	"python": {
		Name: "python",
		EnvDetectFunc: func() (string, error) {
			return exec.LookPath("python")
		},
	},
	"node": {
		Name: "node",
		EnvDetectFunc: func() (string, error) {
			return exec.LookPath("node")
		},
	},
	"java": {
		Name: "java",
		EnvDetectFunc: func() (string, error) {
			// 优先 JAVA_HOME
			javaHome := os.Getenv("JAVA_HOME")
			if javaHome != "" {
				javaPath := filepath.Join(javaHome, "bin", "java")
				if _, err := os.Stat(javaPath); err == nil {
					return javaPath, nil
				}
			}
			// fallback to system PATH
			return exec.LookPath("java")
		},
	},
}

func NewLanguageRuntimeExecutor(runtime string, workDir string, args []string) (*LanguageRuntimeExecutor, error) {
	rt, ok := supportedRuntimes[runtime]
	if !ok {
		return nil, fmt.Errorf("runtime %s not supported", runtime)
	}

	exePath, err := rt.EnvDetectFunc()
	if err != nil {
		return nil, fmt.Errorf("cannot find executable for %s: %v", runtime, err)
	}

	cmd := exec.Command(exePath, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	return &LanguageRuntimeExecutor{
		Cmd:         cmd,
		Interpreter: exePath,
		Args:        args,
		WorkDir:     workDir,
		Env:         cmd.Env,
	}, nil
}
