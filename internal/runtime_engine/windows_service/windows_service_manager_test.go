package windows_service

import (
	"os"
	"runtime"
	"testing"
	"time"
)

func TestWindowsServiceManager_Lifecycle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("This test can only run on Windows.")
	}

	manager, err := NewWindowsServiceManager()
	if err != nil {
		t.Fatalf("Failed to init service manager: %v", err)
	}
	defer func(manager *WindowsServiceManager) {
		_ = manager.Close()
	}(manager)

	serviceName := "TestServiceDemo"
	exePath := os.Getenv("SystemRoot") + `\System32\cmd.exe`

	// 清理残留服务（可选）
	_ = manager.StopService(serviceName)
	//_ = manager.DeleteService(serviceName)
	time.Sleep(1 * time.Second)

	// Create
	if err := manager.CreateService(serviceName, exePath, "/C", "echo", "hello from service"); err != nil {
		t.Errorf("CreateService failed: %v", err)
	}

	// Start
	if err := manager.StartService(serviceName); err != nil {
		t.Errorf("StartService failed: %v", err)
	}

	// Stop
	if err := manager.StopService(serviceName); err != nil {
		t.Errorf("StopService failed: %v", err)
	}

	// Delete
	if err := manager.DeleteService(serviceName); err != nil {
		t.Errorf("DeleteService failed: %v", err)
	}

	t.Log("Windows service lifecycle test passed.")
}
