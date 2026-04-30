package v1alpha1

import (
	"os"
	"path/filepath"

	"github.com/daiyuang/orch/pkg/oopsx"
	"gopkg.in/yaml.v3"
)

// LoadAppFile loads an App from a YAML file. The file is expected to be a
// canonical deploy YAML (not the Kotlin-style .wd DSL).
//
// For .orch plano manifests, resolve internal/deploy/loader.Loader from DI (see cmd/orch-cli/cliapp).
//
// Minimal example:
//
//	apiVersion: warden.arcgolabs.io/v1alpha1
//	kind: App
//	metadata:
//	  name: mall
//	  namespace: default
//	workloads:
//	- name: redis
//	  kind: stateful
//	  runtime: containerd
//	  run:
//	    image: redis:7
//
// ParseAppYAML unmarshals canonical deploy YAML (or JSON, when valid as YAML) from bytes
// and applies the same defaults as [LoadAppFile].
func ParseAppYAML(data []byte) (*App, error) {
	var app App
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "parse app yaml")
	}
	defaultAppMeta(&app)
	return &app, nil
}

func defaultAppMeta(app *App) {
	if app.APIVersion == "" {
		app.APIVersion = "warden.arcgolabs.io/v1alpha1"
	}
	if app.Kind == "" {
		app.Kind = "App"
	}
}

func LoadAppFile(path string) (*App, error) {
	path = filepath.Clean(path)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "read %s", filepath.Base(path))
	}

	app, err := ParseAppYAML(b)
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "parse %s", filepath.Base(path))
	}
	return app, nil
}
