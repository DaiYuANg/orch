package orch

import (
	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"
)

func orchActionSpecs() list.List[compiler.ActionSpec] {
	return compiler.ActionSpecs(
		compiler.ActionSpec{
			Name:     "http",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare an HTTP endpoint: http(8080) or http(8080, "admin").`,
		},
		compiler.ActionSpec{
			Name:     "tcp",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare a TCP endpoint: tcp(5432) or tcp(5432, "postgres").`,
		},
		compiler.ActionSpec{
			Name:     "udp",
			MinArgs:  1,
			MaxArgs:  2,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString),
			Docs:     `Declare a UDP endpoint: udp(8125) or udp(8125, "statsd").`,
		},
		compiler.ActionSpec{
			Name:     "port",
			MinArgs:  2,
			MaxArgs:  3,
			ArgTypes: schema.Types(schema.TypeInt, schema.TypeString, schema.TypeString),
			Docs:     `Declare an endpoint with protocol: port(5432, "tcp") or port(5432, "tcp", "postgres").`,
		},
	)
}
