package task

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/goccy/go-json"
	"github.com/samber/lo"
)

const taskAssignmentBucket = "task_assignments"

type schedulingAssignment struct {
	DeploymentID string    `json:"deployment_id"`
	Workload     string    `json:"workload"`
	DesiredNode  string    `json:"desired_node"`
	WorkerNode   string    `json:"worker_node"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (s *Service) ensureLeaderForScheduling() error {
	if s.raft == nil || !s.raft.Enabled() {
		return nil
	}
	if !s.raft.IsLeader() {
		return fmt.Errorf("%w: current leader is %s", raft.ErrNotLeader, s.raft.Leader())
	}
	return nil
}

func (s *Service) upsertSchedulingAssignment(deploymentID, workload string) (schedulingAssignment, error) {
	if s.raft == nil || !s.raft.Enabled() {
		now := time.Now()
		return schedulingAssignment{
			DeploymentID: deploymentID,
			Workload:     workload,
			DesiredNode:  s.nodeID,
			WorkerNode:   s.nodeID,
			CreatedAt:    now,
			UpdatedAt:    now,
		}, nil
	}
	desired, worker := s.selectWorkerNode(deploymentID)
	now := time.Now()
	record := schedulingAssignment{
		DeploymentID: deploymentID,
		Workload:     workload,
		DesiredNode:  desired,
		WorkerNode:   worker,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return schedulingAssignment{}, err
	}
	if err := s.raft.ApplySet(taskAssignmentBucket, deploymentID, raw); err != nil {
		return schedulingAssignment{}, fmt.Errorf("set scheduling assignment: %w", err)
	}

	// Prefer badger hot cache via raft.Read; fallback remains FSM bbolt.
	if _, err := s.raft.Read(taskAssignmentBucket, deploymentID); err != nil {
		return schedulingAssignment{}, fmt.Errorf("verify scheduling assignment cache: %w", err)
	}
	return record, nil
}

func (s *Service) selectWorkerNode(deploymentID string) (desired string, worker string) {
	if s.raft == nil || !s.raft.Enabled() {
		return s.nodeID, s.nodeID
	}
	servers, err := s.raft.ListServers()
	if err != nil || len(servers) == 0 {
		return s.nodeID, s.nodeID
	}

	ids := lo.Map(servers, func(item raft.Server, _ int) string {
		return strings.TrimSpace(item.ID)
	})
	ids = lo.Filter(ids, func(item string, _ int) bool {
		return item != ""
	})
	if len(ids) == 0 {
		return s.nodeID, s.nodeID
	}

	chosen := ids[s.hashToIndex(deploymentID, len(ids))]
	if chosen == s.nodeID {
		return chosen, chosen
	}

	if s.resolveWorkerAPI(chosen).IsAbsent() {
		// Keep leader-as-worker baseline when remote worker endpoint is not configured.
		s.logger.Info("remote worker has no api mapping, fallback to leader worker mode", "desired_node", chosen, "worker_node", s.nodeID)
		return chosen, s.nodeID
	}
	return chosen, chosen
}

func (s *Service) hashToIndex(input string, size int) int {
	if size <= 1 {
		return 0
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(input))
	return int(hasher.Sum32() % uint32(size))
}

func (s *Service) deleteSchedulingAssignment(deploymentID string) error {
	if s.raft == nil || !s.raft.Enabled() {
		return nil
	}
	if err := s.raft.ApplyDelete(taskAssignmentBucket, deploymentID); err != nil {
		return fmt.Errorf("delete scheduling assignment: %w", err)
	}
	return nil
}
