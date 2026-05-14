package process

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/runtimeinfo"
	"github.com/daiyuang/orch/pkg/oopsx"
)

func (p *Provider) Logs(_ context.Context, meta deployv1.Metadata, workloadName string, opts runtimeinfo.LogOptions) (runtimeinfo.LogResult, error) {
	base := p.nameBase(meta, workloadName)
	logDir := filepath.Join(p.rootOrDefault(), "logs")
	stdout, err := runtimeinfo.ReadTailFile(filepath.Join(logDir, base+".stdout.log"), opts.Tail)
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "process").Wrapf(err, "read stdout log")
	}
	stderr, err := runtimeinfo.ReadTailFile(filepath.Join(logDir, base+".stderr.log"), opts.Tail)
	if err != nil {
		return runtimeinfo.LogResult{}, oopsx.B("runtime", "process").Wrapf(err, "read stderr log")
	}
	return runtimeinfo.LogResult{
		Name:    strings.TrimSpace(workloadName),
		Runtime: deployv1.RuntimeProcess,
		Source:  logDir,
		Content: combineProcessLogs(stdout, stderr),
	}, nil
}

func combineProcessLogs(stdout, stderr string) string {
	if stderr == "" {
		return stdout
	}
	if stdout != "" && !strings.HasSuffix(stdout, "\n") {
		stdout += "\n"
	}
	return stdout + stderr
}

func (p *Provider) openLogs(meta deployv1.Metadata, w deployv1.Workload) (*os.File, *os.File, func(), error) {
	stdoutPath, stderrPath := p.logPaths(meta, w)
	stdout, err := openAppend(stdoutPath)
	if err != nil {
		return nil, nil, func() {}, err
	}
	stderr, err := openAppend(stderrPath)
	if err != nil {
		p.cleanupCloseFile(stdout, "stdout")
		return nil, nil, func() {}, err
	}
	closeLogs := func() {
		p.cleanupCloseFile(stdout, "stdout")
		p.cleanupCloseFile(stderr, "stderr")
	}
	return stdout, stderr, closeLogs, nil
}

func (p *Provider) logPaths(meta deployv1.Metadata, w deployv1.Workload) (string, string) {
	base := p.nameBase(meta, w.Name)
	stdoutPath := filepath.Join(p.rootOrDefault(), "logs", base+".stdout.log")
	stderrPath := filepath.Join(p.rootOrDefault(), "logs", base+".stderr.log")
	if w.Run.Options.Process != nil {
		stdoutPath = overrideProcessLogPath(stdoutPath, w.Run.Options.Process.StdoutPath)
		stderrPath = overrideProcessLogPath(stderrPath, w.Run.Options.Process.StderrPath)
	}
	return stdoutPath, stderrPath
}

func overrideProcessLogPath(defaultPath, configured string) string {
	if path := strings.TrimSpace(configured); path != "" {
		return path
	}
	return defaultPath
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, oopsx.B("runtime", "process").Wrapf(err, "create log dir")
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, oopsx.B("runtime", "process").Wrapf(err, "open log dir")
	}
	defer func() {
		if closeErr := root.Close(); closeErr != nil {
			slog.Default().Warn("process log dir close failed", "error", closeErr)
		}
	}()
	f, err := root.OpenFile(filepath.Base(path), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, oopsx.B("runtime", "process").Wrapf(err, "open log %s", filepath.Base(path))
	}
	return f, nil
}

func (p *Provider) cleanupCloseFile(f *os.File, stream string) {
	if err := f.Close(); err != nil {
		p.logger.Warn("process log close failed", "stream", stream, "error", err)
	}
}
