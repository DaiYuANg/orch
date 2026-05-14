package orch

import "github.com/arcgolabs/plano/schema"

func appFormSpecs() []schema.FormSpec {
	return []schema.FormSpec{
		{
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
		{
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
	}
}
