package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/hostinfo"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// HostinfoEndpoint serves GET /api/v1/hostinfo.
type HostinfoEndpoint struct{}

func NewHostinfoEndpoint() *HostinfoEndpoint {
	return &HostinfoEndpoint{}
}

func (e *HostinfoEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/hostinfo",
		Description: "Host and runtime snapshot for diagnostics.",
		Tags:        httpx.Tags("host"),
	}
}

func (e *HostinfoEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"host"}, "hostinfo", "Collect host/runtime snapshot for diagnostics",
		"Returns CPU, memory, load, disk usage, and OS/host metadata from the local machine."))
}

func (e *HostinfoEndpoint) handle(ctx context.Context, _ *EmptyInput) (*HostinfoOutput, error) {
	snap, err := hostinfo.Collect(ctx)
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "collect hostinfo")
	}
	out := &HostinfoOutput{}
	out.Body = *snap
	return out, nil
}
