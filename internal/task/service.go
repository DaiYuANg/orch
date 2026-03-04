package task

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	internalconfig "github.com/DaiYuANg/warden/internal/config"
	internaldns "github.com/DaiYuANg/warden/internal/dns"
	"github.com/DaiYuANg/warden/internal/dsl"
	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/DaiYuANg/warden/internal/registry"
	"github.com/DaiYuANg/warden/pkg"
	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

const (
	labelManaged      = "warden.managed"
	labelDeploymentID = "warden.deployment.id"
	labelWorkload     = "warden.workload"
	labelUnit         = "warden.unit"
	labelTask         = "warden.task"
	labelReplica      = "warden.replica"
	labelInstanceID   = "warden.instance.id"
	labelRuntime      = "warden.runtime"
	labelPortPrefix   = "warden.port."
	labelDNSResolver  = "warden.dns.resolver"
	labelDNSSearch    = "warden.dns.search."
	labelCheckType    = "warden.check.type"
	labelCheckPath    = "warden.check.path"
	labelCheckCmd     = "warden.check.command"
	labelCheckRetries = "warden.check.retries"
	labelCheckTimeout = "warden.check.timeout"
	labelCheckIntv    = "warden.check.interval"
	labelService      = "warden.service"
)

type Service struct {
	logger *slog.Logger

	mu          sync.RWMutex
	raft        *raft.Service
	runtime     RuntimeExecutor
	runtimeInit RuntimeFactory
	registry    *registry.Service
	nodeID      string
	nodeIP      string
	nodeAPI     map[string]string
	dnsIP       string
	httpClient  *http.Client
	dnsServer   *internaldns.DNSServer
	deployments map[string]*deploymentRecord
	instances   map[string]*instanceRecord

	reconcileInterval time.Duration
	stopCh            chan struct{}
	stopped           chan struct{}
}

func NewService(logger *slog.Logger, registryService *registry.Service) *Service {
	return newService(logger, nil, nil, registryService, nil, newDockerRuntimeExecutor)
}

func NewServiceWithRuntimeFactory(logger *slog.Logger, registryService *registry.Service, runtimeFactory RuntimeFactory) *Service {
	return newService(logger, nil, nil, registryService, nil, runtimeFactory)
}

func newServiceWithRaft(
	logger *slog.Logger,
	cfg *internalconfig.Config,
	registryService *registry.Service,
	raftService *raft.Service,
	dnsServer *internaldns.DNSServer,
) *Service {
	return newService(logger, cfg, raftService, registryService, dnsServer, newDockerRuntimeExecutor)
}

func newService(
	logger *slog.Logger,
	cfg *internalconfig.Config,
	raftService *raft.Service,
	registryService *registry.Service,
	dnsServer *internaldns.DNSServer,
	runtimeFactory RuntimeFactory,
) *Service {
	if runtimeFactory == nil {
		runtimeFactory = newDockerRuntimeExecutor
	}
	nodeID := ""
	if raftService != nil && raftService.Enabled() {
		nodeID = strings.TrimSpace(raftService.NodeID())
	}
	if nodeID == "" {
		nodeID, _ = pkg.MachineID()
	}
	nodeIP := detectNodeIP()
	if nodeID == "" {
		nodeID = uuid.NewString()
	}
	if nodeIP == "" {
		nodeIP = "127.0.0.1"
	}
	return &Service{
		logger:            logger,
		raft:              raftService,
		runtimeInit:       runtimeFactory,
		registry:          registryService,
		nodeID:            nodeID,
		nodeIP:            nodeIP,
		nodeAPI:           buildNodeAPIIndex(cfg),
		dnsIP:             resolveIngressAdvertiseIP(cfg, nodeIP),
		httpClient:        &http.Client{Timeout: 10 * time.Second},
		dnsServer:         dnsServer,
		deployments:       make(map[string]*deploymentRecord),
		instances:         make(map[string]*instanceRecord),
		reconcileInterval: 5 * time.Second,
		stopCh:            make(chan struct{}),
		stopped:           make(chan struct{}),
	}
}

func (s *Service) Start(_ context.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.ensureRuntime(ctx); err != nil {
		s.logger.Warn("runtime unavailable at startup", "error", err)
	} else {
		if err := s.recoverManagedContainers(ctx); err != nil {
			s.logger.Warn("recover managed containers failed", "error", err)
		}
	}

	go s.reconcileLoop()
	return nil
}

