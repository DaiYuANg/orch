package runtime

import (
	"context"
	"log/slog"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/mapping"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type Provider interface {
	Kind() deployv1.RuntimeKind
	Deploy(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload) error
	Stop(ctx context.Context, meta deployv1.Metadata, workloadName string) error
}

type Status = runtimeinfo.Status
type LogOptions = runtimeinfo.LogOptions
type LogResult = runtimeinfo.LogResult

type StatusProvider interface {
	Status(ctx context.Context, meta deployv1.Metadata, workloadName string) (Status, error)
}

type LogsProvider interface {
	Logs(ctx context.Context, meta deployv1.Metadata, workloadName string, opts LogOptions) (LogResult, error)
}

type ProviderPolicy string

const (
	ProviderPolicyAuto     ProviderPolicy = "auto"
	ProviderPolicyRequired ProviderPolicy = "required"
	ProviderPolicyDisabled ProviderPolicy = "disabled"
)

const (
	ProviderStatusRegistered = "registered"
	ProviderStatusDisabled   = "disabled"
	ProviderStatusMissing    = "missing"
)

type ProviderStatus struct {
	Kind       deployv1.RuntimeKind `json:"kind"`
	Policy     ProviderPolicy       `json:"policy"`
	Available  bool                 `json:"available"`
	Registered bool                 `json:"registered"`
	Status     string               `json:"status"`
	Reason     string               `json:"reason,omitempty"`
}

type Manager struct {
	logger           *slog.Logger
	providers        *mapping.ConcurrentMap[deployv1.RuntimeKind, Provider]
	providerStatuses *list.List[ProviderStatus]
}

func NewManager(logger *slog.Logger, providers ...Provider) *Manager {
	return NewManagerWithStatus(logger, list.NewList[ProviderStatus](), providers...)
}

func NewManagerWithStatus(logger *slog.Logger, statuses *list.List[ProviderStatus], providers ...Provider) *Manager {
	idx := mapping.NewConcurrentMapWithCapacity[deployv1.RuntimeKind, Provider](len(providers))
	for _, p := range providers {
		if p == nil {
			continue
		}
		idx.Set(p.Kind(), p)
	}
	return &Manager{
		logger:           logger,
		providers:        idx,
		providerStatuses: statuses,
	}
}

func (m *Manager) HasProvider(kind deployv1.RuntimeKind) bool {
	if m == nil || m.providers == nil {
		return false
	}
	_, ok := m.providers.Get(kind)
	return ok
}

func (m *Manager) RegisteredKinds() *list.List[deployv1.RuntimeKind] {
	if m == nil || m.providers == nil {
		return list.NewList[deployv1.RuntimeKind]()
	}
	kinds := list.NewList(m.providers.Keys()...)
	kinds.Sort(func(a, b deployv1.RuntimeKind) int {
		return strings.Compare(string(a), string(b))
	})
	return kinds
}

func (m *Manager) ProviderStatuses() *list.List[ProviderStatus] {
	if m == nil {
		return list.NewList[ProviderStatus]()
	}
	if m.providerStatuses != nil && !m.providerStatuses.IsEmpty() {
		return sortProviderStatuses(m.providerStatuses.Clone())
	}
	out := list.NewList[ProviderStatus]()
	if m.providers == nil {
		return out
	}
	kinds := m.RegisteredKinds()
	kinds.Range(func(_ int, kind deployv1.RuntimeKind) bool {
		out.Add(ProviderStatus{
			Kind:       kind,
			Policy:     ProviderPolicyAuto,
			Available:  true,
			Registered: true,
			Status:     ProviderStatusRegistered,
		})
		return true
	})
	return out
}

func sortProviderStatuses(statuses *list.List[ProviderStatus]) *list.List[ProviderStatus] {
	statuses.Sort(func(a, b ProviderStatus) int {
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
	return statuses
}

func (m *Manager) Deploy(ctx context.Context, meta deployv1.Metadata, workload deployv1.Workload) error {
	p, ok := m.providers.Get(workload.Runtime)
	if !ok {
		return oopsx.B("runtime").Errorf("runtime provider not registered: %s", workload.Runtime)
	}
	m.logger.Info("deploy workload", "workload", workload.Name, "runtime", workload.Runtime)
	if err := p.Deploy(ctx, meta, workload); err != nil {
		return oopsx.B("runtime").Wrapf(err, "deploy workload %s", workload.Name)
	}
	return nil
}

func (m *Manager) Stop(ctx context.Context, runtime deployv1.RuntimeKind, meta deployv1.Metadata, workloadName string) error {
	p, ok := m.providers.Get(runtime)
	if !ok {
		return oopsx.B("runtime").Errorf("runtime provider not registered: %s", runtime)
	}
	m.logger.Info("stop workload", "workload", workloadName, "runtime", runtime)
	if err := p.Stop(ctx, meta, workloadName); err != nil {
		return oopsx.B("runtime").Wrapf(err, "stop workload %s", workloadName)
	}
	return nil
}

func (m *Manager) Status(ctx context.Context, runtime deployv1.RuntimeKind, meta deployv1.Metadata, workloadName string) (Status, error) {
	p, ok := m.providers.Get(runtime)
	if !ok {
		return Status{}, oopsx.B("runtime").Errorf("runtime provider not registered: %s", runtime)
	}
	statusProvider, ok := p.(StatusProvider)
	if !ok {
		return Status{
			Name:    strings.TrimSpace(workloadName),
			Runtime: runtime,
			Status:  "unknown",
			Message: "runtime status is not implemented for provider " + string(runtime),
		}, nil
	}
	st, err := statusProvider.Status(ctx, meta, workloadName)
	if err != nil {
		return Status{}, oopsx.B("runtime").Wrapf(err, "status workload %s", workloadName)
	}
	if st.Name == "" {
		st.Name = strings.TrimSpace(workloadName)
	}
	if st.Runtime == "" {
		st.Runtime = runtime
	}
	if st.Status == "" {
		st.Status = "unknown"
	}
	return st, nil
}

func (m *Manager) Logs(ctx context.Context, runtime deployv1.RuntimeKind, meta deployv1.Metadata, workloadName string, opts LogOptions) (LogResult, error) {
	p, ok := m.providers.Get(runtime)
	if !ok {
		return LogResult{}, oopsx.B("runtime").Errorf("runtime provider not registered: %s", runtime)
	}
	logsProvider, ok := p.(LogsProvider)
	if !ok {
		return LogResult{}, oopsx.B("runtime").Errorf("runtime logs are not implemented for provider %s", runtime)
	}
	out, err := logsProvider.Logs(ctx, meta, workloadName, opts)
	if err != nil {
		return LogResult{}, oopsx.B("runtime").Wrapf(err, "logs workload %s", workloadName)
	}
	if out.Name == "" {
		out.Name = strings.TrimSpace(workloadName)
	}
	if out.Runtime == "" {
		out.Runtime = runtime
	}
	return out, nil
}
