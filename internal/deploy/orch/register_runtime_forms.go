package orch

import "github.com/arcgolabs/plano/schema"

func runtimeOptionFormSpecs() []schema.FormSpec {
	return []schema.FormSpec{
		dockerFormSpec(),
		containerdFormSpec(),
		podmanFormSpec(),
		firecrackerFormSpec(),
		processFormSpec(),
		systemdFormSpec(),
		windowsServiceFormSpec(),
	}
}

func dockerFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "docker",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "network", Type: schema.TypeString, Docs: "Alias for network_mode."},
			schema.FieldSpec{Name: "network_mode", Type: schema.TypeString},
			schema.FieldSpec{Name: "privileged", Type: schema.TypeBool, Default: false, HasDefault: true},
			schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
		),
	}
}

func podmanFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "podman",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "network", Type: schema.TypeString, Docs: "Alias for network_mode."},
			schema.FieldSpec{Name: "network_mode", Type: schema.TypeString},
			schema.FieldSpec{Name: "privileged", Type: schema.TypeBool, Default: false, HasDefault: true},
			schema.FieldSpec{Name: "labels", Type: schema.MapType{Elem: schema.TypeString}},
		),
	}
}

func containerdFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "containerd",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "namespace", Type: schema.TypeString},
		),
	}
}

func firecrackerFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "firecracker",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "kernel_image_path", Type: schema.TypeString},
			schema.FieldSpec{Name: "rootfs_path", Type: schema.TypeString},
			schema.FieldSpec{Name: "boot_args", Type: schema.TypeString},
			schema.FieldSpec{Name: "binary_path", Type: schema.TypeString},
			schema.FieldSpec{Name: "socket_path", Type: schema.TypeString},
			schema.FieldSpec{Name: "rootfs_read_only", Type: schema.TypeBool},
			schema.FieldSpec{Name: "network_interface_id", Type: schema.TypeString},
			schema.FieldSpec{Name: "tap_device_name", Type: schema.TypeString},
			schema.FieldSpec{Name: "guest_mac", Type: schema.TypeString},
			schema.FieldSpec{Name: "allow_mmds_requests", Type: schema.TypeBool},
			schema.FieldSpec{Name: "vcpu_count", Type: schema.TypeInt},
			schema.FieldSpec{Name: "mem_size_mib", Type: schema.TypeInt},
		),
	}
}

func processFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "process",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "graceful_stop_timeout", Type: schema.TypeString},
			schema.FieldSpec{Name: "stdout_path", Type: schema.TypeString},
			schema.FieldSpec{Name: "stderr_path", Type: schema.TypeString},
		),
	}
}

func systemdFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "systemd",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "unit_name", Type: schema.TypeString},
			schema.FieldSpec{Name: "user", Type: schema.TypeString},
			schema.FieldSpec{Name: "group", Type: schema.TypeString},
			schema.FieldSpec{Name: "restart", Type: schema.TypeString},
			schema.FieldSpec{Name: "restart_sec", Type: schema.TypeString},
			schema.FieldSpec{Name: "wanted_by", Type: schema.TypeString},
		),
	}
}

func windowsServiceFormSpec() schema.FormSpec {
	return schema.FormSpec{
		Name:      "windows_service",
		LabelKind: schema.LabelNone,
		BodyMode:  schema.BodyFieldOnly,
		Fields: schema.Fields(
			schema.FieldSpec{Name: "service_name", Type: schema.TypeString},
			schema.FieldSpec{Name: "display_name", Type: schema.TypeString},
			schema.FieldSpec{Name: "start_type", Type: schema.TypeString},
		),
	}
}
