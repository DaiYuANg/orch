package raft

import (
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

type Manager struct {
	raftNode *raft.Raft
}

func NewRaftManager(nodeID, raftDir string, logger *zap.SugaredLogger) (*Manager, error) {
	node, err := newNode(nodeID, raftDir, logger)
	if err != nil {
		return nil, err
	}
	return &Manager{raftNode: node}, nil
}

func (r *Manager) ApplyLog(logData []byte) error {
	log := r.raftNode.Apply(logData, 0)
	return log.Error()
}

func (r *Manager) GetRaftNode() *raft.Raft {
	return r.raftNode
}
