package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

// DefaultDataRoot returns the directory used for orch persistent state (DNS DB,
// Raft logs/bolt/snapshots). Resolution order:
//
//  1. ORCH_DATA_DIR — explicit override (must already be in the process environment
//     when defaults are applied; use exported shell vars or container env).
//  2. XDG data home + /orch — from [github.com/adrg/xdg]: honors XDG_DATA_HOME and
//     OS-appropriate defaults (e.g. Linux ~/.local/share, macOS ~/Library/Application Support,
//     Windows %LOCALAPPDATA%).
//
// If [xdg.DataHome] is empty after reload, falls back to a "data" directory under [os.Getwd].
func DefaultDataRoot() string {
	if v := strings.TrimSpace(os.Getenv("ORCH_DATA_DIR")); v != "" {
		return filepath.Clean(v)
	}
	xdg.Reload()
	base := strings.TrimSpace(xdg.DataHome)
	if base == "" {
		return filepath.Clean(fallbackRelDataRoot())
	}
	return filepath.Join(base, "orch")
}

func fallbackRelDataRoot() string {
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "data")
	}
	return "data"
}
