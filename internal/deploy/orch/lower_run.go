package orch

import (
	"errors"
	"fmt"
	"strings"

	"github.com/arcgolabs/plano/compiler"

	v1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

func fillRun(run *v1.RunSpec, f *compiler.HIRForm) {
	fillArtifactFromFields(&run.Artifact, f)
	fillExecFromFields(run, f)
	fillProcessIdentityFromFields(run, f)
}

func fillRunFromFields(run *v1.RunSpec, f *compiler.HIRForm) error {
	before := run.Artifact
	fillArtifactFromFields(&run.Artifact, f)
	if err := validateArtifactOverride(before, run.Artifact); err != nil {
		return err
	}
	fillExecFromFields(run, f)
	fillProcessIdentityFromFields(run, f)
	return nil
}

func validateArtifactOverride(before, after v1.ArtifactSpec) error {
	if before.Image != "" && after.Image != before.Image {
		return errors.New("image is set both in run block and workload field")
	}
	if before.Path != "" && after.Path != before.Path {
		return errors.New("path is set both in run block and workload field")
	}
	if before.URL != "" && after.URL != before.URL {
		return errors.New("url is set both in run block and workload field")
	}
	return nil
}

func fillExecFromFields(run *v1.RunSpec, f *compiler.HIRForm) {
	if f.Fields == nil {
		return
	}
	if cmd, ok := f.Fields.Get("command"); ok {
		run.Exec.Command = stringList(cmd.Value)
	}
	if args, ok := f.Fields.Get("args"); ok {
		run.Exec.Args = stringList(args.Value)
	}
}

func fillProcessIdentityFromFields(run *v1.RunSpec, f *compiler.HIRForm) {
	if cwd, ok := stringField(f, "cwd"); ok {
		run.Cwd = cwd
	}
	if user, ok := stringField(f, "user"); ok {
		run.User = strings.TrimSpace(user)
	}
}

func fillArtifactFromFields(artifact *v1.ArtifactSpec, f *compiler.HIRForm) {
	if artifact == nil {
		return
	}
	if img, ok := stringField(f, "image"); ok && strings.TrimSpace(img) != "" {
		artifact.Image = strings.TrimSpace(img)
	}
	if path, ok := stringField(f, "path"); ok && strings.TrimSpace(path) != "" {
		artifact.Path = strings.TrimSpace(path)
	}
	if url, ok := stringField(f, "url"); ok && strings.TrimSpace(url) != "" {
		artifact.URL = strings.TrimSpace(url)
	}
}

func fillRuntimeOptions(opts *v1.RunOptions, f *compiler.HIRForm) error {
	blocks := childFormsByKind(f, "runtime_options")
	if len(blocks) > 1 {
		return errors.New("at most one runtime_options block")
	}
	if len(blocks) == 0 {
		return nil
	}
	return fillRuntimeOptionForms(opts, &blocks[0], "runtime_options")
}

func fillRuntimeOptionForms(opts *v1.RunOptions, f *compiler.HIRForm, scope string) error {
	if err := fillOneOption(childFormsByKind(f, "docker"), scope, "docker", func(form *compiler.HIRForm) {
		opts.Docker = mergeDockerOptions(opts.Docker, lowerDockerOptions(form))
	}); err != nil {
		return err
	}
	if err := fillOneOption(childFormsByKind(f, "podman"), scope, "podman", func(form *compiler.HIRForm) {
		opts.Docker = mergeDockerOptions(opts.Docker, lowerDockerOptions(form))
	}); err != nil {
		return err
	}
	if err := fillOneOption(childFormsByKind(f, "containerd"), scope, "containerd", func(form *compiler.HIRForm) {
		opts.Containerd = lowerContainerdOptions(form)
	}); err != nil {
		return err
	}
	if err := fillOneOption(childFormsByKind(f, "firecracker"), scope, "firecracker", func(form *compiler.HIRForm) {
		opts.Firecracker = lowerFirecrackerOptions(form)
	}); err != nil {
		return err
	}
	if err := fillOneOption(childFormsByKind(f, "process"), scope, "process", func(form *compiler.HIRForm) {
		opts.Process = lowerProcessOptions(form)
	}); err != nil {
		return err
	}
	if err := fillOneOption(childFormsByKind(f, "systemd"), scope, "systemd", func(form *compiler.HIRForm) {
		opts.Systemd = lowerSystemdOptions(form)
	}); err != nil {
		return err
	}
	return fillOneOption(childFormsByKind(f, "windows_service"), scope, "windows_service", func(form *compiler.HIRForm) {
		opts.WindowsService = lowerWindowsServiceOptions(form)
	})
}

func fillOneOption(forms []compiler.HIRForm, scope, name string, fill func(*compiler.HIRForm)) error {
	if len(forms) > 1 {
		return fmt.Errorf("%s: at most one %s block", scope, name)
	}
	if len(forms) == 1 {
		fill(&forms[0])
	}
	return nil
}

func fillDockerOptionsFromFields(opts *v1.RunOptions, f *compiler.HIRForm) error {
	docker := lowerDockerOptions(f)
	if docker != nil {
		opts.Docker = mergeDockerOptions(opts.Docker, docker)
	}
	return fillRuntimeOptionForms(opts, f, "workload")
}
