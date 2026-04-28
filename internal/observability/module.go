package observability

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"observability",
		dix.Providers(
			dix.Provider1(New),
		),
	)
}
