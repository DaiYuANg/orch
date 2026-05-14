package orch

import "github.com/arcgolabs/plano/schema"

func objectFormSpecs() []schema.FormSpec {
	return []schema.FormSpec{
		configFormSpec(),
		secretFormSpec(),
		volumeFormSpec(),
		ingressFormSpec(),
		routeFormSpec(),
		pathFormSpec(),
	}
}

func configFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:         "config",
		LabelKind:    schema.LabelSymbol,
		LabelRefKind: "config",
		Declares:     "config",
		BodyMode:     schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "data", Type: schema.MapType{Elem: schema.TypeString}, Required: true},
		),
	}
}

func secretFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:         "secret",
		LabelKind:    schema.LabelSymbol,
		LabelRefKind: "secret",
		Declares:     "secret",
		BodyMode:     schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "data", Type: schema.MapType{Elem: schema.TypeString}, Required: true},
		),
	}
}

func volumeFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:         "volume",
		LabelKind:    schema.LabelSymbol,
		LabelRefKind: "volume",
		Declares:     "volume",
		BodyMode:     schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "persistent", Type: schema.TypeBool, Default: false, HasDefault: true},
			schema.FieldSpec{Name: "size_bytes", Type: schema.TypeInt},
		),
	}
}

func ingressFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:         "ingress",
		LabelKind:    schema.LabelSymbol,
		LabelRefKind: "ingress",
		Declares:     "ingress",
		BodyMode:     schema.BodyMixed,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "host", Type: schema.TypeString},
		),
		NestedForms: schema.NestedForms("route", "path"),
	}
}

func routeFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "route",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "path", Type: schema.TypeString, Required: true},
			schema.FieldSpec{Name: "backend_workload", Type: schema.TypeString, Required: true},
			schema.FieldSpec{Name: "backend_endpoint", Type: schema.TypeString, Required: true},
		),
	}
}

func pathFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "path",
		LabelKind: schema.LabelString,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "workload", Type: schema.RefType{Kind: "workload"}, Required: true},
			schema.FieldSpec{Name: "endpoint", Type: schema.TypeString},
		),
	}
}
