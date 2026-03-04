package task

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	internalraft "github.com/DaiYuANg/warden/internal/raft"
	"github.com/samber/lo"
)

func (s *Service) MigrateDeployment(ctx context.Context, deploymentID string, req MigrateDeploymentRequest) (*MigrateDeploymentResult, error) {
	if err := s.ensureLeaderForScheduling(); err != nil {
		return nil, err
	}

	targetNode := strings.TrimSpace(req.TargetNode)
	snapshot, ok := s.snapshotDeploymentForMigration(deploymentID)
	if !ok {
		return nil, fmt.Errorf("deployment not found: %s", deploymentID)
	}
	if err := validateMigrationSafety(snapshot, req); err != nil {
		return nil, err
	}
	if targetNode == "" {
		targetNode = s.pickExecutionWorker(snapshot.Deployment.ID, map[string]struct{}{
			snapshot.Deployment.WorkerNode: {},
		})
	}
	if targetNode == "" {
		targetNode = s.nodeID
	}
	if targetNode != s.nodeID && s.resolveWorkerAPI(targetNode).IsAbsent() {
		return nil, fmt.Errorf("target node %q is not executable (missing raft.node_api mapping)", targetNode)
	}

	fromNode := snapshot.Deployment.WorkerNode
	if fromNode == "" {
		fromNode = lo.Ternary(len(snapshot.Instances) > 0, snapshot.Instances[0].NodeID, s.nodeID)
	}
	result := &MigrateDeploymentResult{
		DeploymentID: snapshot.Deployment.ID,
		Workload:     snapshot.Deployment.Workload,
		FromNode:     fromNode,
		ToNode:       targetNode,
		Instances:    len(snapshot.Instances),
		Migrated:     0,
	}
	if fromNode == targetNode {
		return result, nil
	}

	for _, instance := range snapshot.Instances {
		migrated, err := s.migrateInstance(ctx, snapshot.Deployment, instance, targetNode)
		if err != nil {
			return nil, err
		}
		if migrated {
			result.Migrated++
		}
	}

	if err := s.setDeploymentWorkerNode(snapshot.Deployment.ID, targetNode); err != nil {
		return nil, err
	}
	if _, err := s.setSchedulingAssignment(snapshot.Deployment.ID, snapshot.Deployment.Workload, targetNode, targetNode); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Failover(ctx context.Context, req FailoverRequest) (*FailoverResult, error) {
	if err := s.ensureLeaderForScheduling(); err != nil {
		return nil, err
	}
	failed := strings.TrimSpace(req.FailedNode)
	if failed == "" {
		return nil, fmt.Errorf("failed_node is required")
	}

	candidates := s.listDeploymentsByWorker(failed)
	result := &FailoverResult{
		FailedNode:  failed,
		TargetNode:  strings.TrimSpace(req.TargetNode),
		Deployments: len(candidates),
		Migrations:  make([]MigrateDeploymentResult, 0, len(candidates)),
	}

	for _, deployment := range candidates {
		target := result.TargetNode
		if target == "" {
			target = s.pickExecutionWorker(deployment.ID, map[string]struct{}{
				failed: {},
			})
		}
		migration, err := s.MigrateDeployment(ctx, deployment.ID, MigrateDeploymentRequest{
			TargetNode:     target,
			ForceStateful:  req.ForceStateful,
			MaxUnavailable: req.MaxUnavailable,
		})
		if err != nil {
			return nil, err
		}
		result.Migrations = append(result.Migrations, *migration)
	}
	return result, nil
}

func (s *Service) Rebalance(ctx context.Context, req RebalanceRequest) (*RebalanceResult, error) {
	if err := s.ensureLeaderForScheduling(); err != nil {
		return nil, err
	}

	workers := s.listExecutableWorkers()
	result := &RebalanceResult{
		Workers: workers,
	}
	if len(workers) <= 1 {
		return result, nil
	}

	candidates := s.listDeploymentsForRebalance()
	result.Candidates = len(candidates)
	maxMigrations := req.MaxMigrations
	if maxMigrations <= 0 || maxMigrations > len(candidates) {
		maxMigrations = len(candidates)
	}

	for _, deployment := range candidates {
		if result.Applied >= maxMigrations {
			break
		}
		target := workers[s.hashToIndex(deployment.ID, len(workers))]
		if strings.TrimSpace(deployment.WorkerNode) == target {
			continue
		}

		migration, err := s.MigrateDeployment(ctx, deployment.ID, MigrateDeploymentRequest{
			TargetNode:     target,
			ForceStateful:  req.ForceStateful,
			MaxUnavailable: req.MaxUnavailable,
		})
		if err != nil {
			return nil, err
		}
		result.Migrations = append(result.Migrations, *migration)
		result.Applied++
	}
	return result, nil
}

type deploymentMigrationSnapshot struct {
	Deployment DeploymentInfo
	Instances  []instanceRecord
}

func validateMigrationSafety(snapshot deploymentMigrationSnapshot, req MigrateDeploymentRequest) error {
	stateful := lo.ContainsBy(snapshot.Instances, func(item instanceRecord) bool {
		return item.Stateful
	})
	if !stateful {
		return nil
	}
	if !req.ForceStateful {
		return fmt.Errorf("deployment %s is stateful; set force_stateful=true to confirm migration", snapshot.Deployment.ID)
	}
	maxUnavailable := req.MaxUnavailable
	if maxUnavailable <= 0 {
		maxUnavailable = 1
	}
	if maxUnavailable != 1 {
		return fmt.Errorf("stateful migration currently supports max_unavailable=1 only")
	}
	return nil
}

func (s *Service) snapshotDeploymentForMigration(deploymentID string) (deploymentMigrationSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	deployment, ok := s.deployments[deploymentID]
	if !ok {
		return deploymentMigrationSnapshot{}, false
	}
	instances := lo.Map(lo.Values(deployment.instances), func(item *instanceRecord, _ int) instanceRecord {
		return *item
	})
	return deploymentMigrationSnapshot{
		Deployment: deployment.DeploymentInfo,
		Instances:  instances,
	}, true
}

func (s *Service) migrateInstance(ctx context.Context, deployment DeploymentInfo, snapshot instanceRecord, targetNode string) (bool, error) {
	if strings.TrimSpace(snapshot.NodeID) == targetNode {
		return false, nil
	}

	newRuntime, err := s.runInstanceOnNode(ctx, snapshot, targetNode)
	if err != nil {
		return false, err
	}

	stopSource := snapshot
	if err := s.stopInstanceContainer(ctx, &stopSource); err != nil {
		if targetNode == s.nodeID {
			exec, runtimeErr := s.ensureRuntime(ctx, lo.Ternary(newRuntime.Driver != "", newRuntime.Driver, snapshot.Driver))
			if runtimeErr == nil {
				_ = exec.Stop(ctx, newRuntime.ContainerID)
			}
		} else {
			_ = s.stopContainerOnWorker(ctx, targetNode, InternalStopRequest{
				ContainerID: newRuntime.ContainerID,
				Driver:      lo.Ternary(newRuntime.Driver != "", newRuntime.Driver, snapshot.Driver),
			})
		}
		return false, err
	}

	if err := s.commitMigratedInstance(deployment.ID, snapshot.ID, targetNode, newRuntime); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Service) runInstanceOnNode(ctx context.Context, instance instanceRecord, targetNode string) (InternalRunResult, error) {
	if targetNode == s.nodeID {
		exec, err := s.ensureRuntime(ctx, instance.Driver)
		if err != nil {
			return InternalRunResult{}, err
		}
		containerID, err := exec.Run(ctx, instance.RunSpec)
		if err != nil {
			return InternalRunResult{}, err
		}
		return InternalRunResult{
			ContainerID: containerID,
			Driver:      exec.Driver(),
			NodeID:      s.nodeID,
			NodeIP:      s.nodeIP,
		}, nil
	}
	return s.runContainerOnWorker(ctx, targetNode, InternalRunRequest{
		Driver: instance.Driver,
		Spec:   instance.RunSpec,
	})
}

func (s *Service) commitMigratedInstance(deploymentID, instanceID, targetNode string, runtime InternalRunResult) error {
	s.mu.Lock()
	deployment, ok := s.deployments[deploymentID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}
	instance, ok := deployment.instances[instanceID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("instance not found: %s", instanceID)
	}

	instance.ContainerID = runtime.ContainerID
	instance.Status = InstanceStatusRunning
	instance.LastError = ""
	instance.ConsecutiveFailed = 0
	instance.UpdatedAt = time.Now()
	instance.NodeID = lo.Ternary(strings.TrimSpace(runtime.NodeID) != "", strings.TrimSpace(runtime.NodeID), targetNode)
	if strings.TrimSpace(runtime.NodeIP) != "" {
		instance.NodeIP = strings.TrimSpace(runtime.NodeIP)
	}
	if strings.TrimSpace(runtime.Driver) != "" {
		instance.Driver = strings.TrimSpace(runtime.Driver)
	}
	deployment.WorkerNode = targetNode
	deployment.DesiredNode = targetNode
	deployment.UpdatedAt = time.Now()
	s.mu.Unlock()

	refreshed, ok := s.getInstance(instanceID)
	if !ok {
		return fmt.Errorf("instance disappeared after migration: %s", instanceID)
	}
	return s.upsertRegistryEndpoint(refreshed, true)
}

func (s *Service) setDeploymentWorkerNode(deploymentID, workerNode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deployment, ok := s.deployments[deploymentID]
	if !ok {
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}
	deployment.WorkerNode = workerNode
	deployment.DesiredNode = workerNode
	deployment.UpdatedAt = time.Now()
	return nil
}

func (s *Service) listExecutableWorkers() []string {
	workers := []string{s.nodeID}
	if s.raft == nil || !s.raft.Enabled() {
		workers = append(workers, lo.Keys(s.nodeAPI)...)
		workers = lo.FilterMap(workers, func(item string, _ int) (string, bool) {
			trimmed := strings.TrimSpace(item)
			return trimmed, trimmed != ""
		})
		workers = lo.Uniq(workers)
		sort.Strings(workers)
		return workers
	}

	servers, err := s.raft.ListServers()
	if err != nil {
		return []string{s.nodeID}
	}
	workers = lo.FilterMap(servers, func(item internalraft.Server, _ int) (string, bool) {
		nodeID := strings.TrimSpace(item.ID)
		if nodeID == "" {
			return "", false
		}
		if nodeID == s.nodeID || s.resolveWorkerAPI(nodeID).IsPresent() {
			return nodeID, true
		}
		return "", false
	})

	workers = lo.Uniq(workers)
	sort.Strings(workers)
	if len(workers) == 0 {
		return []string{s.nodeID}
	}
	return workers
}

func (s *Service) pickExecutionWorker(seed string, excludes map[string]struct{}) string {
	workers := lo.Filter(s.listExecutableWorkers(), func(item string, _ int) bool {
		_, skip := excludes[item]
		return !skip
	})
	if len(workers) == 0 {
		return s.nodeID
	}
	return workers[s.hashToIndex(seed, len(workers))]
}

func (s *Service) listDeploymentsByWorker(workerNode string) []DeploymentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return lo.Map(
		lo.Filter(lo.Values(s.deployments), func(item *deploymentRecord, _ int) bool {
			if strings.TrimSpace(item.WorkerNode) == strings.TrimSpace(workerNode) {
				return true
			}
			return lo.ContainsBy(lo.Values(item.instances), func(instance *instanceRecord) bool {
				return strings.TrimSpace(instance.NodeID) == strings.TrimSpace(workerNode)
			})
		}),
		func(item *deploymentRecord, _ int) DeploymentInfo { return item.DeploymentInfo },
	)
}

func (s *Service) listDeploymentsForRebalance() []DeploymentInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := lo.FilterMap(lo.Values(s.deployments), func(item *deploymentRecord, _ int) (DeploymentInfo, bool) {
		if item.Status != DeploymentStatusRunning {
			return DeploymentInfo{}, false
		}
		if len(item.instances) == 0 {
			return DeploymentInfo{}, false
		}
		return item.DeploymentInfo, true
	})
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}
