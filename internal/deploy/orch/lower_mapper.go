package orch

import (
	"strings"

	"github.com/arcgolabs/mapper"
	"github.com/arcgolabs/plano/compiler"
)

// NewHIRMapper returns the mapper used by the .orch HIR lowering layer.
// Keep this in DI so compiler-side conversion policy is explicit and reusable.
func NewHIRMapper() *mapper.Mapper {
	return mapper.New(
		mapper.WithFallbackTags("json", "yaml"),
		mapper.WithPlanCacheSize(2048),
		mapper.Converter(strings.TrimSpace),
		mapper.Converter(func(value []any) []string {
			return stringList(value)
		}),
		mapper.Converter(orderedMapToStringMap),
	)
}

func mapHIRFields[D any](m *mapper.Mapper, f *compiler.HIRForm) D {
	var dst D
	if m == nil {
		return dst
	}
	if err := m.MapInto(&dst, hirFormValueMap(f)); err != nil {
		return dst
	}
	return dst
}

func hirFormValueMap(f *compiler.HIRForm) map[string]any {
	if f == nil || f.Fields == nil {
		return nil
	}
	values := make(map[string]any, f.Fields.Len())
	f.Fields.Range(func(name string, field compiler.HIRField) bool {
		values[name] = field.Value
		return true
	})
	return values
}
