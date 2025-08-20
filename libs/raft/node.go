package raft

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/DaiYuANg/warden/libs/pkg"
	"github.com/hashicorp/raft"
	raftwal "github.com/hashicorp/raft-wal"
	"go.uber.org/zap"
)

type Option struct {
}

func newNode(nodeID, raftDir string, logger *zap.SugaredLogger) (*raft.Raft, error) {
	log := newZapLogger(logger)
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(nodeID)
	raftConfig.Logger = log
	if err := pkg.EnsureDir(raftDir, 0700).Error(); err != nil {
		return nil, err
	}
	// 创建目录
	storagePath := filepath.Join(raftDir, "warden.wal")
	if err := pkg.EnsureDir(storagePath, 0700).Error(); err != nil {
		return nil, fmt.Errorf("create raft storage path: %w", err)
	}

	fsm, err := newFsm(raftDir, logger)
	if err != nil {
		return nil, err
	}

	// 打开持久化存储
	raftStorage, err := raftwal.Open(storagePath, raftwal.WithLogger(log))
	if err != nil {
		return nil, fmt.Errorf("open raft WAL storage: %w", err)
	}

	// 创建快照存储
	snapshotStore, err := raft.NewFileSnapshotStore(raftDir, 1, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("create snapshot store: %w", err)
	}

	// 设置 TCP 传输
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:12000")
	if err != nil {
		return nil, fmt.Errorf("resolve raft address: %w", err)
	}

	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("create raft transport: %w", err)
	}

	// 启动 raft 节点
	raftNode, err := raft.NewRaft(raftConfig, fsm, raftStorage, raftStorage, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("create raft node: %w", err)
	}

	return raftNode, nil
}
