package config

import "github.com/arcgolabs/dix"

// Static provides a pre-loaded [Config] (e.g. after Cobra + [LoadFromCobra]).
func Static(cfg Config) dix.Module {
	return dix.NewModule(
		"config",
		dix.Providers(
			dix.Value(cfg),
		),
	)
}