func (s *Service) Stop(ctx context.Context) error {
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}

	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	format, err := dsl.DetectFormat(req.Filename, req.Format)
	if err != nil {
		return nil, err
	}
	workload, err := dsl.ParseContent(format, []byte(req.Content))
	if err != nil {
		return nil, err
	}
	if err := dsl.ValidateWorkload(workload); err != nil {
		return nil, err
	}
	if err := s.ensureLeaderForScheduling(); err != nil {
		return nil, err
	}

	deploymentID := uuid.NewString()
	assignment, err := s.upsertSchedulingAssignment(deploymentID, workload.Name)
	if err != nil {
		return nil, err
	}
	cleanupAssignment := true
	defer func() {
		if cleanupAssignment {
			_ = s.deleteSchedulingAssignment(deploymentID)
		}
	}()

	now := time.Now()
	deployment := &deploymentRecord{
		DeploymentInfo: DeploymentInfo{
			ID:          deploymentID,
			Workload:    workload.Name,
			Format:      format,
			Status:      DeploymentStatusRunning,
			DesiredNode: assignment.DesiredNode,
			WorkerNode:  assignment.WorkerNode,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		instances: make(map[string]*instanceRecord),
	}

	localExec := mo.None[RuntimeExecutor]()
	if assignment.WorkerNode == s.nodeID {
		exec, runtimeErr := s.ensureRuntime(ctx)
		if runtimeErr != nil {
			return nil, runtimeErr
		}
		localExec = mo.Some(exec)
	}

	created := make([]*instanceRecord, 0)
	for _, unit := range workload.Units {
		for _, taskSpec := range unit.Tasks {
			serviceName := buildServiceName(workload.Name, unit.Name, taskSpec.Name)
			routeDefs := s.buildRoutesForTask(deploymentID, serviceName, workload.Name, unit.Name, taskSpec)
			for _, route := range routeDefs {
				if err := s.upsertRoute(route); err != nil {
					return nil, fmt.Errorf("upsert route %s: %w", route.ID, err)
				}
				s.syncDNSForRoute(route)
				deployment.RouteIDs = append(deployment.RouteIDs, route.ID)
			}

			replicas := taskSpec.Replicas
			if replicas <= 0 {
				replicas = 1
			}
			for replica := 0; replica < replicas; replica++ {
				runtimeName := "docker"
				if localExec.IsPresent() {
					runtimeName = localExec.OrEmpty().Driver()
				}
				instance := s.newInstanceRecord(deploymentID, runtimeName, serviceName, workload.Name, unit.Name, taskSpec, replica)

				containerID := ""
				if localExec.IsPresent() {
					runExec := localExec.OrEmpty()
					var runErr error
					containerID, runErr = runExec.Run(ctx, instance.RunSpec)
					if runErr != nil {
						lo.ForEach(created, func(item *instanceRecord, _ int) {
							_ = s.stopInstanceContainer(ctx, item)
						})
						_ = s.deleteRoutesByOwner(deploymentID)
						return nil, fmt.Errorf("start workload %s/%s/%s[%d]: %w", workload.Name, unit.Name, taskSpec.Name, replica, runErr)
					}
				} else {
					workerRun, runErr := s.runContainerOnWorker(ctx, assignment.WorkerNode, instance.RunSpec)
					if runErr != nil {
						lo.ForEach(created, func(item *instanceRecord, _ int) {
							_ = s.stopInstanceContainer(ctx, item)
						})
						_ = s.deleteRoutesByOwner(deploymentID)
						return nil, fmt.Errorf("start remote workload %s/%s/%s[%d] on %s: %w", workload.Name, unit.Name, taskSpec.Name, replica, assignment.WorkerNode, runErr)
					}
					containerID = workerRun.ContainerID
					if strings.TrimSpace(workerRun.Driver) != "" {
						instance.Driver = workerRun.Driver
					}
					if strings.TrimSpace(workerRun.NodeID) != "" {
						instance.NodeID = workerRun.NodeID
					} else {
						instance.NodeID = assignment.WorkerNode
					}
					if strings.TrimSpace(workerRun.NodeIP) != "" {
						instance.NodeIP = workerRun.NodeIP
					}
				}
				instance.ContainerID = containerID
				instance.Status = InstanceStatusRunning
				instance.UpdatedAt = time.Now()
				if err := s.upsertRegistryEndpoint(instance, true); err != nil {
					_ = s.stopInstanceContainer(ctx, instance)
					lo.ForEach(created, func(item *instanceRecord, _ int) {
						_ = s.stopInstanceContainer(ctx, item)
					})
					_ = s.deleteRoutesByOwner(deploymentID)
					return nil, fmt.Errorf("register endpoint %s: %w", instance.ID, err)
				}
				created = append(created, instance)
				deployment.instances[instance.ID] = instance
				deployment.InstanceIDs = append(deployment.InstanceIDs, instance.ID)
			}
		}
	}

	if len(created) == 0 {
		return nil, fmt.Errorf("no runnable task found in workload %s", workload.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.deployments[deploymentID] = deployment
	for _, instance := range created {
		s.instances[instance.ID] = instance
	}
	cleanupAssignment = false

	return &DeployResult{
		DeploymentID: deploymentID,
		WorkloadName: workload.Name,
		Instances:    len(created),
	}, nil
}

func (s *Service) GetDeployment(id string) (DeploymentDetail, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	dep, ok := s.deployments[id]
	if !ok {
		return DeploymentDetail{}, false
	}
	detail := DeploymentDetail{
		Deployment: dep.DeploymentInfo,
		Instances:  make([]InstanceInfo, 0, len(dep.instances)),
	}
	for _, instance := range dep.instances {
		detail.Instances = append(detail.Instances, instance.InstanceInfo)
	}
	return detail, true
}

func (s *Service) ListDeployments() []DeploymentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Map(lo.Values(s.deployments), func(dep *deploymentRecord, _ int) DeploymentInfo {
		return dep.DeploymentInfo
	})
}

func (s *Service) StopDeployment(ctx context.Context, deploymentID string) error {
	s.mu.RLock()
	dep, ok := s.deployments[deploymentID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}
	instances := make([]*instanceRecord, 0, len(dep.instances))
	for _, item := range dep.instances {
		instances = append(instances, item)
	}
	s.mu.RUnlock()

	var firstErr error
	for _, item := range instances {
		if stopErr := s.stopInstanceContainer(ctx, item); stopErr != nil && firstErr == nil {
			firstErr = stopErr
		}
		if err := s.deleteEndpoint(item.ID); err != nil && firstErr == nil {
			firstErr = err
		}
		s.mu.Lock()
		if realItem, exists := s.instances[item.ID]; exists {
			realItem.Status = InstanceStatusStopped
			realItem.UpdatedAt = time.Now()
		}
		s.mu.Unlock()
	}

	s.mu.Lock()
	if realDep, exists := s.deployments[deploymentID]; exists {
		realDep.Status = DeploymentStatusStopped
		realDep.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
	if err := s.deleteRoutesByOwner(deploymentID); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.deleteSchedulingAssignment(deploymentID); err != nil && firstErr == nil {
		firstErr = err
	}

	return firstErr
}

func (s *Service) Logs(ctx context.Context, instanceID string, tail int) (string, error) {
	s.mu.RLock()
	instance, ok := s.instances[instanceID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("instance not found: %s", instanceID)
	}

	if strings.TrimSpace(instance.NodeID) != "" && instance.NodeID != s.nodeID {
		return s.readContainerLogsOnWorker(ctx, instance.NodeID, instance.ContainerID, tail)
	}

	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return "", err
	}
	return exec.Logs(ctx, instance.ContainerID, tail)
}

func (s *Service) reconcileLoop() {
	ticker := time.NewTicker(s.reconcileInterval)
	defer func() {
		ticker.Stop()
		close(s.stopped)
	}()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.reconcile()
		}
	}
}

