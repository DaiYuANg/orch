//go:build windows
// +build windows

package pkg

import "golang.org/x/sys/windows/registry"

// 获取 Windows MachineGuid
func sysMachineId() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer func(k registry.Key) {
		_ = k.Close()
	}(k)
	id, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return ""
	}
	return id
}
