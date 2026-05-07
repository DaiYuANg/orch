package task

import "github.com/daiyuang/orch/internal/workloadmeta"

// ListWorkloadAssignments returns scheduler assignment state replicated through Raft.
func (s *Service) ListWorkloadAssignments() []workloadmeta.Assignment {
	if s == nil || s.raft == nil {
		return nil
	}
	return s.raft.ListWorkloadAssignments()
}
