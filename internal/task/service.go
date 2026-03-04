package task

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DaiYuANg/warden/internal/dsl"
	"github.com/DaiYuANg/warden/internal/registry"
	dockerrt "github.com/DaiYuANg/warden/internal/runtime_engine/docker"
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
	labelPortPrefix   = "warden.port."
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
	docker      *dockerrt.Executor
	registry    *registry.Service
	nodeID      string
	nodeIP      string
	deployments map[string]*deploymentRecord
	instances   map[string]*instanceRecord

	reconcileInterval time.Duration
	stopCh            chan struct{}
	stopped           chan struct{}
}

func NewService(logger *slog.Logger, registryService *registry.Service) *Service {
	nodeID, _ := pkg.MachineID()
	nodeIP := detectNodeIP()
	if nodeID == "" {
		nodeID = uuid.NewString()
	}
	if nodeIP == "" {
		nodeIP = "127.0.0.1"
	}
	return &Service{
		logger:            logger,
		registry:          registryService,
		nodeID:            nodeID,
		nodeIP:            nodeIP,
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
	if _, err := s.ensureDocker(ctx); err != nil {
		s.logger.Warn("docker runtime unavailable at startup", "error", err)
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

	exec, err := s.ensureDocker(ctx)
	if err != nil {
		return nil, err
	}

	deploymentID := uuid.NewString()
	now := time.Now()
	deployment := &deploymentRecord{
		DeploymentInfo: DeploymentInfo{
			ID:        deploymentID,
			Workload:  workload.Name,
			Format:    format,
			Status:    DeploymentStatusRunning,
			CreatedAt: now,
			UpdatedAt: now,
		},
		instances: make(map[string]*instanceRecord),
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
				deployment.RouteIDs = append(deployment.RouteIDs, route.ID)
			}

			replicas := taskSpec.Replicas
			if replicas <= 0 {
				replicas = 1
			}
			for replica := 0; replica < replicas; replica++ {
				instance := s.newInstanceRecord(deploymentID, serviceName, workload.Name, unit.Name, taskSpec, replica)
				containerID, runErr := exec.RunContainer(ctx, instance.RunSpec)
				if runErr != nil {
					for _, item := range created {
						_ = exec.StopAndRemove(ctx, item.ContainerID)
					}
					_ = s.deleteRoutesByOwner(deploymentID)
					return nil, fmt.Errorf("start docker workload %s/%s/%s[%d]: %w", workload.Name, unit.Name, taskSpec.Name, replica, runErr)
				}
				instance.ContainerID = containerID
				instance.Status = InstanceStatusRunning
				instance.UpdatedAt = time.Now()
				if err := s.upsertRegistryEndpoint(instance, true); err != nil {
					_ = exec.StopAndRemove(ctx, instance.ContainerID)
					for _, item := range created {
						_ = exec.StopAndRemove(ctx, item.ContainerID)
					}
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
	exec, err := s.ensureDocker(ctx)
	if err != nil {
		return err
	}

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
		if stopErr := exec.StopAndRemove(ctx, item.ContainerID); stopErr != nil && firstErr == nil {
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

	return firstErr
}

func (s *Service) Logs(ctx context.Context, instanceID string, tail int) (string, error) {
	exec, err := s.ensureDocker(ctx)
	if err != nil {
		return "", err
	}

	s.mu.RLock()
	instance, ok := s.instances[instanceID]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("instance not found: %s", instanceID)
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exec, err := s.ensureDocker(ctx)
	if err != nil {
		s.markInstanceFailed(instanceID, fmt.Sprintf("docker unavailable: %v", err))
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

	exec, err := s.ensureDocker(ctx)
	if err != nil {
		s.markInstanceFailed(instanceID, fmt.Sprintf("docker unavailable for restart: %v", err))
		return
	}

	_ = exec.StopAndRemove(ctx, snapshot.ContainerID)
	newContainerID, err := exec.RunContainer(ctx, snapshot.RunSpec)
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
	exec, err := s.ensureDocker(ctx)
	if err != nil {
		return err
	}

	items, err := exec.ListContainers(ctx, true, map[string][]string{
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
		instance := &instanceRecord{
			InstanceInfo: InstanceInfo{
				ID:           instanceID,
				DeploymentID: deploymentID,
				Service:      serviceName,
				Workload:     c.Labels[labelWorkload],
				Unit:         c.Labels[labelUnit],
				Task:         c.Labels[labelTask],
				Replica:      replica,
				Driver:       "docker",
				ContainerID:  c.ID,
				Status:       InstanceStatusRunning,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			RunSpec: dockerrt.RunSpec{
				Name:   trimContainerName(c.Names),
				Image:  c.Image,
				Labels: c.Labels,
				Ports:  labelsToPorts(c.Labels),
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

func (s *Service) ensureDocker(ctx context.Context) (*dockerrt.Executor, error) {
	s.mu.RLock()
	exec := s.docker
	s.mu.RUnlock()
	if exec != nil {
		return exec, exec.Ping(ctx)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.docker != nil {
		return s.docker, s.docker.Ping(ctx)
	}
	dockerExecutor, err := dockerrt.NewExecutor()
	if err != nil {
		return nil, err
	}
	if err := dockerExecutor.Ping(ctx); err != nil {
		return nil, err
	}
	s.docker = dockerExecutor
	return s.docker, nil
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

func (s *Service) newInstanceRecord(deploymentID, serviceName, workloadName, unitName string, taskSpec dsl.Task, replica int) *instanceRecord {
	instanceID := uuid.NewString()
	check := buildHealthCheck(taskSpec)
	labels := buildLabels(deploymentID, serviceName, workloadName, unitName, taskSpec.Name, instanceID, replica, check, taskSpec.Network)
	for key, value := range collectTaskLabels(taskSpec) {
		labels[key] = value
	}

	spec := dockerrt.RunSpec{
		Name:   sanitizeContainerName(fmt.Sprintf("warden-%s-%s-%s-%d", workloadName, unitName, taskSpec.Name, replica)),
		Image:  taskSpec.Image,
		Cmd:    taskSpec.Command,
		Env:    taskSpec.Env,
		Labels: labels,
		Ports:  extractPorts(taskSpec.Network),
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
			Driver:       "docker",
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
	serviceName string,
	workloadName string,
	unitName string,
	taskName string,
	instanceID string,
	replica int,
	check healthCheckSpec,
	network *dsl.NetworkConfig,
) map[string]string {
	labels := map[string]string{
		labelManaged:      "true",
		labelDeploymentID: deploymentID,
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
	endpoint := registry.ServiceEndpoint{
		ID:       instance.ID,
		Service:  instance.Service,
		NodeID:   s.nodeID,
		NodeIP:   s.nodeIP,
		Runtime:  "docker",
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
	return s.registry.DeleteRoutesByOwner(ownerID)
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
				host = fmt.Sprintf("%s.%s.warden.local", sanitizeDNSLabel(taskName), sanitizeDNSLabel(workloadName))
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

	_ = unitName
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
