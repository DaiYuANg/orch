package systemd

import "strings"

// RuntimeStatusFromActiveState maps systemd's ActiveState to orch runtime status.
func RuntimeStatusFromActiveState(activeState string) string {
	switch strings.TrimSpace(activeState) {
	case "active":
		return "running"
	case "activating", "reloading", "refreshing":
		return "starting"
	case "deactivating":
		return "stopping"
	case "inactive":
		return "stopped"
	case "failed":
		return "failed"
	default:
		if activeState == "" {
			return "unknown"
		}
		return strings.TrimSpace(activeState)
	}
}
