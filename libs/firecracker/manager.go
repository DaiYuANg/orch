package firecracker

import (
	"fmt"
	sdk "github.com/firecracker-microvm/firecracker-go-sdk"
	"golang.org/x/net/context"
	"sync"
	"time"
)

type VMInstance struct {
	ID       VMID
	Options  VMOptions
	Machine  *sdk.Machine
	Status   Status
	StartAt  time.Time
	ErrorMsg string
}

type Manager struct {
	mu       sync.RWMutex
	vms      map[VMID]*VMInstance
	nextID   int64
	basePath string // 如 /tmp/fc-vm-xx.sock、fc-vm-xx.log 统一路径前缀
}

func NewManager(basePath string) *Manager {
	return &Manager{
		vms:      make(map[VMID]*VMInstance),
		basePath: basePath,
	}
}

// 创建并启动一个 VM
func (m *Manager) StartVM(ctx context.Context, opts VMOptions) (VMID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := VMID(fmt.Sprintf("vm-%d", m.nextID))
	m.nextID++

	// 自动补充 socket/log 路径
	opts.SocketPath = fmt.Sprintf("%s/%s.sock", m.basePath, id)
	opts.LogFifo = fmt.Sprintf("%s/%s.log", m.basePath, id)

	vm, err := New(opts)
	if err != nil {
		m.vms[id] = &VMInstance{
			ID:       id,
			Options:  opts,
			Status:   StatusErrored,
			ErrorMsg: err.Error(),
		}
		return "", err
	}

	m.vms[id] = &VMInstance{
		ID:      id,
		Options: opts,
		Machine: vm.Machine,
		Status:  StatusRunning,
		StartAt: time.Now(),
	}
	return id, nil
}

// 查询状态
func (m *Manager) GetStatus(id VMID) (Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vm, ok := m.vms[id]
	if !ok {
		return "", fmt.Errorf("VM %s not found", id)
	}
	return vm.Status, nil
}

// 停止 VM（同步）
func (m *Manager) StopVM(ctx context.Context, id VMID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vm, ok := m.vms[id]
	if !ok {
		return fmt.Errorf("VM %s not found", id)
	}

	if vm.Status != StatusRunning {
		return fmt.Errorf("VM %s is not running", id)
	}

	if err := vm.Machine.StopVMM(); err != nil {
		vm.Status = StatusErrored
		vm.ErrorMsg = err.Error()
		return err
	}
	vm.Status = StatusStopped
	return nil
}

// ListVMs 获取全部 VM 状态（方便 Web UI 展示）
func (m *Manager) ListVMs() []VMInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vms := make([]VMInstance, 0, len(m.vms))
	for _, vm := range m.vms {
		vms = append(vms, *vm)
	}
	return vms
}
