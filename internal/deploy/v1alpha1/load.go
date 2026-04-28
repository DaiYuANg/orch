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
func LoadAppFile(path string) (*App, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var app App
	if err := yaml.Unmarshal(b, &app); err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "parse %s", filepath.Base(path))
	}

	// Fill in friendly defaults (keep schema additive).
	if app.APIVersion == "" {
		app.APIVersion = "warden.arcgolabs.io/v1alpha1"
	}
	if app.Kind == "" {
		app.Kind = "App"
	}
	return &app, nil
}
