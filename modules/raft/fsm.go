package store

import (
	"github.com/hashicorp/raft"
	"io"
)

type FSM struct{}

func (f *FSM) Apply(log *raft.Log) interface{} {
	// 这里写应用状态变更逻辑
	return nil
}
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	// 返回快照
	return nil, nil
}
func (f *FSM) Restore(rc io.ReadCloser) error {
	// 从快照恢复
	return nil
}
