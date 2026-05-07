package orch

import (
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/collectionx/set"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"
)

// Register attaches the orch deploy DSL form graph to the compiler.
func Register(c *compiler.Compiler) error {
	if c == nil {
		return nil
	}
	if err := c.RegisterForms(orchFormSpecs()); err != nil {
		return err
	}
	return c.RegisterActions(orchActionSpecs())
}

func orchFormSpecs() list.List[schema.FormSpec] {
	return schema.FormSpecs(
		schema.FormSpec{
			Name:      "app",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyMixed,
			Docs:      "Root document: metadata plus workload/config/secret/volume/ingress blocks. Short form accepts name/namespace fields directly.",
			Fields: schema.Fields(
				schema.FieldSpec{Name: "name", Type: schema.TypeString, Docs: "App name. Short for metadata.name."},
				schema.FieldSpec{Name: "namespace", Type: schema.TypeString, Docs: "App namespace. Short for metadata.namespace."},
				schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
				schema.FieldSpec{Name: "annotations", Type: schema.MapType{Elem: schema.TypeString}},
				schema.FieldSpec{Name: "runtime", Type: schema.TypeString, Default: "docker", HasDefault: true, Docs: "Default runtime for shorthand workloads."},
			),
			NestedForms: schema.NestedForms("metadata", "docker", "workload", "service", "stateful", "worker", "config", "secret", "volume", "ingress"),
		},
		schema.FormSpec{
			Name:      "metadata",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "name", Type: schema.TypeString, Required: true, Docs: "App name (required)."},
				schema.FieldSpec{Name: "namespace", Type: schema.TypeString},
				schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
				schema.FieldSpec{Name: "annotations", Type: schema.MapType{Elem: schema.TypeString}},
			),
		},
		schema.FormSpec{
			Name:         "workload",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "workload",
			Declares:     "workload",
			BodyMode:     schema.BodyScript,
			Docs:         "Named workload (label is the workload name).",
			Fields: schema.Fields(
				schema.FieldSpec{Name: "kind", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "runtime", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "replicas", Type: schema.TypeInt, Default: 0, HasDefault: true},
				schema.FieldSpec{
					Name:       "depends_on",
					Type:       schema.ListType{Elem: schema.RefType{Kind: "workload"}},
					Default:    []any{},
					HasDefault: true,
					Docs:       "Depends-on edges to other workloads.",
				},
			),
			NestedForms: workloadNestedForms(),
		},
		shorthandWorkloadForm("service", "Service workload. Short for workload kind=service."),
		shorthandWorkloadForm("stateful", "Stateful workload. Short for workload kind=stateful and scheduling.stateful=true."),
		shorthandWorkloadForm("worker", "Worker workload. Short for workload kind=worker."),
		schema.FormSpec{
			Name:      "run",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "image", Type: schema.TypeString},
				schema.FieldSpec{Name: "path", Type: schema.TypeString},
				schema.FieldSpec{Name: "url", Type: schema.TypeString},
				schema.FieldSpec{Name: "command", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
				schema.FieldSpec{Name: "args", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
				schema.FieldSpec{Name: "cwd", Type: schema.TypeString},
				schema.FieldSpec{Name: "user", Type: schema.TypeString},
			),
		},
		schema.FormSpec{
			Name:         "endpoint",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "endpoint",
			Declares:     "endpoint",
			BodyMode:     schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "port", Type: schema.TypeInt, Required: true},
				schema.FieldSpec{Name: "protocol", Type: schema.TypeString, Required: true},
			),
		},
		schema.FormSpec{
			Name:      "mount",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "volume", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "target", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "read_only", Type: schema.TypeBool, Default: false, HasDefault: true},
			),
		},
		schema.FormSpec{
			Name:      "env",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "name", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "value", Type: schema.TypeString, Required: true},
			),
		},
		schema.FormSpec{
			Name:      "resources",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "cpu_millis", Type: schema.TypeInt},
				schema.FieldSpec{Name: "memory_bytes", Type: schema.TypeInt},
			),
		},
		schema.FormSpec{
			Name:        "runtime_options",
			LabelKind:   schema.LabelNone,
			BodyMode:    schema.BodyFormOnly,
			NestedForms: schema.NestedForms("docker", "containerd", "firecracker", "process", "systemd", "windows_service"),
		},
		schema.FormSpec{
			Name:      "docker",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "network", Type: schema.TypeString, Docs: "Alias for network_mode."},
				schema.FieldSpec{Name: "network_mode", Type: schema.TypeString},
				schema.FieldSpec{Name: "privileged", Type: schema.TypeBool, Default: false, HasDefault: true},
				schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
			),
		},
		schema.FormSpec{
			Name:      "containerd",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "namespace", Type: schema.TypeString},
			),
		},
		schema.FormSpec{
			Name:      "firecracker",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "kernel_image_path", Type: schema.TypeString},
				schema.FieldSpec{Name: "rootfs_path", Type: schema.TypeString},
				schema.FieldSpec{Name: "vcpu_count", Type: schema.TypeInt},
				schema.FieldSpec{Name: "mem_size_mib", Type: schema.TypeInt},
			),
		},
		schema.FormSpec{
			Name:      "process",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "graceful_stop_timeout", Type: schema.TypeString},
				schema.FieldSpec{Name: "stdout_path", Type: schema.TypeString},
				schema.FieldSpec{Name: "stderr_path", Type: schema.TypeString},
			),
		},
		schema.FormSpec{
			Name:      "systemd",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "unit_name", Type: schema.TypeString},
				schema.FieldSpec{Name: "user", Type: schema.TypeString},
				schema.FieldSpec{Name: "group", Type: schema.TypeString},
				schema.FieldSpec{Name: "restart", Type: schema.TypeString},
				schema.FieldSpec{Name: "restart_sec", Type: schema.TypeString},
				schema.FieldSpec{Name: "wanted_by", Type: schema.TypeString},
			),
		},
		schema.FormSpec{
			Name:      "windows_service",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "service_name", Type: schema.TypeString},
				schema.FieldSpec{Name: "display_name", Type: schema.TypeString},
				schema.FieldSpec{Name: "start_type", Type: schema.TypeString},
			),
		},
		schema.FormSpec{
			Name:      "scheduling",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "stateful", Type: schema.TypeBool, Default: false, HasDefault: true},
				schema.FieldSpec{Name: "allow_leader", Type: schema.TypeBool, Default: false, HasDefault: true},
				schema.FieldSpec{Name: "preferred_nodes", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
			),
		},
		schema.FormSpec{
			Name:         "config",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "config",
			Declares:     "config",
			BodyMode:     schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "data", Type: schema.MapType{Elem: schema.TypeString}, Required: true},
			),
		},
		schema.FormSpec{
			Name:         "secret",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "secret",
			Declares:     "secret",
			BodyMode:     schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "data", Type: schema.MapType{Elem: schema.TypeString}, Required: true},
			),
		},
		schema.FormSpec{
			Name:         "volume",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "volume",
			Declares:     "volume",
			BodyMode:     schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "persistent", Type: schema.TypeBool, Default: false, HasDefault: true},
				schema.FieldSpec{Name: "size_bytes", Type: schema.TypeInt},
			),
		},
		schema.FormSpec{
			Name:         "ingress",
			LabelKind:    schema.LabelSymbol,
			LabelRefKind: "ingress",
			Declares:     "ingress",
			BodyMode:     schema.BodyMixed,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "host", Type: schema.TypeString},
			),
			NestedForms: schema.NestedForms("route", "path"),
		},
		schema.FormSpec{
			Name:      "route",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "path", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "backend_workload", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "backend_endpoint", Type: schema.TypeString, Required: true},
			),
		},
		schema.FormSpec{
			Name:      "path",
			LabelKind: schema.LabelString,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "workload", Type: schema.RefType{Kind: "workload"}, Required: true},
				schema.FieldSpec{Name: "endpoint", Type: schema.TypeString},
			),
		},
	)
}

