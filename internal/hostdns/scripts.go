package hostdns

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed scripts/*
var hostDNSScripts embed.FS

// TemplateData contains variables used by embedded host DNS installer templates.
type TemplateData struct {
	Zone       string
	Namespace  string
	Nameserver string
	DNSServer  string
	Port       int
}

// RenderTemplate renders an embedded host DNS installer template.
func RenderTemplate(name string, data TemplateData) (string, error) {
	raw, err := hostDNSScripts.ReadFile("scripts/" + name)
	if err != nil {
		return "", fmt.Errorf("read embedded host DNS template %s: %w", name, err)
	}
	tpl, err := template.New(name).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse embedded host DNS template %s: %w", name, err)
	}
	var out bytes.Buffer
	if err := tpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render embedded host DNS template %s: %w", name, err)
	}
	return out.String(), nil
}
