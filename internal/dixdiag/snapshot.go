package dixdiag

import (
	"fmt"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
)

const maxRecentEvents = 50

type Snapshot struct {
	App       RuntimeSummary    `json:"app"`
	Lifecycle LifecycleSnapshot `json:"lifecycle"`
	Events    EventSnapshot     `json:"events"`
	Graph     GraphSnapshot     `json:"graph"`
}

type RuntimeSummary struct {
	Name            string  `json:"name"`
	Version         string  `json:"version,omitempty"`
	Profile         string  `json:"profile"`
	State           string  `json:"state"`
	BuildDuration   string  `json:"buildDuration,omitempty"`
	BuildDurationMS float64 `json:"buildDurationMs,omitempty"`
	StartDuration   string  `json:"startDuration,omitempty"`
	StartDurationMS float64 `json:"startDurationMs,omitempty"`
}

type LifecycleSnapshot struct {
	StartHooks  int                  `json:"startHooks"`
	StopHooks   int                  `json:"stopHooks"`
	Concurrency int                  `json:"concurrency"`
	Start       *list.List[HookInfo] `json:"start"`
	Stop        *list.List[HookInfo] `json:"stop"`
}

type HookInfo struct {
	Name      string  `json:"name"`
	Label     string  `json:"label"`
	Kind      string  `json:"kind"`
	Priority  int     `json:"priority"`
	Parallel  bool    `json:"parallel"`
	Timeout   string  `json:"timeout,omitempty"`
	TimeoutMS float64 `json:"timeoutMs,omitempty"`
	Sequence  int     `json:"sequence"`
}

type EventSnapshot struct {
	Count    int                     `json:"count"`
	Capacity int                     `json:"capacity"`
	Recent   *list.List[RecentEvent] `json:"recent"`
}

type RecentEvent struct {
	At         time.Time `json:"at"`
	Type       string    `json:"type"`
	Operation  string    `json:"operation,omitempty"`
	Target     string    `json:"target,omitempty"`
	Status     string    `json:"status"`
	Duration   string    `json:"duration,omitempty"`
	DurationMS float64   `json:"durationMs,omitempty"`
	Detail     string    `json:"detail,omitempty"`
}

type GraphSnapshot struct {
	Nodes           int    `json:"nodes"`
	Edges           int    `json:"edges"`
	Apps            int    `json:"apps"`
	Modules         int    `json:"modules"`
	Services        int    `json:"services"`
	Operations      int    `json:"operations"`
	EagerOperations int    `json:"eagerOperations"`
	RawOperations   int    `json:"rawOperations"`
	Error           string `json:"error,omitempty"`
}

func (s *Service) Snapshot() Snapshot {
	rt := s.Runtime()
	if rt == nil {
		return Snapshot{
			App: RuntimeSummary{State: "unavailable"},
			Lifecycle: LifecycleSnapshot{
				Concurrency: lifecycleplan.Concurrency,
				Start:       list.NewList[HookInfo](),
				Stop:        list.NewList[HookInfo](),
			},
			Events: EventSnapshot{Recent: list.NewList[RecentEvent]()},
		}
	}

	summary := rt.LifecycleSummary()
	events := rt.RecentEvents()
	buildDuration, startDuration := runtimeDurations(events)
	buildDurationText, buildDurationMS := durationFields(buildDuration)
	startDurationText, startDurationMS := durationFields(startDuration)
	meta := rt.Meta()

	capacity := 0
	if recorder := rt.EventRecorder(); recorder != nil {
		capacity = recorder.Capacity()
	}

	return Snapshot{
		App: RuntimeSummary{
			Name:            rt.Name(),
			Version:         meta.Version,
			Profile:         string(rt.Profile()),
			State:           rt.State().String(),
			BuildDuration:   buildDurationText,
			BuildDurationMS: buildDurationMS,
			StartDuration:   startDurationText,
			StartDurationMS: startDurationMS,
		},
		Lifecycle: LifecycleSnapshot{
			StartHooks:  summary.StartHooks,
			StopHooks:   summary.StopHooks,
			Concurrency: summary.Concurrency,
			Start:       hookInfoList(summary.Start),
			Stop:        hookInfoList(summary.Stop),
		},
		Events: EventSnapshot{
			Count:    events.Len(),
			Capacity: capacity,
			Recent:   recentEventList(events),
		},
		Graph: graphSnapshot(rt),
	}
}

