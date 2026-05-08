package buildmeta

import "runtime/debug"

var (
	version = ""
	commit  = ""
	date    = ""
)

// Version returns the main module version from [debug.ReadBuildInfo] (Go toolchain / module version).
//
// Released artifacts built from a tagged module (e.g. go install @v1.2.3) show that semver.
// Local "go run" / workspace builds usually report "(devel)"; that value is returned as-is.
func Version() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	v := info.Main.Version
	if v == "" {
		return "unknown"
	}
	return v
}

func Commit() string {
	if commit != "" {
		return commit
	}
	return "unknown"
}

func Date() string {
	if date != "" {
		return date
	}
	return "unknown"
}
