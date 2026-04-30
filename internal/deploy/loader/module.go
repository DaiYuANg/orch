package loader

import "github.com/arcgolabs/dix"

// Module registers [Loader]. Compose with [orch.Module] first so [*orch.Orch] is available.
func Module() dix.Module {
	return dix.NewModule(
		"deploy-loader",
		dix.Providers(
			dix.ProviderErr1(NewLoader),
		),
	)
}
