package config

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"config",
		dix.Providers(
			dix.ProviderErr0(Load),
		),
	)
}

