package runtimeinfo

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

type Status struct {
	Name      string               `json:"name"`
	Runtime   deployv1.RuntimeKind `json:"runtime"`
	Status    string               `json:"status"`
	NativeID  string               `json:"nativeId,omitempty"`
	StartedAt time.Time            `json:"startedAt,omitzero"`
	UpdatedAt time.Time            `json:"updatedAt,omitzero"`
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
	content, err := readLogFile(path)
	if err != nil || content == "" {
		return content, err
	}
	return tailLogContent(content, tail), nil
}

func readLogFile(path string) (string, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read log: %w", err)
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			slog.Default().Warn("runtime log dir close failed", "error", closeErr)
		}
	}()
	b, err := root.ReadFile(filepath.Base(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read tail file: %w", err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), nil
}

func tailLogContent(content string, tail int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	tail = NormalizeTailLines(tail)
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
