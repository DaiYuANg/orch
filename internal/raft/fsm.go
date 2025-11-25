package raft

import (
	"errors"
	"io"

	"github.com/hashicorp/raft"
	"go.etcd.io/bbolt"
)

type FSM struct {
	badger *badgerDB
	bblot  *bbolt.DB
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	return nil
}
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	// 返回快照
	return nil, nil
}
func (f *FSM) Restore(rc io.ReadCloser) error {

	return nil
}

func (f *FSM) close() error {
	err1 := f.bblot.Close()
	err2 := f.badger.Close()
	return errors.Join(err1, err2)
}
