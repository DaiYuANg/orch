package api

// HTTP path constants (server base path is /api via httpx.WithBasePath).
const (
	BasePath = "/api"

	PathHealth      = BasePath + "/health"
	PathV1Hostinfo  = BasePath + "/v1/hostinfo"
	PathV1Workloads = BasePath + "/v1/workloads"
	PathV1Deploy    = BasePath + "/v1/deploy"
)
