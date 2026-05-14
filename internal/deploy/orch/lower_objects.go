package orch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func lowerConfig(f *compiler.HIRForm) (v1.Config, error) {
	var config v1.Config
	name, err := symbolLabelName(f)
	if err != nil {
		return config, fmt.Errorf("config: %w", err)
	}
	config.Name = name
	data, ok := stringMapField(f, "data")
	if !ok || len(data) == 0 {
		return config, fmt.Errorf("config %q: data map is required", name)
	}
	config.Data = data
	return config, nil
}

func lowerSecret(f *compiler.HIRForm) (v1.Secret, error) {
	var secret v1.Secret
	name, err := symbolLabelName(f)
	if err != nil {
		return secret, fmt.Errorf("secret: %w", err)
	}
	secret.Name = name
	data, ok := stringMapField(f, "data")
	if !ok || len(data) == 0 {
		return secret, fmt.Errorf("secret %q: data map is required", name)
	}
	secret.Data = data
	return secret, nil
}

func lowerVolume(f *compiler.HIRForm) (v1.Volume, error) {
	var volume v1.Volume
	name, err := symbolLabelName(f)
	if err != nil {
		return volume, fmt.Errorf("volume: %w", err)
	}
	volume.Name = name
	if persistent, ok := boolField(f, "persistent"); ok {
		volume.Persistent = persistent
	}
	if size, ok := int64Field(f, "size_bytes"); ok {
		volume.SizeBytes = size
	}
	return volume, nil
}

func lowerIngress(f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) (v1.Ingress, error) {
	var ingress v1.Ingress
	name, err := symbolLabelName(f)
	if err != nil {
		return ingress, fmt.Errorf("ingress: %w", err)
	}
	ingress.Name = name
	if host, ok := stringField(f, "host"); ok {
		ingress.Host = host
	}
	if err := appendIngressRoutes(&ingress, f, workloadEndpoints); err != nil {
		return ingress, err
	}
	return ingress, nil
}

func appendIngressRoutes(ingress *v1.Ingress, f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) error {
	routeForms := childFormsByKind(f, "route")
	for i := range routeForms {
		route, err := lowerRoute(&routeForms[i])
		if err != nil {
			return err
		}
		ingress.Routes = append(ingress.Routes, route)
	}
	return appendIngressPathRoutes(ingress, f, workloadEndpoints)
}

func appendIngressPathRoutes(ingress *v1.Ingress, f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) error {
	pathForms := childFormsByKind(f, "path")
	for i := range pathForms {
		route, err := lowerPathRoute(&pathForms[i], workloadEndpoints)
		if err != nil {
			return err
		}
		ingress.Routes = append(ingress.Routes, route)
	}
	return nil
}

func lowerRoute(f *compiler.HIRForm) (v1.IngressRoute, error) {
	var route v1.IngressRoute
	path, ok := stringField(f, "path")
	if !ok {
		return route, errors.New("route.path is required")
	}
	route.Path = path
	workload, ok := stringField(f, "backend_workload")
	if !ok {
		return route, errors.New("route.backend_workload is required")
	}
	endpoint, ok := stringField(f, "backend_endpoint")
	if !ok {
		return route, errors.New("route.backend_endpoint is required")
	}
	route.Backend = v1.EndpointRef{Workload: workload, Endpoint: endpoint}
	return route, nil
}

func lowerPathRoute(f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) (v1.IngressRoute, error) {
	var route v1.IngressRoute
	path, err := stringLabelName(f)
	if err != nil {
		return route, fmt.Errorf("path route: %w", err)
	}
	route.Path = path
	workload, ok := workloadRefField(f, "workload")
	if !ok {
		return route, fmt.Errorf("path %q: workload is required", path)
	}
	endpoint, ok := stringField(f, "endpoint")
	if !ok || strings.TrimSpace(endpoint) == "" {
		endpoint, err = inferIngressEndpoint(workload, workloadEndpoints[workload])
		if err != nil {
			return route, fmt.Errorf("path %q: %w", path, err)
		}
	}
	route.Backend = v1.EndpointRef{Workload: workload, Endpoint: strings.TrimSpace(endpoint)}
	return route, nil
}

func stringLabelName(f *compiler.HIRForm) (string, error) {
	if f == nil {
		return "", errors.New("form is nil")
	}
	if f.Label != nil && f.Label.Kind == schema.LabelString {
		s := strings.TrimSpace(f.Label.Value)
		if s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("form %q requires a string label", f.Kind)
}

func inferIngressEndpoint(workload string, endpoints []v1.Endpoint) (string, error) {
	var httpEndpoints []v1.Endpoint
	for _, endpoint := range endpoints {
		if endpoint.Protocol == v1.ProtoHTTP {
			httpEndpoints = append(httpEndpoints, endpoint)
		}
	}
	switch len(httpEndpoints) {
	case 1:
		return httpEndpoints[0].Name, nil
	case 0:
		return "", fmt.Errorf("workload %q has no HTTP endpoint; set endpoint explicitly", workload)
	default:
		return "", fmt.Errorf("workload %q has multiple HTTP endpoints; set endpoint explicitly", workload)
	}
}
