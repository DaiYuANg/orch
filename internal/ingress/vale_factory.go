package ingress

import (
	"log/slog"

	"github.com/arcgolabs/collectionx/list"
	valeruntime "github.com/arcgolabs/vale/runtime"

	"github.com/lyonbrown4d/orch/internal/config"
)

type valeFactory struct {
	buildSnapshot func(*list.List[config.IngressRoute]) (*valeruntime.CompiledSnapshot, int, error)
	newGateway    func(*valeruntime.CompiledSnapshot, *slog.Logger, bool) *valeruntime.Gateway
}

func newValeFactory() valeFactory {
	return valeFactory{
		buildSnapshot: buildValeSnapshot,
		newGateway:    newValeGateway,
	}
}

func (f valeFactory) build(routes *list.List[config.IngressRoute]) (*valeruntime.CompiledSnapshot, int, error) {
	if f.buildSnapshot != nil {
		return f.buildSnapshot(routes)
	}
	return buildValeSnapshot(routes)
}

func (f valeFactory) gateway(snapshot *valeruntime.CompiledSnapshot, log *slog.Logger, dynamic bool) *valeruntime.Gateway {
	if f.newGateway != nil {
		return f.newGateway(snapshot, log, dynamic)
	}
	return newValeGateway(snapshot, log, dynamic)
}

func newValeGateway(snapshot *valeruntime.CompiledSnapshot, log *slog.Logger, dynamic bool) *valeruntime.Gateway {
	return valeruntime.NewGateway(snapshot, log, dynamic, valeruntime.NewNoopMetrics())
}
