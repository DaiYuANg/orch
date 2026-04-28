package composeimport

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestLoadComposeFile_mapsServices(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := filepath.Join(dir, "compose.yaml")
	content := `
name: orch-test
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    depends_on:
      - db
  db:
    image: postgres:15
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := LoadComposeFile(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err := res.App.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if len(res.App.Workloads) != 2 {
		t.Fatalf("workloads: got %d", len(res.App.Workloads))
	}
	var web *deployv1.Workload
	for i := range res.App.Workloads {
		if res.App.Workloads[i].Name == "web" {
			web = &res.App.Workloads[i]
			break
		}
	}
	if web == nil {
		t.Fatal("missing web workload")
	}
	if web.Run.Image != "nginx:alpine" {
		t.Fatalf("image %q", web.Run.Image)
	}
	if len(web.DependsOn) != 1 || web.DependsOn[0].Name != "db" {
		t.Fatalf("dependsOn %+v", web.DependsOn)
	}
}
