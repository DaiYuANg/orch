package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
)

func waitHTTPAssignment(ctx context.Context, t *testing.T, client *apiclient.Client, status string) api.AssignmentItem {
	t.Helper()
	return pollHTTP(t, "assignment", func() (api.AssignmentItem, bool) {
		out, err := client.ListAssignments(ctx)
		if err != nil {
			return api.AssignmentItem{}, false
		}
		return findAssignment(out.Body.Items, status)
	})
}

func waitHTTPWorkload(ctx context.Context, t *testing.T, client *apiclient.Client, name string) api.WorkloadItem {
	t.Helper()
	return pollHTTP(t, "workload "+name, func() (api.WorkloadItem, bool) {
		out, err := client.ListWorkloads(ctx)
		if err != nil {
			return api.WorkloadItem{}, false
		}
		return findWorkloadItem(out.Body.Items, name)
	})
}

func waitHTTPWorkloadGone(ctx context.Context, t *testing.T, client *apiclient.Client, name string) {
	t.Helper()
	pollHTTP(t, "workload "+name+" removed", func() (struct{}, bool) {
		out, err := client.ListWorkloads(ctx)
		if err != nil {
			return struct{}{}, false
		}
		_, found := findWorkloadItem(out.Body.Items, name)
		return struct{}{}, !found
	})
}

func waitHTTPApp(ctx context.Context, t *testing.T, client *apiclient.Client, status string) api.AppDetailItem {
	t.Helper()
	return pollHTTP(t, "app "+status, func() (api.AppDetailItem, bool) {
		out, err := client.GetApp(ctx, deployFlowNamespace, deployFlowApp)
		return out.Body, err == nil && out.Body.Status == status
	})
}

func waitHTTPAppGone(ctx context.Context, t *testing.T, client *apiclient.Client, namespace, name string) {
	t.Helper()
	pollHTTP(t, "app "+namespace+"/"+name+" removed", func() (struct{}, bool) {
		out, err := client.ListApps(ctx)
		if err != nil {
			return struct{}{}, false
		}
		_, found := findAppItem(out.Body.Items, namespace, name)
		return struct{}{}, !found
	})
}

func pollHTTP[T any](t *testing.T, desc string, check func() (T, bool)) T {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		value, ok := check()
		if ok {
			return value
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s did not converge", desc)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func findAssignment(items *list.List[api.AssignmentItem], status string) (api.AssignmentItem, bool) {
	return findFirst(items, func(item api.AssignmentItem) bool {
		return item.Key == deployFlowAssignmentKey() && item.Node == deployFlowRemoteNode && item.Status == status
	})
}

func findWorkloadItem(items *list.List[api.WorkloadItem], name string) (api.WorkloadItem, bool) {
	return findFirst(items, func(item api.WorkloadItem) bool {
		return item.Name == name
	})
}

func findAppItem(items *list.List[api.AppItem], namespace, name string) (api.AppItem, bool) {
	return findFirst(items, func(item api.AppItem) bool {
		return item.Namespace == namespace && item.Name == name
	})
}

func findFirst[T any](items *list.List[T], match func(T) bool) (T, bool) {
	var found T
	ok := false
	items.Range(func(_ int, item T) bool {
		if match(item) {
			found = item
			ok = true
			return false
		}
		return true
	})
	return found, ok
}

func deployFlowAssignmentKey() string {
	return deployFlowNamespace + "/" + deployFlowApp + "/" + deployFlowWorkload
}
