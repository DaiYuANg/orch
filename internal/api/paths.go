package api

// HTTP path constants (server base path is /api via httpx.WithBasePath).
const (
	BasePath = "/api"

	PathHealth             = BasePath + "/health"
	PathV1Hostinfo         = BasePath + "/v1/hostinfo"
	PathV1Workloads        = BasePath + "/v1/workloads"
	PathV1Deploy           = BasePath + "/v1/deploy"
	PathV1DeploySource     = BasePath + "/v1/deploy/source"
	PathV1WorkerDeploy     = BasePath + "/v1/worker/deploy"
	PathV1OrchVPNBootstrap = BasePath + "/v1/orch-vpn/bootstrap"
)
