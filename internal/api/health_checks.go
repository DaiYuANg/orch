package api

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"
)

func dixHealthItems(prefix string, report dix.HealthReport) *list.List[ReadyCheckItem] {
	items := list.NewListWithCapacity[ReadyCheckItem](healthCheckCapacity(report))
	if report.Checks != nil {
		namePrefix := strings.TrimSpace(prefix)
		report.Checks.Range(func(name string, err error) bool {
			item := ReadyCheckItem{
				Name:   namePrefix + name,
				Ready:  err == nil,
				Status: healthCheckStatus(err),
			}
			if err != nil {
				item.Detail = err.Error()
			}
			items.Add(item)
			return true
		})
	}
	return items.Sort(func(left, right ReadyCheckItem) int {
		return strings.Compare(left.Name, right.Name)
	})
}

func addDixHealthItems(dst *list.List[ReadyCheckItem], prefix string, report dix.HealthReport) {
	if dst == nil {
		return
	}
	dst.Merge(dixHealthItems(prefix, report))
}

func healthCheckCapacity(report dix.HealthReport) int {
	if report.Checks == nil {
		return 0
	}
	return report.Checks.Len()
}

func healthCheckStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
