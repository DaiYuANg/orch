package registry

import (
	"log/slog"
	"time"

	"github.com/arcgolabs/collectionx/mapping"
)

type WorkloadRecord struct {
	Name      string    `json:"name"`
	Runtime   string    `json:"runtime"`
	Image     string    `json:"image"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Service struct {
	logger *slog.Logger
	items  *mapping.ConcurrentMap[string, WorkloadRecord]
}

func NewService(logger *slog.Logger) *Service {
	return &Service{
		logger: logger,
		items:  mapping.NewConcurrentMap[string, WorkloadRecord](),
	}
}

func (s *Service) Upsert(record WorkloadRecord) {
	record.UpdatedAt = time.Now()
	s.items.Set(record.Name, record)
	s.logger.Debug("registry upsert", "workload", record.Name, "status", record.Status)
}

func (s *Service) List() []WorkloadRecord {
	return s.items.Values()
}
