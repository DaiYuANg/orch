package raft

import (
	"time"

	"log/slog"

	"github.com/hashicorp/raft"
	"go.etcd.io/bbolt"
)

type Manager struct {
	raftNode *raft.Raft
	fsm      *FSM
}

type ManagerConfig struct {
	NodeID    string
	BindAddr  string
	DataDir   string
	Bootstrap bool
	Join      []string
}

func NewRaftManager(cfg ManagerConfig, logger *slog.Logger) (*Manager, error) {
	node, fsm, err := newNode(cfg, logger)
	if err != nil {
		return nil, err
	}
	return &Manager{
		raftNode: node,
		fsm:      fsm,
	}, nil
}

func (r *Manager) ApplyLog(logData []byte, timeout time.Duration) error {
	log := r.raftNode.Apply(logData, timeout)
	return log.Error()
}

func (r *Manager) GetRaftNode() *raft.Raft {
	return r.raftNode
}

func (r *Manager) MetadataDB() *bbolt.DB {
	if r.fsm == nil {
		return nil
	}
	return r.fsm.bblot
}

func (r *Manager) Read(bucket, key string) ([]byte, error) {
	if r.fsm == nil {
		return nil, bbolt.ErrDatabaseNotOpen
	}
	return r.fsm.Read(bucket, key)
}
