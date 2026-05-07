package orch

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

type appDefaults struct {
	Runtime v1.RuntimeKind
	Docker  *v1.DockerOptions
}

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
	md, err := lowerAppMetadata(root)
	if err != nil {
		return nil, err
	}
	app.Metadata = md

	defaults, err := lowerAppDefaults(root)
	if err != nil {
		return nil, err
	}

	workloadEndpoints := map[string][]v1.Endpoint{}
	for _, wf := range childFormsByKinds(root, "workload", "service", "stateful", "worker") {
		w, err := lowerWorkload(&wf, defaults)
		if err != nil {
			return nil, err
		}
		app.Workloads = append(app.Workloads, w)
		workloadEndpoints[w.Name] = w.Endpoints
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
		in, err := lowerIngress(&ig, workloadEndpoints)
		if err != nil {
			return nil, err
		}
		app.Ingresses = append(app.Ingresses, in)
	}
	return app, nil
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
		return defaults, fmt.Errorf("app: at most one docker block")
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
	var out []compiler.HIRForm
	for i := range parent.Forms.Len() {
		ch, _ := parent.Forms.Get(i)
		if ch.Kind == kind {
			out = append(out, ch)
		}
	}
	return out
}

func childFormsByKinds(parent *compiler.HIRForm, kinds ...string) []compiler.HIRForm {
	if parent == nil {
		return nil
	}
	allowed := make(map[string]struct{}, len(kinds))
	for _, kind := range kinds {
		allowed[kind] = struct{}{}
	}
	var out []compiler.HIRForm
	for i := range parent.Forms.Len() {
		ch, _ := parent.Forms.Get(i)
		if _, ok := allowed[ch.Kind]; ok {
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

func lowerWorkload(f *compiler.HIRForm, defaults appDefaults) (v1.Workload, error) {
	var w v1.Workload
	name, err := symbolLabelName(f)
	if err != nil {
		return w, fmt.Errorf("workload: %w", err)
	}
	w.Name = name

	kind, err := workloadKind(f)
	if err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	w.Kind = kind

	w.Runtime = workloadRuntime(f, defaults)

	if fv, ok := intField(f, "replicas"); ok {
		w.Replicas = fv
	}
	if f.Fields != nil {
		if deps, ok := f.Fields.Get("depends_on"); ok {
			w.DependsOn = workloadRefList(deps.Value)
		}
	}

	runs := childFormsByKind(f, "run")
	if len(runs) > 1 {
		return w, fmt.Errorf("workload %q: at most one run block", name)
	}
	if len(runs) == 1 {
		if err := fillRun(&w.Run, &runs[0]); err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
	}
	if err := fillRunFromFields(&w.Run, f); err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	if strings.TrimSpace(w.Run.Image) == "" {
		return w, fmt.Errorf("workload %q: image is required", name)
	}
	if err := fillRuntimeOptions(&w.Run.Options, f); err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	if err := fillDockerOptionsFromFields(&w.Run.Options, f); err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	w.Run.Options.Docker = mergeDockerOptionsForRuntime(w.Runtime, defaults.Docker, w.Run.Options.Docker)

	for _, ef := range childFormsByKind(f, "endpoint") {
		ep, err := lowerEndpoint(&ef)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Endpoints = append(w.Endpoints, ep)
	}
	for _, ep := range lowerEndpointCalls(f) {
		w.Endpoints = append(w.Endpoints, ep)
	}
	for _, mf := range childFormsByKind(f, "mount") {
		m, err := lowerMount(&mf)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Mounts = append(w.Mounts, m)
	}
	if envMap, ok := stringMapField(f, "env"); ok {
		w.Run.Env = append(w.Run.Env, envVarsFromMap(envMap)...)
	}
	for _, envF := range childFormsByKind(f, "env") {
		ev, err := lowerEnv(&envF)
		if err != nil {
			return w, fmt.Errorf("workload %q: %w", name, err)
		}
		w.Run.Env = append(w.Run.Env, ev)
	}
	resources, err := lowerWorkloadResources(f)
	if err != nil {
		return w, fmt.Errorf("workload %q: %w", name, err)
	}
	w.Resources = resources

	sched := childFormsByKind(f, "scheduling")
	if len(sched) > 1 {
		return w, fmt.Errorf("workload %q: at most one scheduling block", name)
	}
	w.Scheduling = lowerSchedulingFromFields(f, w.Kind == v1.WorkloadKindStateful)
	if len(sched) == 1 {
		w.Scheduling = mergeScheduling(w.Scheduling, lowerScheduling(&sched[0]))
	}
	return w, nil
}

func workloadKind(f *compiler.HIRForm) (v1.WorkloadKind, error) {
	switch f.Kind {
	case "service":
		return v1.WorkloadKindService, nil
	case "stateful":
		return v1.WorkloadKindStateful, nil
	case "worker":
		return v1.WorkloadKindWorker, nil
	}
	kindStr, ok := stringField(f, "kind")
	if !ok || strings.TrimSpace(kindStr) == "" {
		return "", fmt.Errorf("kind is required")
	}
	return v1.WorkloadKind(strings.ToLower(strings.TrimSpace(kindStr))), nil
}

func workloadRuntime(f *compiler.HIRForm, defaults appDefaults) v1.RuntimeKind {
	if rtStr, ok := stringField(f, "runtime"); ok && strings.TrimSpace(rtStr) != "" {
		return v1.RuntimeKind(strings.ToLower(strings.TrimSpace(rtStr)))
	}
	if defaults.Runtime != "" {
		return defaults.Runtime
	}
	return v1.RuntimeDocker
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

func fillRunFromFields(run *v1.RunSpec, f *compiler.HIRForm) error {
	if img, ok := stringField(f, "image"); ok && strings.TrimSpace(img) != "" {
		if run.Image != "" && run.Image != strings.TrimSpace(img) {
			return fmt.Errorf("image is set both in run block and workload field")
		}
		run.Image = strings.TrimSpace(img)
	}
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

func fillDockerOptionsFromFields(opts *v1.RunOptions, f *compiler.HIRForm) error {
	d := lowerDockerOptions(f)
	dockerBlocks := childFormsByKind(f, "docker")
	if len(dockerBlocks) > 1 {
		return fmt.Errorf("at most one docker block")
	}
	if len(dockerBlocks) == 1 {
		d = mergeDockerOptions(d, lowerDockerOptions(&dockerBlocks[0]))
	}
	opts.Docker = mergeDockerOptions(opts.Docker, d)
	return nil
}

func lowerDockerOptions(f *compiler.HIRForm) *v1.DockerOptions {
	var d v1.DockerOptions
	if network, ok := stringField(f, "network"); ok {
		d.NetworkMode = strings.TrimSpace(network)
	}
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

func mergeDockerOptionsForRuntime(runtime v1.RuntimeKind, base, override *v1.DockerOptions) *v1.DockerOptions {
	if runtime != v1.RuntimeDocker {
		return override
	}
	return mergeDockerOptions(base, override)
}

func mergeDockerOptions(base, override *v1.DockerOptions) *v1.DockerOptions {
	if base == nil && override == nil {
		return nil
	}
	var out v1.DockerOptions
	if base != nil {
		out = *base
		if base.Labels != nil {
			out.Labels = cloneStringMap(base.Labels)
		}
	}
	if override != nil {
		if override.NetworkMode != "" {
			out.NetworkMode = override.NetworkMode
		}
		if override.Privileged {
			out.Privileged = true
		}
		if len(override.Labels) > 0 {
			if out.Labels == nil {
				out.Labels = map[string]string{}
			}
			for k, v := range override.Labels {
				out.Labels[k] = v
			}
		}
	}
	if out.NetworkMode == "" && !out.Privileged && len(out.Labels) == 0 {
		return nil
	}
	return &out
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

func lowerEndpointCalls(f *compiler.HIRForm) []v1.Endpoint {
	if f == nil {
		return nil
	}
	var out []v1.Endpoint
	for i := range f.Calls.Len() {
		call, _ := f.Calls.Get(i)
		ep, ok := endpointFromCall(call)
		if ok {
			out = append(out, ep)
		}
	}
	return out
}

func endpointFromCall(call compiler.HIRCall) (v1.Endpoint, bool) {
	switch call.Name {
	case "http":
		return endpointFromProtocolCall(call, v1.ProtoHTTP, "http")
	case "tcp":
		return endpointFromProtocolCall(call, v1.ProtoTCP, "")
	case "udp":
		return endpointFromProtocolCall(call, v1.ProtoUDP, "")
	case "port":
		return endpointFromPortCall(call)
	default:
		return v1.Endpoint{}, false
	}
}

func endpointFromProtocolCall(call compiler.HIRCall, proto v1.EndpointProto, defaultName string) (v1.Endpoint, bool) {
	port, ok := callIntArg(call, 0)
	if !ok {
		return v1.Endpoint{}, false
	}
	name := strings.TrimSpace(defaultName)
	if name == "" {
		name = fmt.Sprintf("%s-%d", proto, port)
	}
	if custom, ok := callStringArg(call, 1); ok && strings.TrimSpace(custom) != "" {
		name = strings.TrimSpace(custom)
	}
	return v1.Endpoint{Name: name, Port: port, Protocol: proto}, true
}

func endpointFromPortCall(call compiler.HIRCall) (v1.Endpoint, bool) {
	port, ok := callIntArg(call, 0)
	if !ok {
		return v1.Endpoint{}, false
	}
	protoStr, ok := callStringArg(call, 1)
	if !ok || strings.TrimSpace(protoStr) == "" {
		return v1.Endpoint{}, false
	}
	proto := v1.EndpointProto(strings.ToLower(strings.TrimSpace(protoStr)))
	name := fmt.Sprintf("%s-%d", proto, port)
	if custom, ok := callStringArg(call, 2); ok && strings.TrimSpace(custom) != "" {
		name = strings.TrimSpace(custom)
	}
	return v1.Endpoint{Name: name, Port: port, Protocol: proto}, true
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

func lowerWorkloadResources(f *compiler.HIRForm) (*v1.Resources, error) {
	var out *v1.Resources
	if spec, ok := stringField(f, "resources"); ok && strings.TrimSpace(spec) != "" {
		res, err := parseResourceSpec(spec)
		if err != nil {
			return nil, err
		}
		out = res
	}
	if cpu, ok := int64Field(f, "cpu_millis"); ok {
		if out == nil {
			out = &v1.Resources{}
		}
		out.CPUMillis = cpu
	}
	if mem, ok := int64Field(f, "memory_bytes"); ok {
		if out == nil {
			out = &v1.Resources{}
		}
		out.MemoryBytes = mem
	}
	blocks := childFormsByKind(f, "resources")
	if len(blocks) > 1 {
		return nil, fmt.Errorf("at most one resources block")
	}
	if len(blocks) == 1 {
		if out != nil {
			return nil, fmt.Errorf("resources are set both as fields and resources block")
		}
		out = lowerResources(&blocks[0])
	}
	return out, nil
}

func parseResourceSpec(raw string) (*v1.Resources, error) {
	parts := strings.Split(strings.TrimSpace(raw), "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf(`resources must be "cpu/memory", got %q`, raw)
	}
	cpu, err := parseCPUMillis(parts[0])
	if err != nil {
		return nil, err
	}
	mem, err := schema.ParseSize(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("parse memory resource: %w", err)
	}
	return &v1.Resources{CPUMillis: cpu, MemoryBytes: mem.Bytes}, nil
}

func parseCPUMillis(raw string) (int64, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("cpu resource is empty")
	}
	if strings.HasSuffix(s, "m") {
		n, err := strconv.ParseInt(strings.TrimSuffix(s, "m"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse cpu resource %q: %w", raw, err)
		}
		return n, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse cpu resource %q: %w", raw, err)
	}
	return int64(f * 1000), nil
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

func lowerSchedulingFromFields(f *compiler.HIRForm, statefulDefault bool) *v1.Scheduling {
	var s v1.Scheduling
	s.Stateful = statefulDefault
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

func mergeScheduling(base, override *v1.Scheduling) *v1.Scheduling {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}
	out := *base
	if override.Stateful {
		out.Stateful = true
	}
	if override.AllowLeader {
		out.AllowLeader = true
	}
	if len(override.PreferredNodes) > 0 {
		out.PreferredNodes = append([]string(nil), override.PreferredNodes...)
	}
	if !out.Stateful && !out.AllowLeader && len(out.PreferredNodes) == 0 {
		return nil
	}
	return &out
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

func lowerIngress(f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) (v1.Ingress, error) {
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
	for _, pf := range childFormsByKind(f, "path") {
		rt, err := lowerPathRoute(&pf, workloadEndpoints)
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

func lowerPathRoute(f *compiler.HIRForm, workloadEndpoints map[string][]v1.Endpoint) (v1.IngressRoute, error) {
	var r v1.IngressRoute
	path, err := stringLabelName(f)
	if err != nil {
		return r, fmt.Errorf("path route: %w", err)
	}
	r.Path = path
	workload, ok := workloadRefField(f, "workload")
	if !ok {
		return r, fmt.Errorf("path %q: workload is required", path)
	}
	endpoint, ok := stringField(f, "endpoint")
	if !ok || strings.TrimSpace(endpoint) == "" {
		endpoint, err = inferIngressEndpoint(workload, workloadEndpoints[workload])
		if err != nil {
			return r, fmt.Errorf("path %q: %w", path, err)
		}
	}
	r.Backend = v1.EndpointRef{Workload: workload, Endpoint: strings.TrimSpace(endpoint)}
	return r, nil
}

func stringLabelName(f *compiler.HIRForm) (string, error) {
	if f == nil {
		return "", fmt.Errorf("form is nil")
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
	for _, ep := range endpoints {
		if ep.Protocol == v1.ProtoHTTP {
			httpEndpoints = append(httpEndpoints, ep)
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

func workloadRefField(f *compiler.HIRForm, name string) (string, bool) {
	v, ok := rawField(f, name)
	if !ok {
		return "", false
	}
	switch ref := v.(type) {
	case schema.Ref:
		if ref.Kind == "workload" && strings.TrimSpace(ref.Name) != "" {
			return strings.TrimSpace(ref.Name), true
		}
	case string:
		if strings.TrimSpace(ref) != "" {
			return strings.TrimSpace(ref), true
		}
	}
	return "", false
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
	case *mapping.OrderedMap[string, any]:
		out := make(map[string]string, m.Len())
		m.Range(func(k string, val any) bool {
			switch t := val.(type) {
			case string:
				out[k] = t
			default:
				out[k] = fmt.Sprint(val)
			}
			return true
		})
		return out, true
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

func envVarsFromMap(m map[string]string) []v1.EnvVar {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]v1.EnvVar, 0, len(keys))
	for _, k := range keys {
		out = append(out, v1.EnvVar{Name: k, Value: m[k]})
	}
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
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

func callIntArg(call compiler.HIRCall, idx int) (int, bool) {
	arg, ok := call.Args.Get(idx)
	if !ok {
		return 0, false
	}
	return intFromAny(arg.Value)
}

func callStringArg(call compiler.HIRCall, idx int) (string, bool) {
	arg, ok := call.Args.Get(idx)
	if !ok {
		return "", false
	}
	s, ok := arg.Value.(string)
	return s, ok
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
