package registry

import (
	"log/slog"
	"slices"
	"strings"
	"time"

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

func (s *Service) List() []WorkloadRecord {
	out := s.items.Values()
	slices.SortFunc(out, func(a, b WorkloadRecord) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}
