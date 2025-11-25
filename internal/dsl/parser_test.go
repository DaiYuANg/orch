package dsl

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseYAML(t *testing.T) {
	path := filepath.Join("example.yaml")
	workload, err := ParseYAML(path)

	assert.NoError(t, err, "should parse YAML without error")
	assert.NotNil(t, workload, "workload should not be nil")

	assert.Equal(t, "my-app", workload.Name)
	assert.Len(t, workload.Units, 1)

	unit := workload.Units[0]
	assert.Equal(t, "backend", unit.Name)
	assert.Len(t, unit.Tasks, 1)

	task := unit.Tasks[0]
	assert.Equal(t, "api", task.Name)
	assert.Equal(t, "docker", task.Driver)

	fmt.Printf("%+v\n", workload)
}

func TestParseHCL(t *testing.T) {
	path := filepath.Join("example.hcl")
	workload, err := ParseHCL(path)

	assert.NoError(t, err, "should parse HCL without error")
	assert.NotNil(t, workload, "workload should not be nil")

	assert.Equal(t, "my-app", workload.Name)
	assert.Len(t, workload.Units, 1)

	unit := workload.Units[0]
	assert.Equal(t, "backend", unit.Name)
	assert.Len(t, unit.Tasks, 1)

	task := unit.Tasks[0]
	assert.Equal(t, "api", task.Name)
	assert.Equal(t, "docker", task.Driver)

	fmt.Printf("%+v\n", workload)
}
