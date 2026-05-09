package lifecycleplan

import "time"

const (
	Concurrency         = 4
	RecentEventCapacity = 256

	PriorityLogging  = 0
	PriorityShutdown = 5
	PriorityIdentity = 10
	PriorityRaft     = 20
	PriorityNetwork  = 30
	PriorityWorkload = 40
	PriorityReady    = 100
)

const (
	TimeoutShort    = 5 * time.Second
	TimeoutStart    = 30 * time.Second
	TimeoutStop     = 15 * time.Second
	TimeoutShutdown = 10 * time.Second
	TimeoutReady    = 10 * time.Second
)

const (
	HookLogging       = "logging"
	HookNodeID        = "nodeid"
	HookRaft          = "raft"
	HookDNS           = "dns"
	HookIngress       = "ingress"
	HookHTTPServer    = "httpserver"
	HookOrchVPN       = "orchvpn-gateway"
	HookTaskReconcile = "task-reconcile"
	HookScheduler     = "scheduler"
	HookStartupInfo   = "startupinfo"
	HookObservability = "observability"
)