func hookInfoList(in *list.List[dix.LifecycleHookSummary]) *list.List[HookInfo] {
	if in == nil {
		return list.NewList[HookInfo]()
	}
	return list.MapList(in, func(_ int, hook dix.LifecycleHookSummary) HookInfo {
		timeout, timeoutMS := durationFields(hook.Timeout)
		return HookInfo{
			Name:      hook.Name,
			Label:     hook.Label,
			Kind:      string(hook.Kind),
			Priority:  hook.Priority,
			Parallel:  hook.Parallel,
			Timeout:   timeout,
			TimeoutMS: timeoutMS,
			Sequence:  hook.Sequence,
		}
	})
}

func recentEventList(records *list.List[dix.EventRecord]) *list.List[RecentEvent] {
	if records == nil || records.Len() == 0 {
		return list.NewList[RecentEvent]()
	}
	values := records.Values()
	start := max(len(values)-maxRecentEvents, 0)
	return list.MapList(list.NewList(values[start:]...), func(_ int, record dix.EventRecord) RecentEvent {
		return recentEvent(record)
	})
}

func recentEvent(record dix.EventRecord) RecentEvent {
	out := RecentEvent{At: record.At, Status: "ok"}
	if record.Event == nil {
		out.Type = "unknown"
		return out
	}

	switch event := record.Event.(type) {
	case dix.BuildEvent, dix.StartEvent, dix.StopEvent:
		applyRuntimeEvent(&out, event)
	case dix.HealthCheckEvent:
		out.Type = "health"
		out.Operation = string(event.Kind)
		out.Target = event.Name
		out.Duration, out.DurationMS = durationFields(event.Duration)
		out.Status = eventStatus(event.Err)
		out.Detail = errDetail(event.Err)
	case dix.StateTransitionEvent:
		out.Type = "state"
		out.Operation = event.From.String() + "->" + event.To.String()
		out.Target = event.Meta.Name
		out.Detail = event.Reason
	case dix.ProviderEvent, dix.ResolveEvent, dix.LifecycleHookEvent:
		applyOperationEvent(&out, event)
	case dix.MessageEvent:
		out.Type = "message"
		out.Operation = string(event.Level)
		out.Target = event.Message
	default:
		out.Type = fmt.Sprintf("%T", record.Event)
	}
	return out
}

func applyRuntimeEvent(out *RecentEvent, event any) {
	switch event := event.(type) {
	case dix.BuildEvent:
		setTimedEvent(out, "build", "build", event.Meta.Name, event.Duration, event.Err)
	case dix.StartEvent:
		setTimedEvent(out, "start", "start", event.Meta.Name, event.Duration, event.Err)
	case dix.StopEvent:
		setTimedEvent(out, "stop", "stop", event.Meta.Name, event.Duration, event.Err)
	}
}

func applyOperationEvent(out *RecentEvent, event any) {
	switch event := event.(type) {
	case dix.ProviderEvent:
		setTimedEvent(out, "provider", event.Operation, nonEmpty(event.Service, event.Label), event.Duration, event.Err)
	case dix.ResolveEvent:
		setTimedEvent(out, "resolve", event.Operation, event.Service, event.Duration, event.Err)
	case dix.LifecycleHookEvent:
		setTimedEvent(out, "hook", string(event.Kind), nonEmpty(event.Name, event.Label), event.Duration, event.Err)
	}
}

func setTimedEvent(out *RecentEvent, typ, operation, target string, duration time.Duration, err error) {
	out.Type = typ
	out.Operation = operation
	out.Target = target
	out.Duration, out.DurationMS = durationFields(duration)
	out.Status = eventStatus(err)
	out.Detail = errDetail(err)
}

func runtimeDurations(records *list.List[dix.EventRecord]) (time.Duration, time.Duration) {
	var buildDuration time.Duration
	var startDuration time.Duration
	if records == nil {
		return buildDuration, startDuration
	}
	records.Range(func(_ int, record dix.EventRecord) bool {
		switch event := record.Event.(type) {
		case dix.BuildEvent:
			buildDuration = event.Duration
		case dix.StartEvent:
			startDuration = event.Duration
		}
		return true
	})
	return buildDuration, startDuration
}

func durationFields(d time.Duration) (string, float64) {
	if d <= 0 {
		return "", 0
	}
	return d.String(), float64(d.Microseconds()) / 1000
}

func eventStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

func errDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
