package runtimeinfo

import (
	"errors"
	"os"
	"strings"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

type Status struct {
	Name      string               `json:"name"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Status    string               `json:"status"`
	NativeID  string               `json:"nativeId,omitempty"`
	StartedAt time.Time            `json:"startedAt,omitempty"`
	UpdatedAt time.Time            `json:"updatedAt,omitempty"`
	Message   string               `json:"message,omitempty"`
}

type LogOptions struct {
	Tail int `json:"tail,omitempty"`
}

type LogResult struct {
	Name    string               `json:"name"`
	Runtime deployv1.RuntimeKind `json:"runtime"`
	Source  string               `json:"source,omitempty"`
	Content string               `json:"content"`
}

func NormalizeTailLines(tail int) int {
	if tail <= 0 {
		return 100
	}
	if tail > 5000 {
		return 5000
	}
	return tail
}

func ReadTailFile(path string, tail int) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	content := strings.ReplaceAll(string(b), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	tail = NormalizeTailLines(tail)
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.Join(lines, "\n") + "\n", nil
}
