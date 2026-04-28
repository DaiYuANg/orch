// Package composeimport maps Docker Compose specifications into deploy/v1alpha1.App.
// Runtime providers and schedulers must only accept the canonical App after conversion.
package composeimport

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	composetypes "github.com/compose-spec/compose-go/v2/types"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// LoadComposeFile parses a Docker Compose file with compose-spec/go and maps it to the canonical App.
func LoadComposeFile(ctx context.Context, path string) (*Result, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "compose path")
	}

	details, err := loader.LoadConfigFiles(ctx, []string{abs}, filepath.Dir(abs))
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "compose load config files")
	}
	mergeOSEnv(details)

	proj, err := loader.LoadWithContext(ctx, *details, func(o *loader.Options) {
		o.SkipConsistencyCheck = true
	})
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "compose parse")
	}

	return MapProject(proj)
}

func mergeOSEnv(details *composetypes.ConfigDetails) {
	if details == nil {
		return
	}
	env := composetypes.Mapping{}
	for _, kv := range os.Environ() {
		idx := strings.IndexByte(kv, '=')
		if idx <= 0 {
			continue
		}
		env[kv[:idx]] = kv[idx+1:]
	}
	details.Environment = env
}
