package api

import (
	"sort"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"
)

func dixHealthItems(prefix string, report dix.HealthReport) *list.List[ReadyCheckItem] {
	items := make([]ReadyCheckItem, 0)
	if report.Checks != nil {
		report.Checks.Range(func(name string, err error) bool {
			item := ReadyCheckItem{
				Name:   strings.TrimSpace(prefix) + name,
				Ready:  err == nil,
				Status: healthCheckStatus(err),
			}
			if err != nil {
				item.Detail = err.Error()
			}
			items = append(items, item)
			return true
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return list.NewListWithCapacity[ReadyCheckItem](len(items), items...)
}

func addDixHealthItems(dst *list.List[ReadyCheckItem], prefix string, report dix.HealthReport) {
	if dst == nil {
		return
	}
	dixHealthItems(prefix, report).Range(func(_ int, item ReadyCheckItem) bool {
		dst.Add(item)
		return true
	})
}

func healthCheckStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
