package orch

import (
	"github.com/arcgolabs/plano/compiler"
)

// NewCompiler returns a plano compiler with orch deploy forms registered.
// Used by [Module] and tests. Application code should resolve [*Orch] from dix and use package deploy/loader for mixed manifest dispatch.
func NewCompiler() (*compiler.Compiler, error) {
	c := compiler.New(compiler.Options{})
	if err := Register(c); err != nil {
		return nil, err
	}
	return c, nil
}
