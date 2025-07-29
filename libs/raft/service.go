package store

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
)

// RaftBadgerService 服务封装，结合 Raft 和 BadgerDB
type RaftBadgerService struct {
	raft   *RaftManager
	db     *BadgerDB
	nodeID string
}

// NewRaftBadgerService 初始化服务
func NewRaftBadgerService(nodeID, raftDir, dbDir string, zapLogger *zap.SugaredLogger) (*RaftBadgerService, error) {
	raftMgr, err := NewRaftManager(nodeID, raftDir, zapLogger)
	if err != nil {
		return nil, err
	}

	db, err := NewBadgerDB(dbDir, zapLogger)
	if err != nil {
		return nil, err
	}

	return &RaftBadgerService{
		raft:   raftMgr,
		db:     db,
		nodeID: nodeID,
	}, nil
}

func (s *RaftBadgerService) Write(key, value []byte) error {
	// 1. 写入 BadgerDB
	err := s.db.Write(key, value)
	if err != nil {
		return err
	}
	// 2. 提交 Raft 日志（模拟）
	logData := fmt.Sprintf("Write Operation: Key=%s, Value=%s", key, value)
	return s.raft.ApplyLog([]byte(logData))
}

// Read 从 BadgerDB 中读取数据
func (s *RaftBadgerService) Read(key []byte) ([]byte, error) {
	return s.db.Read(key)
}

// Close 关闭服务
func (s *RaftBadgerService) Close() error {
	err1 := s.raft.raftNode.Shutdown()
	err2 := s.db.Close()
	return errors.Join(err1.Error(), err2)
}
