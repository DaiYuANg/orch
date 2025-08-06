package runtime_engine

import (
	"fmt"
	"os"

	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
	"golang.org/x/net/context"
)

type VMOptions struct {
	KernelImagePath string
	RootFSPath      string
	SocketPath      string
	LogFifo         string
	VCPUs           int64
	MemMiB          int64
}

type VM struct {
	Machine *sdk.Machine
}

// 默认配置
func defaultVMOptions() VMOptions {
	return VMOptions{
		VCPUs:  1,
		MemMiB: 128,
	}
}

type VMID string

func New(opts VMOptions) (*VM, error) {
	ctx := context.Background()

	// 设置默认值
	if opts.VCPUs == 0 {
		opts.VCPUs = 1
	}
	if opts.MemMiB == 0 {
		opts.MemMiB = 128
	}

	// 清理旧 socket
	_ = os.Remove(opts.SocketPath)

	// 构建启动命令
	cmd := sdk.VMCommandBuilder{}.
		WithSocketPath(opts.SocketPath).
		WithBin("/usr/local/bin/firecracker").
		Build(ctx)

	// 配置 microVM
	cfg := sdk.Config{
		SocketPath:      opts.SocketPath,
		LogFifo:         opts.LogFifo,
		LogLevel:        "Debug",
		KernelImagePath: opts.KernelImagePath,
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  sdk.Int64(opts.VCPUs),
			MemSizeMib: sdk.Int64(opts.MemMiB),
			//HtEnabled:  sdk.Bool(false),
		},
		Drives: []models.Drive{
			{
				DriveID:      sdk.String("rootfs"),
				PathOnHost:   sdk.String(opts.RootFSPath),
				IsRootDevice: sdk.Bool(true),
				IsReadOnly:   sdk.Bool(false),
			},
		},
	}

	machine, err := sdk.NewMachine(ctx, cfg, sdk.WithProcessRunner(cmd))
	if err != nil {
		return nil, fmt.Errorf("create machine: %w", err)
	}

	if err := machine.Start(ctx); err != nil {
		return nil, fmt.Errorf("start machine: %w", err)
	}

	return &VM{Machine: machine}, nil
}
