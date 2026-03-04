package task

import (
	"sort"
	"strings"

	"github.com/samber/lo"
)

func mapToEnv(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	items := lo.FilterMap(lo.Entries(values), func(item lo.Entry[string, string], _ int) (string, bool) {
		key := strings.TrimSpace(item.Key)
		if key == "" {
			return "", false
		}
		return key + "=" + item.Value, true
	})
	sort.Strings(items)
	return items
}

func matchLabelFilters(labels map[string]string, filters map[string][]string) bool {
	labelFilters := filters["label"]
	if len(labelFilters) == 0 {
		return true
	}
	return lo.EveryBy(labelFilters, func(item string) bool {
		pair := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(pair) != 2 {
			return false
		}
		key := strings.TrimSpace(pair[0])
		value := strings.TrimSpace(pair[1])
		return labels[key] == value
	})
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	return lo.Reduce(lo.Entries(values), func(agg map[string]string, item lo.Entry[string, string], _ int) map[string]string {
		agg[item.Key] = item.Value
		return agg
	}, map[string]string{})
}

func valueAt(values []string, idx int) string {
	if idx < 0 || idx >= len(values) {
		return ""
	}
	return values[idx]
}
