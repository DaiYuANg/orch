package auth

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"auth",
		dix.Providers(
			dix.ProviderErr1(NewJWTAuthenticationProvider),
			dix.ProviderErr3(NewEngine),
			dix.ProviderErr2(NewGuard),
		),
	)
}
