package win_svc

import (
	"fmt"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
	"time"
)

func CreateService() {
	m, err := mgr.Connect()
	serviceName := "MyTestService"

	servicePath := `C:\path\to\your\service.exe`

	// 创建服务，参数依次是：
	// 服务名、服务执行路径、配置选项
	s, err := m.CreateService(serviceName, servicePath, mgr.Config{
		DisplayName: "My Test Service",
		StartType:   mgr.StartAutomatic, // 自动启动
	}, "arg1", "arg2") // 传递给服务程序的参数（可选）
	if err != nil {
		panic(err)
	}
	defer s.Close()

	fmt.Println("Service created successfully.")
}

func StartService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func(m *mgr.Mgr) {
		err := m.Disconnect()
		if err != nil {
			println(err)
		}
	}(m)

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Start()
}

func StopService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return err
	}

	// 等待状态变更
	timeout := time.Now().Add(10 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return fmt.Errorf("timeout waiting for stop")
		}
		time.Sleep(500 * time.Millisecond)
		status, _ = s.Query()
	}

	return nil
}