func (s *Service) reconcile() {
	s.mu.RLock()
	instanceIDs := lo.Keys(s.instances)
	s.mu.RUnlock()

	lo.ForEach(instanceIDs, func(id string, _ int) {
		s.reconcileInstance(id)
	})
}

func (s *Service) reconcileInstance(instanceID string) {
	s.mu.RLock()
	instance, ok := s.instances[instanceID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	snapshot := *instance
	s.mu.RUnlock()

	if snapshot.Status == InstanceStatusStopped || snapshot.Status == InstanceStatusFailed {
		return
	}
	if strings.TrimSpace(snapshot.NodeID) != "" && snapshot.NodeID != s.nodeID {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		s.markInstanceFailed(instanceID, fmt.Sprintf("runtime unavailable: %v", err))
		return
	}

	status, err := exec.Status(ctx, snapshot.ContainerID)
	if err != nil || !status.Running {
		_ = s.setEndpointHealth(snapshot.ID, false)
		reason := "container not running"
		if err != nil {
			reason = err.Error()
		}
		s.restartInstance(instanceID, reason)
		return
	}

	if strings.EqualFold(snapshot.HealthCheck.Type, "http") {
		if !snapshot.LastCheckAt.IsZero() && time.Since(snapshot.LastCheckAt) < snapshot.HealthCheck.Interval {
			return
		}
		if err := s.runHTTPHealthCheck(snapshot.HealthCheck); err != nil {
			_ = s.setEndpointHealth(snapshot.ID, false)
			s.mu.Lock()
			if real, exists := s.instances[instanceID]; exists {
				real.LastCheckAt = time.Now()
				real.ConsecutiveFailed++
				real.LastError = err.Error()
				real.UpdatedAt = time.Now()
				if real.ConsecutiveFailed >= real.HealthCheck.Retries {
					s.mu.Unlock()
					s.restartInstance(instanceID, "http health check failed")
					return
				}
				real.Status = InstanceStatusUnknown
				s.refreshDeploymentStatusLocked(real.DeploymentID)
			}
			s.mu.Unlock()
			return
		}
	}

	s.mu.Lock()
	if real, exists := s.instances[instanceID]; exists {
		real.LastCheckAt = time.Now()
		real.Status = InstanceStatusRunning
		real.ConsecutiveFailed = 0
		real.LastError = ""
		real.UpdatedAt = time.Now()
		s.refreshDeploymentStatusLocked(real.DeploymentID)
	}
	s.mu.Unlock()
	_ = s.setEndpointHealth(snapshot.ID, true)
}

func (s *Service) restartInstance(instanceID string, reason string) {
	s.mu.RLock()
	instance, ok := s.instances[instanceID]
	if !ok {
		s.mu.RUnlock()
		return
	}
	snapshot := *instance
	s.mu.RUnlock()

	if snapshot.RestartCount >= snapshot.MaxRestarts {
		s.markInstanceFailed(instanceID, fmt.Sprintf("restart budget exhausted: %s", reason))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		s.markInstanceFailed(instanceID, fmt.Sprintf("runtime unavailable for restart: %v", err))
		return
	}

	_ = exec.Stop(ctx, snapshot.ContainerID)
	newContainerID, err := exec.Run(ctx, snapshot.RunSpec)
	if err != nil {
		s.mu.Lock()
		if real, exists := s.instances[instanceID]; exists {
			real.RestartCount++
			real.LastError = err.Error()
			real.UpdatedAt = time.Now()
			if real.RestartCount >= real.MaxRestarts {
				real.Status = InstanceStatusFailed
			}
			s.refreshDeploymentStatusLocked(real.DeploymentID)
		}
		s.mu.Unlock()
		_ = s.setEndpointHealth(snapshot.ID, false)
		return
	}

	s.mu.Lock()
	if real, exists := s.instances[instanceID]; exists {
		real.ContainerID = newContainerID
		real.RestartCount++
		real.ConsecutiveFailed = 0
		real.LastError = ""
		real.Status = InstanceStatusRunning
		real.UpdatedAt = time.Now()
		s.refreshDeploymentStatusLocked(real.DeploymentID)
	}
	s.mu.Unlock()
	if refreshed, ok := s.getInstance(instanceID); ok {
		_ = s.upsertRegistryEndpoint(refreshed, true)
	}
}

func (s *Service) recoverManagedContainers(ctx context.Context) error {
	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return err
	}

	items, err := exec.List(ctx, true, map[string][]string{
		"label": []string{labelManaged + "=true"},
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range items {
		deploymentID := c.Labels[labelDeploymentID]
		instanceID := c.Labels[labelInstanceID]
		if deploymentID == "" || instanceID == "" {
			continue
		}
		dep, ok := s.deployments[deploymentID]
		if !ok {
			now := time.Now()
			dep = &deploymentRecord{
				DeploymentInfo: DeploymentInfo{
					ID:        deploymentID,
					Workload:  c.Labels[labelWorkload],
					Format:    "unknown",
					Status:    DeploymentStatusRunning,
					CreatedAt: now,
					UpdatedAt: now,
				},
				instances: make(map[string]*instanceRecord),
			}
			s.deployments[deploymentID] = dep
		}
		if _, exists := s.instances[instanceID]; exists {
			continue
		}

		replica, _ := strconv.Atoi(c.Labels[labelReplica])
		serviceName := c.Labels[labelService]
		if serviceName == "" {
			serviceName = buildServiceName(c.Labels[labelWorkload], c.Labels[labelUnit], c.Labels[labelTask])
		}
		driver := strings.TrimSpace(c.Labels[labelRuntime])
		if driver == "" {
			driver = exec.Driver()
		}
		instance := &instanceRecord{
			InstanceInfo: InstanceInfo{
				ID:           instanceID,
				DeploymentID: deploymentID,
				Service:      serviceName,
				Workload:     c.Labels[labelWorkload],
				Unit:         c.Labels[labelUnit],
				Task:         c.Labels[labelTask],
				Replica:      replica,
				NodeID:       s.nodeID,
				NodeIP:       s.nodeIP,
				Driver:       driver,
				ContainerID:  c.ID,
				Status:       InstanceStatusRunning,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			RunSpec: RuntimeRunSpec{
				Name:       trimContainerName(c.Names),
				Image:      c.Image,
				Labels:     c.Labels,
				Ports:      labelsToPorts(c.Labels),
				DNSServers: labelsToDNSServers(c.Labels),
				DNSSearch:  labelsToDNSSearch(c.Labels),
			},
			HealthCheck: labelsToHealthCheck(c.Labels),
			MaxRestarts: max(3, mustAtoi(c.Labels[labelCheckRetries], 3)),
			LastCheckAt: time.Now(),
		}
		s.instances[instance.ID] = instance
		dep.instances[instance.ID] = instance
		dep.InstanceIDs = append(dep.InstanceIDs, instance.ID)
		_ = s.upsertRegistryEndpoint(instance, true)

		routes := s.buildRoutesFromLabelMap(
			deploymentID,
			serviceName,
			instance.Workload,
			instance.Unit,
			instance.Task,
			c.Labels,
			labelsToPorts(c.Labels),
		)
		for _, route := range routes {
			_ = s.upsertRoute(route)
			if !containsString(dep.RouteIDs, route.ID) {
				dep.RouteIDs = append(dep.RouteIDs, route.ID)
			}
		}
	}
	return nil
}

func (s *Service) ensureRuntime(ctx context.Context) (RuntimeExecutor, error) {
	s.mu.RLock()
	exec := s.runtime
	s.mu.RUnlock()
	if exec != nil {
		return exec, exec.Ping(ctx)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtime != nil {
		return s.runtime, s.runtime.Ping(ctx)
	}
	runtime, err := s.runtimeInit()
	if err != nil {
		return nil, err
	}
	if err := runtime.Ping(ctx); err != nil {
		return nil, err
	}
	s.runtime = runtime
	return s.runtime, nil
}

func (s *Service) runHTTPHealthCheck(check healthCheckSpec) error {
	if check.Port <= 0 {
		return fmt.Errorf("http health check missing port")
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", check.Port, ensurePath(check.Path))
	timeout := check.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("health status=%d", resp.StatusCode)
	}
	return nil
}

func (s *Service) markInstanceFailed(instanceID string, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, ok := s.instances[instanceID]
	if !ok {
		return
	}
	instance.Status = InstanceStatusFailed
	instance.LastError = message
	instance.UpdatedAt = time.Now()
	s.refreshDeploymentStatusLocked(instance.DeploymentID)
	_ = s.setEndpointHealth(instanceID, false)
}

func (s *Service) refreshDeploymentStatusLocked(deploymentID string) {
	dep, ok := s.deployments[deploymentID]
	if !ok {
		return
	}
	dep.UpdatedAt = time.Now()
	running := 0
	stopped := 0
	for _, instance := range dep.instances {
		switch instance.Status {
		case InstanceStatusRunning, InstanceStatusUnknown:
			running++
		case InstanceStatusStopped:
			stopped++
		}
	}
	switch {
	case running > 0:
		dep.Status = DeploymentStatusRunning
	case stopped == len(dep.instances):
		dep.Status = DeploymentStatusStopped
	default:
		dep.Status = DeploymentStatusFailed
	}
}

func (s *Service) newInstanceRecord(deploymentID, runtimeName, serviceName, workloadName, unitName string, taskSpec dsl.Task, replica int) *instanceRecord {
	if strings.TrimSpace(runtimeName) == "" {
		runtimeName = "docker"
	}
	instanceID := uuid.NewString()
	check := buildHealthCheck(taskSpec)
	labels := buildLabels(deploymentID, runtimeName, serviceName, workloadName, unitName, taskSpec.Name, instanceID, replica, check, taskSpec.Network, taskSpec.DNS)
	for key, value := range collectTaskLabels(taskSpec) {
		labels[key] = value
	}

	spec := RuntimeRunSpec{
		Name:       sanitizeContainerName(fmt.Sprintf("warden-%s-%s-%s-%d", workloadName, unitName, taskSpec.Name, replica)),
		Image:      taskSpec.Image,
		Cmd:        taskSpec.Command,
		Env:        taskSpec.Env,
		Labels:     labels,
		Ports:      extractPorts(taskSpec.Network),
		DNSServers: extractDNSServers(taskSpec.DNS),
		DNSSearch:  extractDNSSearch(taskSpec.DNS),
	}

	now := time.Now()
	return &instanceRecord{
		InstanceInfo: InstanceInfo{
			ID:           instanceID,
			DeploymentID: deploymentID,
			Service:      serviceName,
			Workload:     workloadName,
			Unit:         unitName,
			Task:         taskSpec.Name,
			Replica:      replica,
			NodeID:       s.nodeID,
			NodeIP:       s.nodeIP,
			Driver:       runtimeName,
			Status:       InstanceStatusUnknown,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		RunSpec:     spec,
		HealthCheck: check,
		MaxRestarts: max(3, check.Retries),
	}
}

func buildHealthCheck(taskSpec dsl.Task) healthCheckSpec {
	ports := extractPorts(taskSpec.Network)
	port := 0
	if p, ok := ports["http"]; ok {
		port = p
	}
	if port == 0 {
		port = mo.TupleToOption(lo.Find(lo.Values(ports), func(candidate int) bool {
			return candidate > 0
		})).OrElse(0)
	}

	check := healthCheckSpec{
		Type:     "",
		Port:     port,
		Interval: 10 * time.Second,
		Timeout:  3 * time.Second,
		Retries:  3,
	}
	if taskSpec.Check == nil {
		return check
	}

	check.Type = strings.ToLower(strings.TrimSpace(taskSpec.Check.Type))
	check.Path = taskSpec.Check.Path
	check.Command = taskSpec.Check.Command
	if taskSpec.Check.Interval != "" {
		if d, err := time.ParseDuration(taskSpec.Check.Interval); err == nil {
			check.Interval = d
		}
	}
	if taskSpec.Check.Timeout != "" {
		if d, err := time.ParseDuration(taskSpec.Check.Timeout); err == nil {
			check.Timeout = d
		}
	}
	if taskSpec.Check.Retries > 0 {
		check.Retries = taskSpec.Check.Retries
	}
	return check
}

func extractPorts(network *dsl.NetworkConfig) map[string]int {
	if network == nil || len(network.Port) == 0 {
		return nil
	}
	return lo.PickBy(network.Port, func(_ string, port int) bool {
		return port > 0
	})
}

func buildLabels(
	deploymentID string,
	runtimeName string,
	serviceName string,
	workloadName string,
	unitName string,
	taskName string,
	instanceID string,
	replica int,
	check healthCheckSpec,
	network *dsl.NetworkConfig,
	dnsConfig *dsl.DNSConfig,
) map[string]string {
	labels := map[string]string{
		labelManaged:      "true",
		labelDeploymentID: deploymentID,
		labelRuntime:      runtimeName,
		labelService:      serviceName,
		labelWorkload:     workloadName,
		labelUnit:         unitName,
		labelTask:         taskName,
		labelInstanceID:   instanceID,
		labelReplica:      strconv.Itoa(replica),
		labelCheckType:    check.Type,
		labelCheckPath:    check.Path,
		labelCheckCmd:     check.Command,
		labelCheckRetries: strconv.Itoa(check.Retries),
		labelCheckTimeout: check.Timeout.String(),
		labelCheckIntv:    check.Interval.String(),
	}
	if network != nil {
		for name, port := range network.Port {
			labels[labelPortPrefix+name] = strconv.Itoa(port)
		}
	}
	if dnsConfig != nil {
		resolver := strings.TrimSpace(dnsConfig.Resolver)
		if resolver != "" {
			labels[labelDNSResolver] = resolver
		}
		domains := extractDNSSearch(dnsConfig)
		for idx, domain := range domains {
			labels[labelDNSSearch+strconv.Itoa(idx)] = domain
		}
	}
	return labels
}

func labelsToPorts(labels map[string]string) map[string]int {
	ports := lo.Reduce(lo.Entries(labels), func(agg map[string]int, item lo.Entry[string, string], _ int) map[string]int {
		if !strings.HasPrefix(item.Key, labelPortPrefix) {
			return agg
		}
		port, err := strconv.Atoi(item.Value)
		if err != nil || port <= 0 {
			return agg
		}
		name := strings.TrimPrefix(item.Key, labelPortPrefix)
		agg[name] = port
		return agg
	}, map[string]int{})
	if len(ports) == 0 {
		return nil
	}
	return ports
}

func extractDNSServers(config *dsl.DNSConfig) []string {
	if config == nil {
		return nil
	}
	resolver := strings.TrimSpace(config.Resolver)
	if resolver == "" {
		return nil
	}
	raw := strings.FieldsFunc(resolver, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	servers := lo.FilterMap(raw, func(item string, _ int) (string, bool) {
		trimmed := strings.TrimSpace(item)
		return trimmed, trimmed != ""
	})
	if len(servers) == 0 {
		return nil
	}
	return lo.Uniq(servers)
}

func extractDNSSearch(config *dsl.DNSConfig) []string {
	if config == nil {
		return nil
	}
	domains := lo.FilterMap(config.Domains, func(item string, _ int) (string, bool) {
		trimmed := strings.TrimSpace(item)
		return trimmed, trimmed != ""
	})
	if len(domains) == 0 {
		return nil
	}
	return lo.Uniq(domains)
}

func labelsToDNSServers(labels map[string]string) []string {
	return extractDNSServers(&dsl.DNSConfig{
		Resolver: labels[labelDNSResolver],
	})
}

func labelsToDNSSearch(labels map[string]string) []string {
	entries := lo.Filter(lo.Entries(labels), func(item lo.Entry[string, string], _ int) bool {
		return strings.HasPrefix(item.Key, labelDNSSearch)
	})
	if len(entries) == 0 {
		return nil
	}
	type indexed struct {
		idx    int
		domain string
	}
	sorted := lo.Map(entries, func(item lo.Entry[string, string], _ int) indexed {
		idx := mustAtoi(strings.TrimPrefix(item.Key, labelDNSSearch), 0)
		return indexed{
			idx:    idx,
			domain: strings.TrimSpace(item.Value),
		}
	})
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].idx < sorted[j].idx
	})
	domains := lo.FilterMap(sorted, func(item indexed, _ int) (string, bool) {
		return item.domain, item.domain != ""
	})
	if len(domains) == 0 {
		return nil
	}
	return lo.Uniq(domains)
}

func labelsToHealthCheck(labels map[string]string) healthCheckSpec {
	check := healthCheckSpec{
		Type:     labels[labelCheckType],
		Path:     labels[labelCheckPath],
		Command:  labels[labelCheckCmd],
		Retries:  mustAtoi(labels[labelCheckRetries], 3),
		Timeout:  mustDuration(labels[labelCheckTimeout], 3*time.Second),
		Interval: mustDuration(labels[labelCheckIntv], 10*time.Second),
	}
	ports := labelsToPorts(labels)
	if p, ok := ports["http"]; ok {
		check.Port = p
	} else {
		check.Port = mo.TupleToOption(lo.Find(lo.Values(ports), func(port int) bool {
			return port > 0
		})).OrElse(0)
	}
	return check
}

func sanitizeContainerName(in string) string {
	if in == "" {
		return "warden-task"
	}
	builder := strings.Builder{}
	for _, r := range strings.ToLower(strings.TrimSpace(in)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('-')
	}
	out := strings.Trim(builder.String(), "-_")
	if out == "" {
		return "warden-task"
	}
	if len(out) > 63 {
		return out[:63]
	}
	return out
}

func trimContainerName(names []string) string {
	for _, name := range names {
		trimmed := strings.TrimPrefix(strings.TrimSpace(name), "/")
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mustDuration(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return v
}

func mustAtoi(raw string, fallback int) int {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func ensurePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func containsString(items []string, target string) bool {
	return lo.Contains(items, target)
}

func (s *Service) getInstance(instanceID string) (*instanceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	instance, ok := s.instances[instanceID]
	if !ok {
		return nil, false
	}
	copy := *instance
	return &copy, true
}

func (s *Service) upsertRegistryEndpoint(instance *instanceRecord, healthy bool) error {
	if s.registry == nil || instance == nil {
		return nil
	}
	nodeID := strings.TrimSpace(instance.NodeID)
	if nodeID == "" {
		nodeID = s.nodeID
	}
	nodeIP := strings.TrimSpace(instance.NodeIP)
	if nodeIP == "" {
		nodeIP = s.nodeIP
	}
	endpoint := registry.ServiceEndpoint{
		ID:       instance.ID,
		Service:  instance.Service,
		NodeID:   nodeID,
		NodeIP:   nodeIP,
		Runtime:  instance.Driver,
		Protocol: registry.RouteProtocolHTTP,
		Ports:    instance.RunSpec.Ports,
		Healthy:  healthy,
		Metadata: map[string]string{
			"deployment_id": instance.DeploymentID,
			"workload":      instance.Workload,
			"unit":          instance.Unit,
			"task":          instance.Task,
			"container_id":  instance.ContainerID,
		},
		CreatedAt: instance.CreatedAt,
		UpdatedAt: time.Now(),
	}
	return s.registry.UpsertEndpoint(endpoint)
}

func (s *Service) setEndpointHealth(endpointID string, healthy bool) error {
	if s.registry == nil {
		return nil
	}
	return s.registry.SetEndpointHealth(endpointID, healthy)
}

func (s *Service) upsertRoute(route registry.Route) error {
	if s.registry == nil {
		return nil
	}
	return s.registry.UpsertRoute(route)
}

func (s *Service) deleteEndpoint(endpointID string) error {
	if s.registry == nil {
		return nil
	}
	return s.registry.DeleteEndpoint(endpointID)
}

func (s *Service) deleteRoutesByOwner(ownerID string) error {
	if s.registry == nil {
		return nil
	}
	httpRoutes, err := s.registry.ListRoutes(registry.RouteProtocolHTTP)
	if err != nil {
		return err
	}
	targetHosts := lo.FilterMap(httpRoutes, func(item registry.Route, _ int) (string, bool) {
		host := strings.TrimSpace(item.Host)
		return host, item.OwnerID == ownerID && host != ""
	})

	if err := s.registry.DeleteRoutesByOwner(ownerID); err != nil {
		return err
	}
	s.deleteUnusedDNSRecords(targetHosts)
	return nil
}

func (s *Service) deleteUnusedDNSRecords(hosts []string) {
	if s.dnsServer == nil || len(hosts) == 0 || s.registry == nil {
		return
	}

	remaining, err := s.registry.ListRoutes(registry.RouteProtocolHTTP)
	if err != nil {
		s.logger.Warn("list routes for dns cleanup failed", "error", err)
		return
	}
	existing := lo.Reduce(remaining, func(agg map[string]struct{}, item registry.Route, _ int) map[string]struct{} {
		host := strings.TrimSpace(item.Host)
		if host != "" {
			agg[host] = struct{}{}
		}
		return agg
	}, map[string]struct{}{})

	lo.ForEach(lo.Uniq(hosts), func(host string, _ int) {
		if _, ok := existing[host]; ok {
			return
		}
		s.dnsServer.DeleteRecord(host)
	})
}

func (s *Service) syncDNSForRoute(route registry.Route) {
	if s.dnsServer == nil || route.Protocol != registry.RouteProtocolHTTP || !route.Enabled {
		return
	}
	host := strings.TrimSpace(route.Host)
	if host == "" {
		return
	}
	s.dnsServer.SetRecord(host, s.dnsIP)
}

func (s *Service) stopInstanceContainer(ctx context.Context, instance *instanceRecord) error {
	if instance == nil {
		return nil
	}
	workerID := strings.TrimSpace(instance.NodeID)
	if workerID != "" && workerID != s.nodeID {
		return s.stopContainerOnWorker(ctx, workerID, instance.ContainerID)
	}
	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return err
	}
	return exec.Stop(ctx, instance.ContainerID)
}

func (s *Service) InternalRun(ctx context.Context, req InternalRunRequest) (*InternalRunResult, error) {
	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return nil, err
	}

	containerID, err := exec.Run(ctx, req.Spec)
	if err != nil {
		return nil, err
	}
	return &InternalRunResult{
		ContainerID: containerID,
		Driver:      exec.Driver(),
		NodeID:      s.nodeID,
		NodeIP:      s.nodeIP,
	}, nil
}

func (s *Service) InternalStop(ctx context.Context, containerID string) error {
	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return err
	}
	return exec.Stop(ctx, strings.TrimSpace(containerID))
}

func (s *Service) InternalLogs(ctx context.Context, containerID string, tail int) (string, error) {
	exec, err := s.ensureRuntime(ctx)
	if err != nil {
		return "", err
	}
	return exec.Logs(ctx, strings.TrimSpace(containerID), tail)
}

func (s *Service) buildRoutesForTask(
	deploymentID string,
	serviceName string,
	workloadName string,
	unitName string,
	taskSpec dsl.Task,
) []registry.Route {
	labels := collectTaskLabels(taskSpec)
	ports := extractPorts(taskSpec.Network)
	return s.buildRoutesFromLabelMap(deploymentID, serviceName, workloadName, unitName, taskSpec.Name, labels, ports)
}

func (s *Service) buildRoutesFromLabelMap(
	deploymentID string,
	serviceName string,
	workloadName string,
	unitName string,
	taskName string,
	labels map[string]string,
	ports map[string]int,
) []registry.Route {
	now := time.Now()
	routes := make([]registry.Route, 0)

	if parseBoolDefault(labels["warden.ingress.http.enable"], true) {
		httpPort := mustAtoi(labels["warden.ingress.http.port"], 0)
		if httpPort <= 0 {
			if p, ok := ports["http"]; ok {
				httpPort = p
			} else {
				httpPort = mo.TupleToOption(lo.Find(lo.Values(ports), func(port int) bool {
					return port > 0
				})).OrElse(0)
			}
		}
		if httpPort > 0 {
			host := strings.TrimSpace(labels["warden.ingress.http.host"])
			if host == "" {
				host = fmt.Sprintf("%s.warden.local", sanitizeDNSLabel(serviceName))
			}
			pathPrefix := ensurePath(labels["warden.ingress.http.path"])
			routeID := fmt.Sprintf("route:%s:http:%s:%s", deploymentID, strings.ToLower(host), pathPrefix)
			routes = append(routes, registry.Route{
				ID:         routeID,
				OwnerID:    deploymentID,
				Service:    serviceName,
				Protocol:   registry.RouteProtocolHTTP,
				Host:       strings.ToLower(host),
				PathPrefix: pathPrefix,
				TargetPort: httpPort,
				Enabled:    true,
				Source:     "warden-label",
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}

	if listen := mustAtoi(labels["warden.ingress.tcp.listen"], 0); listen > 0 {
		target := mustAtoi(labels["warden.ingress.tcp.port"], 0)
		if target <= 0 {
			target = listen
		}
		routes = append(routes, registry.Route{
			ID:         fmt.Sprintf("route:%s:tcp:%d", deploymentID, listen),
			OwnerID:    deploymentID,
			Service:    serviceName,
			Protocol:   registry.RouteProtocolTCP,
			ListenPort: listen,
			TargetPort: target,
			Enabled:    true,
			Source:     "warden-label",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	if listen := mustAtoi(labels["warden.ingress.udp.listen"], 0); listen > 0 {
		target := mustAtoi(labels["warden.ingress.udp.port"], 0)
		if target <= 0 {
			target = listen
		}
		routes = append(routes, registry.Route{
			ID:         fmt.Sprintf("route:%s:udp:%d", deploymentID, listen),
			OwnerID:    deploymentID,
			Service:    serviceName,
			Protocol:   registry.RouteProtocolUDP,
			ListenPort: listen,
			TargetPort: target,
			Enabled:    true,
			Source:     "warden-label",
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}

	_ = workloadName
	_ = unitName
	_ = taskName
	return uniqRoutes(routes)
}

func collectTaskLabels(taskSpec dsl.Task) map[string]string {
	normalizedLabels := lo.MapEntries(taskSpec.Labels, func(key, value string) (string, string) {
		return strings.TrimSpace(key), strings.TrimSpace(value)
	})

	tagLabels := lo.Reduce(taskSpec.Tags, func(agg map[string]string, tag string, _ int) map[string]string {
		raw := strings.TrimSpace(tag)
		if raw == "" {
			return agg
		}
		parts := strings.SplitN(raw, "=", 2)
		key := strings.TrimSpace(parts[0])
		if len(parts) == 1 {
			agg[key] = "true"
			return agg
		}
		agg[key] = strings.TrimSpace(parts[1])
		return agg
	}, map[string]string{})

	return lo.Assign(normalizedLabels, tagLabels)
}

func uniqRoutes(routes []registry.Route) []registry.Route {
	return lo.UniqBy(routes, func(route registry.Route) string {
		return route.ID
	})
}

func parseBoolDefault(raw string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(raw))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func buildServiceName(workloadName, unitName, taskName string) string {
	return sanitizeDNSLabel(fmt.Sprintf("%s-%s-%s", workloadName, unitName, taskName))
}

func sanitizeDNSLabel(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return "svc"
	}
	builder := strings.Builder{}
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteRune('-')
	}
	out := strings.Trim(builder.String(), "-")
	if out == "" {
		return "svc"
	}
	return out
}

func detectNodeIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	ip := lo.Reduce(addrs, func(found string, addr net.Addr, _ int) string {
		if found != "" {
			return found
		}
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP == nil || ipNet.IP.IsLoopback() {
			return ""
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			return ""
		}
		return ip4.String()
	}, "")
	return mo.TupleToOption(ip, ip != "").OrElse("127.0.0.1")
}
