package ingress

import (
	"errors"
	"path"
	"strings"

	"github.com/daiyuang/orch/internal/config"
)

type routeMeta struct {
	prefixNorm string // "" = match all (root); else e.g. "api"
	stripPath  string // e.g. "/api"
}

func newRouteMeta(raw *config.IngressRoute) (routeMeta, error) {
	if len(raw.UpstreamEndpoints()) == 0 {
		return routeMeta{}, errors.New("no upstream endpoints")
	}

	pp := normalizePathPrefix(raw.PathPrefix)
	if pp == "/" {
		return routeMeta{prefixNorm: "", stripPath: "/"}, nil
	}

	prefixNorm := strings.Trim(strings.TrimPrefix(pp, "/"), "/")
	strip := strings.TrimSpace(raw.StripPrefix)
	if strip == "" {
		strip = pp
	}
	stripPath := path.Clean(strip)
	if stripPath != "/" && !strings.HasPrefix(stripPath, "/") {
		stripPath = "/" + stripPath
	}

	return routeMeta{
		prefixNorm: prefixNorm,
		stripPath:  stripPath,
	}, nil
}

func normalizedPath(p string) string {
	p = path.Clean(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func normalizePathPrefix(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || p == "/" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if p != "/" {
		p = strings.TrimSuffix(p, "/")
	}
	return p
}

func (m routeMeta) matches(reqPath string) bool {
	if m.prefixNorm == "" {
		return true
	}
	base := "/" + m.prefixNorm
	if reqPath == base {
		return true
	}
	return strings.HasPrefix(reqPath, base+"/")
}

// pathRel returns the suffix path (always starting with /) sent to upstreams.
func (m routeMeta) pathRel(reqPath string) (rel string, ok bool) {
	switch {
	case m.prefixNorm == "":
		return reqPath, true
	case !strings.HasPrefix(reqPath, m.stripPath):
		return "", false
	default:
		rel = strings.TrimPrefix(reqPath, m.stripPath)
		if rel == "" {
			rel = "/"
		} else if !strings.HasPrefix(rel, "/") {
			rel = "/" + rel
		}
		return rel, true
	}
}
