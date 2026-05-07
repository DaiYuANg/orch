package task

import (
	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/internal/workloadmeta"
)

// ListWorkloadAssignments returns scheduler assignment state replicated through Raft.
func (s *Service) ListWorkloadAssignments() *list.List[workloadmeta.Assignment] {
	if s == nil || s.raft == nil {
		return list.NewList[workloadmeta.Assignment]()
	}
	return s.raft.ListWorkloadAssignments()
}
