package metrics

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"metrics",
		dix.Providers(
			dix.Provider1(New),
		),
	)
}
