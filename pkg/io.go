package pkg

import (
	"fmt"
	"github.com/samber/mo"
	"os"
)

func EnsureDir(path string, perm os.FileMode) mo.Result[struct{}] {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return mo.Err[struct{}](fmt.Errorf("path exists but is not a directory: %s", path))
		}
		return mo.Ok(struct{}{})
	}
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, perm); err != nil {
			return mo.Err[struct{}](fmt.Errorf("failed to create directory %s: %w", path, err))
		}
		return mo.Ok(struct{}{})
	}
	return mo.Err[struct{}](fmt.Errorf("failed to check directory %s: %w", path, err))
}
