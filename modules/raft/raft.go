package store

import "os"

import (
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftManager 用于管理 Raft 集群
type RaftManager struct {
	raftNode *raft.Raft
}

// NewRaftManager 初始化 Raft 集群
func NewRaftManager(nodeID, raftDir string) (*RaftManager, error) {
	// 创建 Raft 配置
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(nodeID)

	// 创建 Raft 存储
	raftStorage, err := raftboltdb.NewBoltStore(raftDir)
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return nil, err
	}

	// 创建 Raft 节点
	raftNode, err := raft.NewRaft(raftConfig, nil, raftStorage, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	return &RaftManager{raftNode: raftNode}, nil
}

// ApplyLog 提交 Raft 日志
func (r *RaftManager) ApplyLog(logData []byte) error {
	// 模拟提交日志到 Raft 节点
	// 这里可以替换为你的实际应用逻辑
	log := r.raftNode.Apply(logData, 0)
	return log.Error()
}

// GetRaftNode 返回 Raft 节点
func (r *RaftManager) GetRaftNode() *raft.Raft {
	return r.raftNode
}
