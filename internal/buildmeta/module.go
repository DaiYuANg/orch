package buildmeta

import "github.com/arcgolabs/dix"

func Module() dix.Module {
	return dix.NewModule(
		"buildmeta",
		dix.Providers(
			dix.Value(dix.AppMeta{Version: Version()}),
		),
	)
}
