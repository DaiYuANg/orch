package raft

import (
	"errors"
	"fmt"
	"go.uber.org/zap"
)

// Service 服务封装，结合 Raft 和 badgerDB
type Service struct {
	raft   *Manager
	nodeID string
}

// NewRaftBadgerService 初始化服务
func NewRaftBadgerService(nodeID, raftDir string, zapLogger *zap.SugaredLogger) (*Service, error) {
	raftMgr, err := NewRaftManager(nodeID, raftDir, zapLogger)
	if err != nil {
		return nil, err
	}

	return &Service{
		raft:   raftMgr,
		nodeID: nodeID,
	}, nil
}

func (s *Service) Write(key, value []byte) error {
	logData := fmt.Sprintf("Write Operation: Key=%s, Value=%s", key, value)
	return s.raft.ApplyLog([]byte(logData))
}

// Close 关闭服务
func (s *Service) Close() error {
	err1 := s.raft.raftNode.Shutdown()
	return errors.Join(err1.Error())
}
