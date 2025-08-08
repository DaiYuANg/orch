package engine

import "github.com/DaiYuANg/warden/runtime_engine/systemd"

type Engine struct {
	systemdManager *systemd.SystemdManager
	//firecrackerManager *FirecrackerManager
}

func NewEngine() {

}
