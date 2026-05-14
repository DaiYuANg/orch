package raftsvc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// DeployReconcileSignals returns a coalesced notification channel: one signal may represent multiple FSM applies.
func (s *Service) DeployReconcileSignals() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.deploySignalCh
}

// ListDesiredDeployApps returns a snapshot of replicated desired App documents (latest per metadata key).
func (s *Service) ListDesiredDeployApps() *list.List[deployv1.App] {
	if s == nil || s.fsm == nil {
		return list.NewList[deployv1.App]()
	}
	return s.fsm.listDeployApps()
}

func (s *Service) GetDesiredDeployApp(meta deployv1.Metadata) (deployv1.App, bool) {
	if s == nil || s.fsm == nil {
		return deployv1.App{}, false
	}
	return s.fsm.getDeployApp(meta)
}

// ApplyDeployApp replicates a validated [deployv1.App] through Raft; the local FSM is updated on every peer after commit.
// Callers must target the Raft leader.
func (s *Service) ApplyDeployApp(ctx context.Context, app deployv1.App) error {
	if s == nil {
		return oopsx.B("raft").Errorf("nil service")
	}
	b, err := json.Marshal(struct {
		Type string       `json:"type"`
		App  deployv1.App `json:"app"`
	}{
		Type: cmdUpsertDeployApp,
		App:  app,
	})
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "marshal deploy app command")
	}

	return s.applyCommand(ctx, b, 30*time.Second, "not leader: send deploy to the raft leader node")
}

func (s *Service) ApplyDeleteDeployApp(ctx context.Context, meta deployv1.Metadata) error {
	if s == nil {
		return oopsx.B("raft").Errorf("nil service")
	}
	if meta.Name == "" {
		return oopsx.B("raft").Errorf("metadata.name is required")
	}
	b, err := json.Marshal(struct {
		Type     string            `json:"type"`
		Metadata deployv1.Metadata `json:"metadata"`
	}{
		Type:     cmdDeleteDeployApp,
		Metadata: meta,
	})
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "marshal delete deploy app command")
	}
	return s.applyCommand(ctx, b, 30*time.Second, "not leader: send delete to the raft leader node")
}
