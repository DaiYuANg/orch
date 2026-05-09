package dixdiag

import (
	"context"
	"errors"
	"fmt"
	"sync"

	collectionmapping "github.com/arcgolabs/collectionx/mapping"
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/raftsvc"
)

var ErrRuntimeUnavailable = errors.New("dix runtime is not attached")

type Service struct {
	mu sync.RWMutex
	rt *dix.Runtime
}

func New() *Service {
	return &Service{}
}

func (s *Service) Attach(rt *dix.Runtime) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rt = rt
}

func (s *Service) Runtime() *dix.Runtime {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rt
}

func (s *Service) CheckHealth(ctx context.Context) dix.HealthReport {
	if rt := s.Runtime(); rt != nil {
		return rt.CheckHealth(ctx)
	}
	return unavailableReport(dix.HealthKindGeneral)
}

func (s *Service) CheckReadiness(ctx context.Context) dix.HealthReport {
	if rt := s.Runtime(); rt != nil {
		return rt.CheckReadiness(ctx)
	}
	return unavailableReport(dix.HealthKindReadiness)
}

func CheckControlPlaneReady(ctx context.Context, cfg config.Config, raft *raftsvc.Service) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if raft == nil {
		return errors.New("raft service unavailable")
	}
	status, err := raft.Status(ctx)
	if err != nil {
		return fmt.Errorf("raft status: %w", err)
	}
	if !status.Ready || status.LeaderID == "" {
		detail := status.Message
		if detail == "" {
			detail = status.State
		}
		return fmt.Errorf("raft not ready: %s", detail)
	}
	if status.IsLeader {
		return nil
	}
	if _, ok := cfg.Cluster.NodeURL(status.LeaderID); ok {
		return nil
	}
	return fmt.Errorf("writes require cluster.nodes.%s or the leader API", status.LeaderID)
}

func unavailableReport(kind dix.HealthKind) dix.HealthReport {
	checks := collectionmapping.NewMap[string, error]()
	checks.Set("runtime", ErrRuntimeUnavailable)
	return dix.HealthReport{Kind: kind, Checks: checks}
}
