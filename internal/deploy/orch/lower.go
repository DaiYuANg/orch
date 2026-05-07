package orch

import (
	"fmt"
	"strings"

	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

// lowerHIR turns compiled orch HIR into the canonical v1alpha1 App. The HIR must contain
// exactly one top-level app form.
func lowerHIR(hir *compiler.HIR) (*v1.App, error) {
	if hir == nil {
		return nil, fmt.Errorf("hir is nil")
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
	metas := childFormsByKind(root, "metadata")
	if len(metas) != 1 {
		return nil, fmt.Errorf("app requires exactly one metadata block, got %d", len(metas))
	}
	md, err := lowerMetadata(&metas[0])
	if err != nil {
		return nil, err
	}
	app.Metadata = md

	for _, wf := range childFormsByKind(root, "workload") {
		w, err := lowerWorkload(&wf)
		if err != nil {
			return nil, err
		}
		app.Workloads = append(app.Workloads, w)
	}
	for _, cf := range childFormsByKind(root, "config") {
		c, err := lowerConfig(&cf)
		if err != nil {
			return nil, err
		}
		app.Configs = append(app.Configs, c)
	}
	for _, sf := range childFormsByKind(root, "secret") {
		s, err := lowerSecret(&sf)
		if err != nil {
			return nil, err
		}
		app.Secrets = append(app.Secrets, s)
	}
	for _, vf := range childFormsByKind(root, "volume") {
		v, err := lowerVolume(&vf)
		if err != nil {
			return nil, err
		}
		app.Volumes = append(app.Volumes, v)
	}
	for _, ig := range childFormsByKind(root, "ingress") {
		in, err := lowerIngress(&ig)
		if err != nil {
			return nil, err
		}
		app.Ingresses = append(app.Ingresses, in)
	}
	return app, nil
}

func childFormsByKind(parent *compiler.HIRForm, kind string) []compiler.HIRForm {
	if parent == nil {
		return nil
	}
	var out []compiler.HIRForm
	for i := range parent.Forms.Len() {
		ch, _ := parent.Forms.Get(i)
		if ch.Kind == kind {
			out = append(out, ch)
		}
	}
	return out
}

func lowerMetadata(f *compiler.HIRForm) (v1.Metadata, error) {
	var md v1.Metadata
	name, ok := stringField(f, "name")
	if !ok || strings.TrimSpace(name) == "" {
		return md, fmt.Errorf("metadata.name is required")
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
		return "", fmt.Errorf("form is nil")
	}
	if f.Label != nil && f.Label.Kind == schema.LabelSymbol {
		s := strings.TrimSpace(f.Label.Value)
		if s != "" {
			return s, nil
		}
	}
	return "", fmt.Errorf("form %q requires a symbol label (name)", f.Kind)
}

func lowerWorkload(f *compiler.HIRForm) (v1.Workload, error) {
	var w v1.Workload
	name, err := symbolLabelName(f)
	if err != nil {
		return w, fmt.Errorf("workload: %w", err)
	}
	w.Name = name

	kindStr, ok := stringField(f, "kind")
	if !ok {
		return w, fmt.Errorf("workload %q: kind is required", name)
	}
	w.Kind = v1.WorkloadKind(strings.ToLower(strings.TrimSpace(kindStr)))

	rtStr, ok := stringField(f, "runtime")
	if !ok {
		return w, fmt.Errorf("workload %q: runtime is required", name)
	}
	w.Runtime = v1.RuntimeKind(strings.ToLower(strings.TrimSpace(rtStr)))

	if fv, ok := intField(f, "replicas"); ok {
		w.Replicas = fv
	}
	if f.Fields != nil {
		if deps, ok := f.Fields.Get("depends_on"); ok {
			w.DependsOn = workloadRefList(deps.Value)
		}
	}

	runs := childFormsByKind(f, "run")
	if len(runs) != 1 {
		return w, fmt.Errorf("workload %q: expected exactly one run block, got %d", name, len(runs))
	}
	if err := fillRun(&w.Run, &runs[0]); err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	if err := fillRuntimeOptions(&w.Run.Options, f); err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}

	for _, ef := range childFormsByKind(f, "endpoint") {
		ep, err := lowerEndpoint(&ef)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Endpoints = append(w.Endpoints, ep)
	}
	for _, mf := range childFormsByKind(f, "mount") {
		m, err := lowerMount(&mf)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Mounts = append(w.Mounts, m)
	}
	for _, envF := range childFormsByKind(f, "env") {
		ev, err := lowerEnv(&envF)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Run.Env = append(w.Run.Env, ev)
	}
	res := childFormsByKind(f, "resources")
	if len(res) > 1 {
		return w, fmt.Errorf("workload %q: at most one resources block", name)
	}
	if len(res) == 1 {
		w.Resources = lowerResources(&res[0])
	}
	sched := childFormsByKind(f, "scheduling")
	if len(sched) > 1 {
		return w, fmt.Errorf("workload %q: at most one scheduling block", name)
	}
	if len(sched) == 1 {
		w.Scheduling = lowerScheduling(&sched[0])
	}
	return w, nil
}

func fillRun(run *v1.RunSpec, f *compiler.HIRForm) error {
	img, ok := stringField(f, "image")
	if !ok || strings.TrimSpace(img) == "" {
		return fmt.Errorf("run.image is required")
	}
	run.Image = strings.TrimSpace(img)
	if f.Fields != nil {
		if cmd, ok := f.Fields.Get("command"); ok {
			run.Command = stringList(cmd.Value)
		}
		if args, ok := f.Fields.Get("args"); ok {
			run.Args = stringList(args.Value)
		}
	}
	if cwd, ok := stringField(f, "cwd"); ok {
		run.Cwd = cwd
	}
	return nil
}

func fillRuntimeOptions(opts *v1.RunOptions, f *compiler.HIRForm) error {
	blocks := childFormsByKind(f, "runtime_options")
	if len(blocks) > 1 {
		return fmt.Errorf("at most one runtime_options block")
	}
	if len(blocks) == 0 {
		return nil
	}
	dockerBlocks := childFormsByKind(&blocks[0], "docker")
	if len(dockerBlocks) > 1 {
		return fmt.Errorf("runtime_options: at most one docker block")
	}
	if len(dockerBlocks) == 1 {
		opts.Docker = lowerDockerOptions(&dockerBlocks[0])
	}
	return nil
}

func lowerDockerOptions(f *compiler.HIRForm) *v1.DockerOptions {
	var d v1.DockerOptions
	if networkMode, ok := stringField(f, "network_mode"); ok {
		d.NetworkMode = strings.TrimSpace(networkMode)
	}
	if privileged, ok := boolField(f, "privileged"); ok {
		d.Privileged = privileged
	}
	if labels, ok := stringMapField(f, "labels"); ok {
		d.Labels = labels
	}
	if d.NetworkMode == "" && !d.Privileged && len(d.Labels) == 0 {
		return nil
	}
	return &d
}

func lowerEndpoint(f *compiler.HIRForm) (v1.Endpoint, error) {
	var e v1.Endpoint
	name, err := symbolLabelName(f)
	if err != nil {
		return e, fmt.Errorf("endpoint: %w", err)
	}
	e.Name = name
	port, ok := intField(f, "port")
	if !ok {
		return e, fmt.Errorf("endpoint %q: port is required", name)
	}
	e.Port = port
	proto, ok := stringField(f, "protocol")
	if !ok {
		return e, fmt.Errorf("endpoint %q: protocol is required", name)
	}
	e.Protocol = v1.EndpointProto(strings.ToLower(strings.TrimSpace(proto)))
	return e, nil
}

func lowerMount(f *compiler.HIRForm) (v1.Mount, error) {
	var m v1.Mount
	vol, ok := stringField(f, "volume")
	if !ok {
		return m, fmt.Errorf("mount.volume is required")
	}
	m.Volume = v1.VolumeRef{Name: vol}
	tgt, ok := stringField(f, "target")
	if !ok {
		return m, fmt.Errorf("mount.target is required")
	}
	m.Target = tgt
	if ro, ok := boolField(f, "read_only"); ok {
		m.ReadOnly = ro
	}
	return m, nil
}

func lowerEnv(f *compiler.HIRForm) (v1.EnvVar, error) {
	var e v1.EnvVar
	n, ok := stringField(f, "name")
	if !ok {
		return e, fmt.Errorf("env.name is required")
	}
	e.Name = n
	val, ok := stringField(f, "value")
	if !ok {
		return e, fmt.Errorf("env.value is required")
	}
	e.Value = val
	return e, nil
}

func lowerResources(f *compiler.HIRForm) *v1.Resources {
	var r v1.Resources
	if cpu, ok := int64Field(f, "cpu_millis"); ok {
		r.CPUMillis = cpu
	}
	if mem, ok := int64Field(f, "memory_bytes"); ok {
		r.MemoryBytes = mem
	}
	if r.CPUMillis == 0 && r.MemoryBytes == 0 {
		return nil
	}
	return &r
}

func lowerScheduling(f *compiler.HIRForm) *v1.Scheduling {
	var s v1.Scheduling
	if stateful, ok := boolField(f, "stateful"); ok {
		s.Stateful = stateful
	}
	if allowLeader, ok := boolField(f, "allow_leader"); ok {
		s.AllowLeader = allowLeader
	}
	if preferredNodes, ok := rawField(f, "preferred_nodes"); ok {
		s.PreferredNodes = stringList(preferredNodes)
	}
	if !s.Stateful && !s.AllowLeader && len(s.PreferredNodes) == 0 {
		return nil
	}
	return &s
}

func lowerConfig(f *compiler.HIRForm) (v1.Config, error) {
	var c v1.Config
	name, err := symbolLabelName(f)
	if err != nil {
		return c, fmt.Errorf("config: %w", err)
	}
	c.Name = name
	data, ok := stringMapField(f, "data")
	if !ok || len(data) == 0 {
		return c, fmt.Errorf("config %q: data map is required", name)
	}
	c.Data = data
	return c, nil
}

func lowerSecret(f *compiler.HIRForm) (v1.Secret, error) {
	var s v1.Secret
	name, err := symbolLabelName(f)
	if err != nil {
		return s, fmt.Errorf("secret: %w", err)
	}
	s.Name = name
	data, ok := stringMapField(f, "data")
	if !ok || len(data) == 0 {
		return s, fmt.Errorf("secret %q: data map is required", name)
	}
	s.Data = data
	return s, nil
}

func lowerVolume(f *compiler.HIRForm) (v1.Volume, error) {
	var v v1.Volume
	name, err := symbolLabelName(f)
	if err != nil {
		return v, fmt.Errorf("volume: %w", err)
	}
	v.Name = name
	if p, ok := boolField(f, "persistent"); ok {
		v.Persistent = p
	}
	if sz, ok := int64Field(f, "size_bytes"); ok {
		v.SizeBytes = sz
	}
	return v, nil
}

func lowerIngress(f *compiler.HIRForm) (v1.Ingress, error) {
	var ing v1.Ingress
	name, err := symbolLabelName(f)
	if err != nil {
		return ing, fmt.Errorf("ingress: %w", err)
	}
	ing.Name = name
	if h, ok := stringField(f, "host"); ok {
		ing.Host = h
	}
	for _, rf := range childFormsByKind(f, "route") {
		rt, err := lowerRoute(&rf)
		if err != nil {
			return ing, err
		}
		ing.Routes = append(ing.Routes, rt)
	}
	return ing, nil
}

func lowerRoute(f *compiler.HIRForm) (v1.IngressRoute, error) {
	var r v1.IngressRoute
	p, ok := stringField(f, "path")
	if !ok {
		return r, fmt.Errorf("route.path is required")
	}
	r.Path = p
	bw, ok := stringField(f, "backend_workload")
	if !ok {
		return r, fmt.Errorf("route.backend_workload is required")
	}
	be, ok := stringField(f, "backend_endpoint")
	if !ok {
		return r, fmt.Errorf("route.backend_endpoint is required")
	}
	r.Backend = v1.EndpointRef{Workload: bw, Endpoint: be}
	return r, nil
}

func stringField(f *compiler.HIRForm, name string) (string, bool) {
	if f == nil || f.Fields == nil {
		return "", false
	}
	fld, ok := f.Fields.Get(name)
	if !ok {
		return "", false
	}
	s, ok := fld.Value.(string)
	return s, ok
}

func boolField(f *compiler.HIRForm, name string) (bool, bool) {
	if f == nil || f.Fields == nil {
		return false, false
	}
	fld, ok := f.Fields.Get(name)
	if !ok {
		return false, false
	}
	b, ok := fld.Value.(bool)
	return b, ok
}

func intField(f *compiler.HIRForm, name string) (int, bool) {
	v, ok := rawField(f, name)
	if !ok {
		return 0, false
	}
	i, ok := intFromAny(v)
	return i, ok
}

func int64Field(f *compiler.HIRForm, name string) (int64, bool) {
	v, ok := rawField(f, name)
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func rawField(f *compiler.HIRForm, name string) (any, bool) {
	if f == nil || f.Fields == nil {
		return nil, false
	}
	fld, ok := f.Fields.Get(name)
	if !ok {
		return nil, false
	}
	return fld.Value, true
}

func stringMapField(f *compiler.HIRForm, name string) (map[string]string, bool) {
	v, ok := rawField(f, name)
	if !ok || v == nil {
		return nil, false
	}
	return mapStringString(v)
}

func mapStringString(v any) (map[string]string, bool) {
	switch m := v.(type) {
	case map[string]string:
		return m, true
	case map[string]any:
		out := make(map[string]string, len(m))
		for k, val := range m {
			switch t := val.(type) {
			case string:
				out[k] = t
			default:
				out[k] = fmt.Sprint(val)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func intFromAny(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case int32:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func stringList(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func workloadRefList(v any) []v1.WorkloadRef {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []v1.WorkloadRef
	for _, it := range items {
		if r, ok := it.(schema.Ref); ok && r.Kind == "workload" {
			out = append(out, v1.WorkloadRef{Name: r.Name})
		}
	}
	return out
}
