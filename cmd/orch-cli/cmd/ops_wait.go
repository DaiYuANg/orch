package cmd

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/pterm/pterm"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func waitReady(ctx context.Context, c *apiclient.Client, timeout time.Duration, progress bool) (*api.ReadyOutput, error) {
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	spinner := startStatusSpinner(progress, "waiting for control plane readiness")

	var last *api.ReadyOutput
	var lastErr error
	out, err := backoff.Retry(waitCtx, func() (*api.ReadyOutput, error) {
		out, err := c.Ready(waitCtx)
		if err != nil {
			lastErr = err
			updateStatusSpinner(spinner, "waiting for control plane readiness last_error="+err.Error())
			return nil, oopsx.B("cli").Wrapf(err, "ready")
		}
		last = out
		updateStatusSpinner(spinner, "waiting for control plane readiness status="+out.Body.Status)
		if out.Body.Ready {
			successWatchSpinner(spinner, "control plane ready")
			return out, nil
		}
		return nil, errWaitPending
	}, backoff.WithBackOff(backoff.NewConstantBackOff(500*time.Millisecond)))
	if err == nil {
		return out, nil
	}
	failWatchSpinner(spinner, "control plane readiness timed out")
	if lastErr != nil && !errors.Is(err, errWaitPending) {
		return last, oopsx.B("cli").Wrapf(lastErr, "wait ready timed out after %s", timeout)
	}
	return last, oopsx.B("cli").Errorf("wait ready timed out after %s", timeout)
}

func waitAppStatus(ctx context.Context, c *apiclient.Client, namespace, name, target string, timeout time.Duration, progress bool) (*api.GetAppOutput, error) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		target = "running"
	}
	if timeout <= 0 {
		return nil, oopsx.B("cli").Errorf("--timeout must be greater than zero")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	spinner := startStatusSpinner(progress, "waiting for app "+name+" status="+target)
	waiter := newAppStatusWaiter(c, namespace, name, target, spinner)

	out, err := backoff.Retry(waitCtx, waiter.poll(waitCtx), backoff.WithBackOff(backoff.NewConstantBackOff(500*time.Millisecond)))
	return waiter.result(out, err, timeout)
}

type appStatusWaiter struct {
	client       *apiclient.Client
	namespace    string
	name         string
	target       string
	spinner      *pterm.SpinnerPrinter
	last         *api.GetAppOutput
	lastErr      error
	permanentErr error
}

func newAppStatusWaiter(client *apiclient.Client, namespace, name, target string, spinner *pterm.SpinnerPrinter) *appStatusWaiter {
	return &appStatusWaiter{
		client:    client,
		namespace: namespace,
		name:      name,
		target:    target,
		spinner:   spinner,
	}
}

func (w *appStatusWaiter) poll(ctx context.Context) func() (*api.GetAppOutput, error) {
	return func() (*api.GetAppOutput, error) {
		out, err := w.client.GetApp(ctx, w.namespace, w.name)
		if err != nil {
			w.lastErr = err
			updateStatusSpinner(w.spinner, "waiting for app "+w.name+" last_error="+err.Error())
			return nil, oopsx.B("cli").Wrapf(err, "get app")
		}
		return w.recordOutput(out)
	}
}

func (w *appStatusWaiter) recordOutput(out *api.GetAppOutput) (*api.GetAppOutput, error) {
	w.last = out
	status := strings.ToLower(strings.TrimSpace(out.Body.Status))
	updateStatusSpinner(w.spinner, "waiting for app "+w.name+" status="+status+" ready="+appReadyText(out.Body.Running, out.Body.DesiredWorkloads))
	if status == w.target {
		successWatchSpinner(w.spinner, "app "+w.name+" status="+status)
		return out, nil
	}
	if status == "failed" && w.target != "failed" {
		return nil, w.failedStatus(out)
	}
	return nil, errWaitPending
}

func (w *appStatusWaiter) failedStatus(out *api.GetAppOutput) error {
	failWatchSpinner(w.spinner, "app "+w.name+" failed")
	w.permanentErr = oopsx.B("cli").Errorf("app %s reached failed status: %s", w.name, nonEmpty(out.Body.LastError))
	return oopsx.B("cli").Wrapf(backoff.Permanent(w.permanentErr), "app reached terminal status")
}

func (w *appStatusWaiter) result(out *api.GetAppOutput, err error, timeout time.Duration) (*api.GetAppOutput, error) {
	if err == nil {
		return out, nil
	}
	if w.permanentErr != nil {
		return w.last, w.permanentErr
	}
	failWatchSpinner(w.spinner, "wait app timed out")
	if w.lastErr != nil && !errors.Is(err, errWaitPending) {
		return w.last, oopsx.B("cli").Wrapf(w.lastErr, "wait app timed out after %s", timeout)
	}
	return w.last, oopsx.B("cli").Errorf("wait app timed out after %s", timeout)
}

func startStatusSpinner(progress bool, text string) *pterm.SpinnerPrinter {
	if !progress || !stderrIsTerminal() {
		return nil
	}
	spinner, err := pterm.DefaultSpinner.WithRemoveWhenDone(false).Start(text)
	if err != nil {
		return nil
	}
	return spinner
}

func updateStatusSpinner(spinner *pterm.SpinnerPrinter, text string) {
	if spinner != nil {
		spinner.UpdateText(text)
	}
}
