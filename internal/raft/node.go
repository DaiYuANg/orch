package raft

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DaiYuANg/warden/pkg"
	"github.com/hashicorp/raft"
	raftwal "github.com/hashicorp/raft-wal"
)

func newNode(cfg ManagerConfig, logger *slog.Logger) (*raft.Raft, *FSM, error) {
	if strings.TrimSpace(cfg.NodeID) == "" {
		return nil, nil, fmt.Errorf("raft node id is required")
	}
	if strings.TrimSpace(cfg.BindAddr) == "" {
		return nil, nil, fmt.Errorf("raft bind addr is required")
	}
	if strings.TrimSpace(cfg.DataDir) == "" {
		return nil, nil, fmt.Errorf("raft data dir is required")
	}

	log := newZapLogger(logger)
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(cfg.NodeID)
	raftConfig.Logger = log
	if err := pkg.EnsureDir(cfg.DataDir, 0700).Error(); err != nil {
		return nil, nil, err
	}

	storagePath := filepath.Join(cfg.DataDir, "warden.wal")
	if err := pkg.EnsureDir(storagePath, 0700).Error(); err != nil {
		return nil, nil, fmt.Errorf("create raft storage path: %w", err)
	}

	fsm, err := newFsm(cfg.DataDir, logger)
	if err != nil {
		return nil, nil, err
	}

	raftStorage, err := raftwal.Open(storagePath, raftwal.WithLogger(log))
	if err != nil {
		return nil, nil, fmt.Errorf("open raft WAL storage: %w", err)
	}

	snapshotStore, err := raft.NewFileSnapshotStore(cfg.DataDir, 1, os.Stdout)
	if err != nil {
		return nil, nil, fmt.Errorf("create snapshot store: %w", err)
	}

	addr, err := net.ResolveTCPAddr("tcp", cfg.BindAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve raft address: %w", err)
	}

	transport, err := raft.NewTCPTransport(cfg.BindAddr, addr, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return nil, nil, fmt.Errorf("create raft transport: %w", err)
	}

	raftNode, err := raft.NewRaft(raftConfig, fsm, raftStorage, raftStorage, snapshotStore, transport)
	if err != nil {
		return nil, nil, fmt.Errorf("create raft node: %w", err)
	}

	if cfg.Bootstrap {
		if err := bootstrapIfEmpty(raftNode, raft.ServerID(cfg.NodeID), raft.ServerAddress(cfg.BindAddr)); err != nil {
			return nil, nil, err
		}
	}

	return raftNode, fsm, nil
}

func bootstrapIfEmpty(node *raft.Raft, id raft.ServerID, address raft.ServerAddress) error {
	current := node.GetConfiguration()
	if err := current.Error(); err != nil {
		return fmt.Errorf("get raft configuration: %w", err)
	}
	if len(current.Configuration().Servers) > 0 {
		return nil
	}
	future := node.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      id,
				Address: address,
			},
		},
	})
	if err := future.Error(); err != nil && err != raft.ErrCantBootstrap {
		return fmt.Errorf("bootstrap raft cluster: %w", err)
	}
	return nil
}
