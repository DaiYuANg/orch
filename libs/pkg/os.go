package pkg

import (
	"runtime"
)

type OperatingSystem string

const (
	OSLinux   OperatingSystem = "linux"
	OSDarwin  OperatingSystem = "darwin"
	OSWindows OperatingSystem = "windows"
	OSFreeBSD OperatingSystem = "freebsd"
	OSOther   OperatingSystem = "other"
)

func CurrentOS() OperatingSystem {
	switch runtime.GOOS {
	case "linux":
		return OSLinux
	case "darwin":
		return OSDarwin
	case "windows":
		return OSWindows
	case "freebsd":
		return OSFreeBSD
	default:
		return OSOther
	}
}

func IsUnixLike() bool {
	switch CurrentOS() {
	case OSLinux, OSDarwin, OSFreeBSD:
		return true
	default:
		return false
	}
}
