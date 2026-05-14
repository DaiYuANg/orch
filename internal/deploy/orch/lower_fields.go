package orch

import (
	"fmt"
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/plano/compiler"
	"github.com/arcgolabs/plano/schema"

	v1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func stringField(f *compiler.HIRForm, name string) (string, bool) {
	if f == nil || f.Fields == nil {
		return "", false
	}
	field, ok := f.Fields.Get(name)
	if !ok {
		return "", false
	}
	s, ok := field.Value.(string)
	return s, ok
}

func boolField(f *compiler.HIRForm, name string) (bool, bool) {
	if f == nil || f.Fields == nil {
		return false, false
	}
	field, ok := f.Fields.Get(name)
	if !ok {
		return false, false
	}
	b, ok := field.Value.(bool)
	return b, ok
}

func intField(f *compiler.HIRForm, name string) (int, bool) {
	value, ok := rawField(f, name)
	if !ok {
		return 0, false
	}
	i, ok := intFromAny(value)
	return i, ok
}

func int64Field(f *compiler.HIRForm, name string) (int64, bool) {
	value, ok := rawField(f, name)
	if !ok {
		return 0, false
	}
	switch x := value.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	case int32:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func rawField(f *compiler.HIRForm, name string) (any, bool) {
	if f == nil || f.Fields == nil {
		return nil, false
	}
	field, ok := f.Fields.Get(name)
	if !ok {
		return nil, false
	}
	return field.Value, true
}

func workloadRefField(f *compiler.HIRForm, name string) (string, bool) {
	value, ok := rawField(f, name)
	if !ok {
		return "", false
	}
	switch ref := value.(type) {
	case schema.Ref:
		if ref.Kind == "workload" && strings.TrimSpace(ref.Name) != "" {
			return strings.TrimSpace(ref.Name), true
		}
	case string:
		if strings.TrimSpace(ref) != "" {
			return strings.TrimSpace(ref), true
		}
	}
	return "", false
}

func stringMapField(f *compiler.HIRForm, name string) (map[string]string, bool) {
	value, ok := rawField(f, name)
	if !ok || value == nil {
		return nil, false
	}
	return mapStringString(value)
}

func mapStringString(value any) (map[string]string, bool) {
	switch m := value.(type) {
	case map[string]string:
		return m, true
	case *mapping.OrderedMap[string, any]:
		return orderedMapToStringMap(m), true
	case map[string]any:
		return anyMapToStringMap(m), true
	default:
		return nil, false
	}
}

func orderedMapToStringMap(m *mapping.OrderedMap[string, any]) map[string]string {
	out := mapping.NewMapWithCapacity[string, string](m.Len())
	m.Range(func(k string, val any) bool {
		out.Set(k, stringFromMapValue(val))
		return true
	})
	return out.All()
}

func anyMapToStringMap(m map[string]any) map[string]string {
	out := mapping.NewMapWithCapacity[string, string](len(m))
	for k, val := range m {
		out.Set(k, stringFromMapValue(val))
	}
	return out.All()
}

func stringFromMapValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}

func envVarsFromMap(m map[string]string) []v1.EnvVar {
	keys := mapping.NewMapFrom(m).Keys()
	sort.Strings(keys)
	return list.MapList(list.NewList(keys...), func(_ int, k string) v1.EnvVar {
		return v1.EnvVar{Name: k, Value: m[k]}
	}).Values()
}

func cloneStringMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	return mapping.NewMapFrom(m).All()
}

func intFromAny(value any) (int, bool) {
	switch x := value.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case int32:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func stringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return list.FilterMapList(list.NewList(items...), func(_ int, it any) (string, bool) {
		if s, ok := it.(string); ok {
			return s, true
		}
		return "", false
	}).Values()
}

func callIntArg(call compiler.HIRCall, idx int) (int, bool) {
	arg, ok := call.Args.Get(idx)
	if !ok {
		return 0, false
	}
	return intFromAny(arg.Value)
}

func callStringArg(call compiler.HIRCall, idx int) (string, bool) {
	arg, ok := call.Args.Get(idx)
	if !ok {
		return "", false
	}
	s, ok := arg.Value.(string)
	return s, ok
}

func workloadRefList(value any) []v1.WorkloadRef {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	return list.FilterMapList(list.NewList(items...), func(_ int, it any) (v1.WorkloadRef, bool) {
		if r, ok := it.(schema.Ref); ok && r.Kind == "workload" {
			return v1.WorkloadRef{Name: r.Name}, true
		}
		return v1.WorkloadRef{}, false
	}).Values()
}
