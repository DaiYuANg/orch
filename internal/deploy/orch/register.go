package orch

import (
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"
)

// Register attaches the orch deploy DSL form graph to the compiler.
func Register(c *compiler.Compiler) error {
	if c == nil {
		return nil
	}
	return c.RegisterForms(orchFormSpecs())
}

func orchFormSpecs() list.List[schema.FormSpec] {
	return schema.FormSpecs(
		schema.FormSpec{
			Name:        "app",
			LabelKind:   schema.LabelNone,
			BodyMode:    schema.BodyFormOnly,
			Docs:        "Root document: metadata plus workload/config/secret/volume/ingress blocks.",
			NestedForms: schema.NestedForms("metadata", "workload", "config", "secret", "volume", "ingress"),
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
			BodyMode:     schema.BodyMixed,
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
			NestedForms: schema.NestedForms("run", "endpoint", "mount", "env", "resources"),
		},
		schema.FormSpec{
			Name:      "run",
			LabelKind: schema.LabelNone,
			BodyMode:  schema.BodyFieldOnly,
			Fields: schema.Fields(
				schema.FieldSpec{Name: "image", Type: schema.TypeString, Required: true},
				schema.FieldSpec{Name: "command", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
				schema.FieldSpec{Name: "args", Type: schema.ListType{Elem: schema.TypeString}, Default: []any{}, HasDefault: true},
				schema.FieldSpec{Name: "cwd", Type: schema.TypeString},
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
			NestedForms: schema.NestedForms("route"),
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
	)
}
