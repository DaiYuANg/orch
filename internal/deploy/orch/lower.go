package orch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

type appDefaults struct {
	Runtime v1.RuntimeKind
	Docker  *v1.DockerOptions
}

// lowerHIR turns compiled orch HIR into the canonical v1alpha1 App. The HIR must contain
// exactly one top-level app form.
func lowerHIR(hir *compiler.HIR) (*v1.App, error) {
	if hir == nil {
		return nil, errors.New("hir is nil")
	}
	var roots []compiler.HIRForm
	for i := range hir.Forms.Len() {
		f, _ := hir.Forms.Get(i)
		if f.Kind == "app" {
			roots = append(roots, f)
		}
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("expected exactly one top-level app form, got %d", len(roots))
	}
	return lowerApp(&roots[0])
}

func lowerApp(root *compiler.HIRForm) (*v1.App, error) {
	app := &v1.App{}
	metadata, defaults, err := lowerAppHeader(root)
	if err != nil {
		return nil, err
	}
	app.Metadata = metadata

	workloadEndpoints, err := lowerAppWorkloads(app, root, defaults)
	if err != nil {
		return nil, err
	}
	if err := lowerAppResources(app, root); err != nil {
		return nil, err
	}
	if err := lowerAppIngresses(app, root, workloadEndpoints); err != nil {
		return nil, err
	}
	return app, nil
}

func lowerAppHeader(root *compiler.HIRForm) (v1.Metadata, appDefaults, error) {
	metadata, err := lowerAppMetadata(root)
	if err != nil {
		return v1.Metadata{}, appDefaults{}, err
	}
	defaults, err := lowerAppDefaults(root)
	if err != nil {
		return v1.Metadata{}, appDefaults{}, err
	}
	return metadata, defaults, nil
}

func lowerAppWorkloads(app *v1.App, root *compiler.HIRForm, defaults appDefaults) (map[string][]v1.Endpoint, error) {
	workloadEndpoints := map[string][]v1.Endpoint{}
	workloadForms := childFormsByKinds(root, "workload", "service", "stateful", "worker")
	for i := range workloadForms {
		workload, err := lowerWorkload(&workloadForms[i], defaults)
		if err != nil {
			return nil, err
		}
		app.Workloads = append(app.Workloads, workload)
		workloadEndpoints[workload.Name] = workload.Endpoints
	}
	return workloadEndpoints, nil
}

func lowerAppResources(app *v1.App, root *compiler.HIRForm) error {
	if err := lowerAppConfigs(app, root); err != nil {
		return err
	}
	if err := lowerAppSecrets(app, root); err != nil {
		return err
	}
	return lowerAppVolumes(app, root)
}

func lowerAppConfigs(app *v1.App, root *compiler.HIRForm) error {
	configForms := childFormsByKind(root, "config")
	for i := range configForms {
		config, err := lowerConfig(&configForms[i])
		if err != nil {
			return err
		}
		app.Configs = append(app.Configs, config)
	}
	return nil
}

func lowerAppSecrets(app *v1.App, root *compiler.HIRForm) error {
	secretForms := childFormsByKind(root, "secret")
	for i := range secretForms {
		secret, err := lowerSecret(&secretForms[i])
		if err != nil {
			return err
		}
		app.Secrets = append(app.Secrets, secret)
	}
	return nil
}

func lowerAppVolumes(app *v1.App, root *compiler.HIRForm) error {
	volumeForms := childFormsByKind(root, "volume")
	for i := range volumeForms {
		volume, err := lowerVolume(&volumeForms[i])
		if err != nil {
			return err
		}
		app.Volumes = append(app.Volumes, volume)
	}
	return nil
}

func lowerAppIngresses(app *v1.App, root *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) error {
	ingressForms := childFormsByKind(root, "ingress")
	for i := range ingressForms {
		ingress, err := lowerIngress(&ingressForms[i], workloadEndpoints)
		if err != nil {
			return err
		}
		app.Ingresses = append(app.Ingresses, ingress)
	}
	return nil
}

func lowerAppMetadata(root *compiler.HIRForm) (v1.Metadata, error) {
	metas := childFormsByKind(root, "metadata")
	if len(metas) > 1 {
		return v1.Metadata{}, fmt.Errorf("app requires at most one metadata block, got %d", len(metas))
	}
	if len(metas) == 1 {
		return lowerMetadata(&metas[0])
	}
	return lowerMetadata(root)
}

func lowerAppDefaults(root *compiler.HIRForm) (appDefaults, error) {
	defaults := appDefaults{Runtime: v1.RuntimeDocker}
	if rt, ok := stringField(root, "runtime"); ok && strings.TrimSpace(rt) != "" {
		defaults.Runtime = v1.RuntimeKind(strings.ToLower(strings.TrimSpace(rt)))
	}
	blocks := childFormsByKind(root, "docker")
	if len(blocks) > 1 {
		return defaults, errors.New("app: at most one docker block")
	}
	if len(blocks) == 1 {
		defaults.Docker = lowerDockerOptions(&blocks[0])
	}
	return defaults, nil
}

func childFormsByKind(parent *compiler.HIRForm, kind string) []compiler.HIRForm {
	if parent == nil {
		return nil
	}
	out := list.NewList[compiler.HIRForm]()
	for i := range parent.Forms.Len() {
		ch, _ := parent.Forms.Get(i)
		if ch.Kind == kind {
			out.Add(ch)
		}
	}
	return out.Values()
}

func childFormsByKinds(parent *compiler.HIRForm, kinds ...string) []compiler.HIRForm {
	if parent == nil {
		return nil
	}
	allowed := set.NewSetWithCapacity[string](len(kinds), kinds...)
	out := list.NewList[compiler.HIRForm]()
	for i := range parent.Forms.Len() {
		ch, _ := parent.Forms.Get(i)
		if allowed.Contains(ch.Kind) {
			out.Add(ch)
		}
	}
	return out.Values()
}

func lowerMetadata(f *compiler.HIRForm) (v1.Metadata, error) {
	var md v1.Metadata
	name, ok := stringField(f, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return md, errors.New("metadata.name is required")
	}
	md.Name = strings.TrimSpace(name)
	if ns, ok := stringField(f, "namespace"); ok {
		md.Namespace = strings.TrimSpace(ns)
	}
	if m, ok := stringMapField(f, "labels"); ok {
		md.Labels = m
	}
	if m, ok := stringMapField(f, "annotations"); ok {
		md.Annotations = m
	}
	return md, nil
}

func symbolLabelName(f *compiler.HIRForm) (string, error) {
	if f == nil {
		return "", errors.New("form is nil")
	}
	if f.Label != nil && f.Label.Kind == schema.LabelSymbol {
		s := strings.TrimSpace(f.Label.Value)
		if s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("form %q requires a symbol label (name)", f.Kind)
}
