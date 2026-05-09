package metrics

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"metrics",
		dix.Providers(
			dix.Provider1(New, dix.Eager()),
			dix.Provider1(func(s *Service) dix.Observer {
				return s
			}),
		),
	)
}
