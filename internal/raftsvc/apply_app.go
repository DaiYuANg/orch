package raftsvc

import (
	"encoding/json"
	"time"

	"github.com/arcgolabs/collectionx/list"
	hraft "github.com/hashicorp/raft"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/pkg/oopsx"
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

// ApplyDeployApp replicates a validated [deployv1.App] through Raft when enabled; the local FSM is updated on every peer after commit.
// Callers must target the Raft leader when Raft is enabled.
func (s *Service) ApplyDeployApp(app deployv1.App) error {
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

	if !s.cfg.Raft.Enabled || s.r == nil {
		s.fsm.applyCommandPayload(b)
		return nil
	}
	if s.r.State() != hraft.Leader {
		return oopsx.B("raft").Errorf("not leader: send deploy to the raft leader node")
	}
	return s.r.Apply(b, 30*time.Second).Error()
}
