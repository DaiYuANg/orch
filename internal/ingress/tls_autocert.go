package ingress

import (
	"crypto/tls"
	"path/filepath"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func newAutocertManager(tlsCfg config.IngressTLSAuto, domains *list.List[string], dataRoot string) (*autocert.Manager, error) {
	if domains.Len() == 0 {
		return nil, oopsx.B("ingress").Errorf("ingress.tls.domains is required when ingress.tls.enabled")
	}
	cacheDir := strings.TrimSpace(tlsCfg.CacheDir)
	if cacheDir == "" {
		cacheDir = filepath.Join(dataRoot, "autocert")
	}
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		Email:      strings.TrimSpace(tlsCfg.Email),
		HostPolicy: autocert.HostWhitelist(domains.Values()...),
	}
	dirURL := acme.LetsEncryptURL
	if tlsCfg.Staging {
		dirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
	}
	m.Client = &acme.Client{DirectoryURL: dirURL}
	return m, nil
}

func serverTLSConfig(m *autocert.Manager) *tls.Config {
	return &tls.Config{
		GetCertificate: m.GetCertificate,
		MinVersion:     tls.VersionTLS12,
		NextProtos:     []string{"h2", "http/1.1", acme.ALPNProto},
	}
}
