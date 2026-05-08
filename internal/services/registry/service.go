package registry

import (
	"log/slog"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"
)

type WorkloadRecord struct {
	Name      string    `json:"name"`
	Node      string    `json:"node,omitempty"`
	Runtime   string    `json:"runtime"`
	Artifact  string    `json:"artifact"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Service struct {
	logger *slog.Logger
	items  *mapping.ShardedConcurrentMap[string, WorkloadRecord]
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
		items:  mapping.NewShardedConcurrentMap[string, WorkloadRecord](0, mapping.HashString),
	}
}

func (s *Service) Upsert(record WorkloadRecord) {
	record.UpdatedAt = time.Now()
	s.items.Set(record.Name, record)
	s.logger.Debug("registry upsert", "workload", record.Name, "status", record.Status)
}

func (s *Service) Delete(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	s.items.Delete(name)
	s.logger.Debug("registry delete", "workload", name)
}

func (s *Service) List() *list.List[WorkloadRecord] {
	out := list.NewListWithCapacity[WorkloadRecord](s.items.Len())
	s.items.Range(func(_ string, record WorkloadRecord) bool {
		out.Add(record)
		return true
	})
	out.Sort(func(a, b WorkloadRecord) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}
