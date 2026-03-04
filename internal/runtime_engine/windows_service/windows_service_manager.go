//go:build windows
// +build windows

package windows_service

import (
	"fmt"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type WindowsServiceManager struct {
	mgr *mgr.Mgr
}

// NewWindowsServiceManager 初始化 WindowsServiceManager
func NewWindowsServiceManager() (*WindowsServiceManager, error) {
	if runtime.GOOS != "windows" {
		return nil, fmt.Errorf("当前系统不支持 Windows 服务")
	}

	if !isAdmin() {
		return nil, fmt.Errorf("当前用户不是管理员，无法管理服务")
	}

	m, err := mgr.Connect()
	if err != nil {
		return nil, err
	}
	return &WindowsServiceManager{mgr: m}, nil
}

// 关闭连接
func (wsm *WindowsServiceManager) Close() error {
	return wsm.mgr.Disconnect()
}

// 创建服务
func (wsm *WindowsServiceManager) CreateService(name, displayName, exePath string, args ...string) error {
	s, err := wsm.mgr.CreateService(name, exePath, mgr.Config{
		DisplayName: displayName,
		StartType:   mgr.StartAutomatic,
	}, args...)
	if err != nil {
		return err
	}
	defer s.Close()
	fmt.Println("✅ 服务创建成功:", name)
	return nil
}

// 启动服务
func (wsm *WindowsServiceManager) StartService(name string) error {
	s, err := wsm.mgr.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Start()
}

// 停止服务
func (wsm *WindowsServiceManager) StopService(name string) error {
	s, err := wsm.mgr.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	// 等待服务真正停止
	timeout := time.Now().Add(10 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return fmt.Errorf("停止服务超时: %s", name)
		}
		time.Sleep(500 * time.Millisecond)
		status, _ = s.Query()
	}

	fmt.Println("🛑 服务已停止:", name)
	return nil
}

func (w *WindowsServiceManager) DeleteService(name string) error {
	s, err := w.mgr.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()
	return s.Delete()
}

func (w *WindowsServiceManager) QueryService(name string) (svc.Status, error) {
	s, err := w.mgr.OpenService(name)
	if err != nil {
		return svc.Status{}, err
	}
	defer s.Close()
	return s.Query()
}

// 权限判断工具（仅 Windows 有效）
func isAdmin() bool {
	var sid *windows.SID
	sid, _ = windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)

	token := windows.Token(0)
	isMember, err := token.IsMember(sid)
	return err == nil && isMember
}
