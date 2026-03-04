package task

import (
	"fmt"
	"time"

	"github.com/DaiYuANg/warden/internal/raft"
	"github.com/goccy/go-json"
)

const taskAssignmentBucket = "task_assignments"

type schedulingAssignment struct {
	DeploymentID string    `json:"deployment_id"`
	Workload     string    `json:"workload"`
	NodeID       string    `json:"node_id"`
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

func (s *Service) upsertSchedulingAssignment(deploymentID, workload string) error {
	if s.raft == nil || !s.raft.Enabled() {
		return nil
	}
	now := time.Now()
	record := schedulingAssignment{
		DeploymentID: deploymentID,
		Workload:     workload,
		NodeID:       s.nodeID,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := s.raft.ApplySet(taskAssignmentBucket, deploymentID, raw); err != nil {
		return fmt.Errorf("set scheduling assignment: %w", err)
	}

	// Prefer badger hot cache via raft.Read; fallback remains FSM bbolt.
	if _, err := s.raft.Read(taskAssignmentBucket, deploymentID); err != nil {
		return fmt.Errorf("verify scheduling assignment cache: %w", err)
	}
	return nil
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
