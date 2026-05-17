package orch

import (
	"context"
	"fmt"
	"strings"

	"github.com/arcgolabs/mapper"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/diag"
	"go/token"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// Orch compiles .orch sources with a shared [compiler.Compiler] and lowers HIR to [v1.App].
// Construct via [NewOrch] from dix ([Module]) so the compiler is a process singleton.
type Orch struct {
	c      *compiler.Compiler
	mapper *mapper.Mapper
}

// NewOrch wraps a non-nil plano compiler with orch forms registered ([NewCompiler] / [Module]).
func NewOrch(c *compiler.Compiler) (*Orch, error) {
	return NewOrchWithMapper(c, NewHIRMapper())
}

// NewOrchWithMapper wraps compiler and HIR mapper dependencies for dix composition.
func NewOrchWithMapper(c *compiler.Compiler, m *mapper.Mapper) (*Orch, error) {
	if c == nil {
		return nil, oopsx.B("deploy", "orch").Errorf("nil compiler")
	}
	if m == nil {
		return nil, oopsx.B("deploy", "orch").Errorf("nil HIR mapper")
	}
	return &Orch{c: c, mapper: m}, nil
}

// Compiler returns the underlying plano compiler.
func (o *Orch) Compiler() *compiler.Compiler {
	if o == nil {
		return nil
	}
	return o.c
}

// LoadAppFile parses and compiles a .orch path into the canonical v1alpha1 App model.
func (o *Orch) LoadAppFile(ctx context.Context, path string) (*v1.App, error) {
	if o == nil || o.c == nil {
		return nil, oopsx.B("deploy", "orch").Errorf("nil Orch")
	}
	res := o.c.CompileFileDetailed(ctx, path)
	return o.appFromCompileResult(path, res)
}

// LoadAppBytes compiles .orch source bytes; virtualName appears in diagnostics and import resolution.
func (o *Orch) LoadAppBytes(ctx context.Context, virtualName string, src []byte) (*v1.App, error) {
	if o == nil || o.c == nil {
		return nil, oopsx.B("deploy", "orch").Errorf("nil Orch")
	}
	res := o.c.CompileSourceDetailed(ctx, virtualName, src)
	return o.appFromCompileResult(virtualName, res)
}

// LoadAppString compiles .orch source text.
func (o *Orch) LoadAppString(ctx context.Context, virtualName, src string) (*v1.App, error) {
	return o.LoadAppBytes(ctx, virtualName, []byte(src))
}

// LowerHIR lowers compiled orch HIR into [v1.App].
func (o *Orch) LowerHIR(hir *compiler.HIR) (*v1.App, error) {
	if o == nil {
		return nil, oopsx.B("deploy", "orch").Errorf("nil Orch")
	}
	return lowerHIRWithMapper(hir, o.mapper)
}

func (o *Orch) appFromCompileResult(virtualName string, res compiler.Result) (*v1.App, error) {
	if res.Diagnostics.HasError() {
		return nil, oopsx.B("deploy").Wrapf(diagsErr(res.FileSet, res.Diagnostics), "compile %s", virtualName)
	}
	if res.HIR == nil {
		return nil, oopsx.B("deploy").Errorf("compile %s: missing HIR", virtualName)
	}
	app, err := lowerHIRWithMapper(res.HIR, o.mapper)
	if err != nil {
		return nil, oopsx.B("deploy").Wrapf(err, "lower %s", virtualName)
	}
	if app.APIVersion == "" {
		app.APIVersion = "warden.arcgolabs.io/v1alpha1"
	}
	if app.Kind == "" {
		app.Kind = "App"
	}
	return app, nil
}

func diagsErr(fset *token.FileSet, d diag.Diagnostics) error {
	lines := make([]string, 0, len(d))
	for i := range d {
		lines = append(lines, d[i].Format(fset))
	}
	return fmt.Errorf("%s", strings.TrimSpace(strings.Join(lines, "\n")))
}
