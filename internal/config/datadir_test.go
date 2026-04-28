package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDataRoot_ORCH_DATA_DIR(t *testing.T) {
	t.Setenv("ORCH_DATA_DIR", filepath.FromSlash("/custom/state"))
	t.Setenv("XDG_DATA_HOME", filepath.FromSlash("/should-not-use"))

	got := DefaultDataRoot()
	want := filepath.Clean(filepath.FromSlash("/custom/state"))
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestDefaultDataRoot_XDG_DATA_HOME(t *testing.T) {
	t.Setenv("ORCH_DATA_DIR", "")
	// adrg/xdg only honors XDG_DATA_HOME when it is an absolute path (see pathutil.EnvPath).
	dir := filepath.Join(t.TempDir(), "xdg-data")
	t.Setenv("XDG_DATA_HOME", dir)

	got := DefaultDataRoot()
	want := filepath.Join(dir, "orch")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFallbackRelDataRoot_hasComponents(t *testing.T) {
	s := fallbackRelDataRoot()
	if !strings.Contains(s, "data") {
		t.Fatalf("expected path segment data, got %q", s)
	}
}
