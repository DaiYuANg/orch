//go:build linux
// +build linux

package systemd

type ServiceSpec struct {
	Description      string
	Type             string   // "simple", "notify", "forking", "oneshot", ...
	ExecStart        []string // 单个 ExecStart 命令（binary + args），常用情形
	UncleanIsFailure bool     // 是否把 unclean exit 视为失败（传给 PropExecStart 的第二个参数）

	// 可选的 lifecycle commands（如果需要多行 ExecStartPre/ExecStop，使用 ExtraProperties）
	ExecStop []string

	Restart    string        // e.g. "no", "on-failure" （如需请用 ExtraProperties 或扩展）
	RestartSec time.Duration // 如果要设置 restart time，建议用 ExtraProperties（或扩展此 struct）

	User             string
	Group            string
	WorkingDirectory string
	Environment      map[string]string // map -> 变为 ["K=V", ...]
	Wants            []string
	After            []string
	WantedBy         []string
	RemainAfterExit  bool
	Slice            string

	// 透传：如果某些 systemd 属性没有在上面列出，你可以直接把 dbus.Property 放在这里
	ExtraProperties []sd.Property
}

func (m *SystemdManager) StartTransientUnitFromSpec(ctx context.Context, name string, spec *ServiceSpec, timeout time.Duration) (uint32, error) {
	if m.conn == nil {
		return 0, errors.New("not connected to systemd")
	}
	if spec == nil {
		return 0, errors.New("spec is nil")
	}
	if !strings.HasSuffix(name, ".service") {
		name = name + ".service"
	}

	props := make([]sd.Property, 0, 16)

	// 常用 helper props（如果库提供 helper，优先用 helper）
	if spec.Description != "" {
		props = append(props, sd.PropDescription(spec.Description))
	}
	if spec.Type != "" {
		props = append(props, sd.PropType(spec.Type))
	}
	if spec.ExecStart != nil && len(spec.ExecStart) > 0 {
		// PropExecStart 接受 []string (binary + args)
		props = append(props, sd.PropExecStart(spec.ExecStart, spec.UncleanIsFailure))
	}
	if spec.RemainAfterExit {
		props = append(props, sd.PropRemainAfterExit(true))
	}
	if spec.Slice != "" {
		props = append(props, sd.PropSlice(spec.Slice))
	}
	// After / Wants / WantedBy (unit-level helpers)
	if len(spec.After) > 0 {
		props = append(props, sd.PropAfter(spec.After...))
	}
	if len(spec.Wants) > 0 {
		props = append(props, sd.PropWants(spec.Wants...))
	}
	if len(spec.WantedBy) > 0 {
		props = append(props, sd.PropWantedBy(spec.WantedBy...))
	}

	// 直接用 raw Property 填一些没有 helper 的字段
	if spec.WorkingDirectory != "" {
		props = append(props, sd.Property{
			Name:  "WorkingDirectory",
			Value: godbus.MakeVariant(spec.WorkingDirectory),
		})
	}
	if spec.User != "" {
		props = append(props, sd.Property{
			Name:  "User",
			Value: godbus.MakeVariant(spec.User),
		})
	}
	if spec.Group != "" {
		props = append(props, sd.Property{
			Name:  "Group",
			Value: godbus.MakeVariant(spec.Group),
		})
	}

	// Environment: map -> []string{"K=V", ...}
	if len(spec.Environment) > 0 {
		env := make([]string, 0, len(spec.Environment))
		for k, v := range spec.Environment {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		props = append(props, sd.Property{
			Name:  "Environment",
			Value: godbus.MakeVariant(env),
		})
	}

	// ExecStop 如果需要（注意：go-systemd 对某些 ExecStop* 属性支持在库中有限，
	// 若遇到错误，可把它放到 ExtraProperties）
	if spec.ExecStop != nil && len(spec.ExecStop) > 0 {
		// 把 ExecStop 作为单一 string[] 传入（多数情况只有 1 条）
		props = append(props, sd.Property{
			Name:  "ExecStop",
			Value: godbus.MakeVariant(spec.ExecStop),
		})
	}

	// 透传额外属性（上层可直接指定更复杂或新加的 systemd 属性）
	if len(spec.ExtraProperties) > 0 {
		props = append(props, spec.ExtraProperties...)
	}

	// 调用 StartTransientUnitContext（异步 job 回调放在 ch）
	jobCh := make(chan string, 1)
	jobId, err := m.conn.StartTransientUnitContext(ctx, name, "replace", props, jobCh)
	if err != nil {
		return 0, fmt.Errorf("StartTransientUnit failed: %w", err)
	}

	select {
	case <-jobCh:
		return jobId, nil
	case <-time.After(timeout):
		return jobId, fmt.Errorf("start transient unit timed out after %s (jobId=%d)", timeout, jobId)
	case <-ctx.Done():
		return jobId, ctx.Err()
	}
}
