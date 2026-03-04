package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntimeFactoriesIncludesAllDrivers(t *testing.T) {
	runtime := newFakeRuntime("mock")
	factories := newRuntimeFactories(func() (RuntimeExecutor, error) {
		return runtime, nil
	})

	expectedDrivers := []string{
		driverDocker,
		driverContainerd,
		driverSystemd,
		driverFirecracker,
		driverWindowsService,
	}

	for _, driver := range expectedDrivers {
		factory, ok := factories[driver]
		require.True(t, ok)
		require.NotNil(t, factory)
	}
}

func TestNormalizeRuntimeDriverAliases(t *testing.T) {
	assert.Equal(t, driverDocker, normalizeRuntimeDriver(""))
	assert.Equal(t, driverWindowsService, normalizeRuntimeDriver("windowsservice"))
	assert.Equal(t, driverWindowsService, normalizeRuntimeDriver("windows_service"))
	assert.Equal(t, driverFirecracker, normalizeRuntimeDriver("fire-cracker"))
	assert.Equal(t, driverContainerd, normalizeRuntimeDriver(" containerd "))
}
