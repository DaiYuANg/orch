package orch

import "github.com/arcgolabs/dix"

// Module registers the plano compiler and [.orch] [*Orch] singleton.
func Module() dix.Module {
	return dix.NewModule(
		"deploy-orch",
		dix.Providers(
			dix.Provider0(NewHIRMapper),
			dix.ProviderErr0(NewCompiler),
			dix.ProviderErr2(NewOrchWithMapper),
		),
	)
}
