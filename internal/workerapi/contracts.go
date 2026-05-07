package workerapi

import deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"

const PathV1WorkerDeploy = "/api/v1/worker/deploy"

type DeployWorkloadBody struct {
	Metadata deployv1.Metadata `json:"metadata"`
	Workload deployv1.Workload `json:"workload"`
	Node     string            `json:"node,omitempty"`
}

type DeployWorkloadInput struct {
	Body DeployWorkloadBody `json:"body"`
}

type DeployWorkloadOutput struct {
	Body struct {
		Accepted bool   `json:"accepted"`
		Node     string `json:"node"`
		Status   string `json:"status"`
		Workload string `json:"workload"`
	} `json:"body"`
}
