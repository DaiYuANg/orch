package store

import (
	"go.uber.org/zap"
	"net"
	"os"
	"path/filepath"
	"time"
)

import (
	"github.com/hashicorp/raft"
	raftwal "github.com/hashicorp/raft-wal"
)

// RaftManager 用于管理 Raft 集群
type RaftManager struct {
	raftNode *raft.Raft
}

// NewRaftManager 初始化 Raft 集群
func NewRaftManager(nodeID, raftDir string, logger *zap.SugaredLogger) (*RaftManager, error) {
	// 创建 Raft 配置
	log := newZapLogger(logger)
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(nodeID)
	raftConfig.Logger = log
	// 创建 Raft 存储
	storagePath := filepath.Join(raftDir, "warden.wal")
	if err := os.MkdirAll(storagePath, 0700); err != nil {
		return nil, err
	}
	raftStorage, err := raftwal.Open(storagePath, raftwal.WithLogger(log))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(raftDir, 0700); err != nil {
		return nil, err
	}
	// 创建快照存储
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stdout)
	if err != nil {
		return nil, err
	}
	// 创建网络传输
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:12000")
	if err != nil {
		return nil, err
	}
	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return nil, err
	}
	raftNode, err := raft.NewRaft(raftConfig, &FSM{}, raftStorage, raftStorage, snapshotStore, transport)
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