func shorthandWorkloadForm(name string, docs string) schema.FormSpec {
	return schema.FormSpec{
		Name:         name,
		LabelKind:    schema.LabelSymbol,
		LabelRefKind: "workload",
		Declares:     "workload",
		BodyMode:     schema.BodyScript,
		Docs:         docs,
		Fields:       shorthandWorkloadFields(),
		NestedForms:  workloadNestedForms(),
	}
}

func shorthandWorkloadFields() *mapping.OrderedMap[string, schema.FieldSpec] {
	return schema.Fields(
		schema.FieldSpec{Name: "runtime", Type: schema.TypeString},
		schema.FieldSpec{Name: "image", Type: schema.TypeString},
		schema.FieldSpec{Name: "path", Type: schema.TypeString},
		schema.FieldSpec{Name: "url", Type: schema.TypeString},
		schema.FieldSpec{Name: "command", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
		schema.FieldSpec{Name: "args", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
		schema.FieldSpec{Name: "cwd", Type: schema.TypeString},
		schema.FieldSpec{Name: "user", Type: schema.TypeString},
		schema.FieldSpec{Name: "env", Type: schema.MapType{Elem: schema.TypeString}},
		schema.FieldSpec{Name: "resources", Type: schema.TypeString, Docs: `Compact "cpu/memory", e.g. "500m/512Mi".`},
		schema.FieldSpec{Name: "cpu_millis", Type: schema.TypeInt},
		schema.FieldSpec{Name: "memory_bytes", Type: schema.TypeInt},
		schema.FieldSpec{Name: "replicas", Type: schema.TypeInt, Default: 0, HasDefault: true},
		schema.FieldSpec{
			Name:       "depends_on",
			Type:       schema.ListType{Elem: schema.RefType{Kind: "workload"}},
			Default:    []any{},
			HasDefault: true,
			Docs:       "Depends-on edges to other workloads.",
		},
		schema.FieldSpec{Name: "network", Type: schema.TypeString, Docs: "Docker network_mode shorthand."},
		schema.FieldSpec{Name: "network_mode", Type: schema.TypeString},
		schema.FieldSpec{Name: "privileged", Type: schema.TypeBool, Default: false, HasDefault: true},
		schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
		schema.FieldSpec{Name: "allow_leader", Type: schema.TypeBool, Default: false, HasDefault: true},
		schema.FieldSpec{Name: "preferred_nodes", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
	)
}

func workloadNestedForms() *set.Set[string] {
	return schema.NestedForms("run", "runtime_options", "docker", "containerd", "firecracker", "process", "systemd", "windows_service", "endpoint", "mount", "env", "resources", "scheduling")
}

func orchActionSpecs() list.List[compiler.ActionSpec] {
	return compiler.ActionSpecs(
		compiler.ActionSpec{
			Name:     "http",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare an HTTP endpoint: http(8080) or http(8080, "admin").`,
		},
		compiler.ActionSpec{
			Name:     "tcp",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare a TCP endpoint: tcp(5432) or tcp(5432, "postgres").`,
		},
		compiler.ActionSpec{
			Name:     "udp",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare a UDP endpoint: udp(8125) or udp(8125, "statsd").`,
		},
		compiler.ActionSpec{
			Name:     "port",
			MinArgs:  2,
			MaxArgs:  3,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString, schema.TypeString),
			Docs:     `Declare an endpoint with protocol: port(5432, "tcp") or port(5432, "tcp", "postgres").`,
		},
	)
}
