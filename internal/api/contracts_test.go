package api

import (
	"encoding/json"
	"testing"

	"github.com/arcgolabs/collectionx/list"
)

func TestCollectionBackedOutputsSerializeAsJSONArrays(t *testing.T) {
	t.Parallel()

	workloads := ListWorkloadsOutput{}
	workloads.Body.Items = list.NewList(WorkloadItem{Name: "web", Runtime: "docker", Artifact: "nginx", Status: "running"})
	raw, err := json.Marshal(workloads.Body)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Items []WorkloadItem `json:"items"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.Items) != 1 || wire.Items[0].Name != "web" {
		t.Fatalf("wire items = %#v", wire.Items)
	}

	var decoded ListWorkloadsOutput
	if err := json.Unmarshal(raw, &decoded.Body); err != nil {
		t.Fatal(err)
	}
	got, ok := decoded.Body.Items.Get(0)
	if decoded.Body.Items.Len() != 1 || !ok || got.Name != "web" {
		t.Fatalf("decoded items = %#v", decoded.Body.Items)
	}
}

func TestCollectionBackedBootstrapRoutesSerializeAsJSONArray(t *testing.T) {
	t.Parallel()

	out := OrchVPNBootstrapOutput{}
	out.Body.ContainerRoutes = list.NewList("10.42.0.10/32")
	raw, err := json.Marshal(out.Body)
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		ContainerRoutes []string `json:"container_routes"`
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if len(wire.ContainerRoutes) != 1 || wire.ContainerRoutes[0] != "10.42.0.10/32" {
		t.Fatalf("container routes = %#v", wire.ContainerRoutes)
	}
}
