package raft

import (
	"errors"
	"fmt"
	"github.com/hashicorp/memberlist"
	"go.uber.org/zap"
)

// Service 服务封装，结合 Raft 和 badgerDB
type Service struct {
	raft   *Manager
	nodeID string
	member *memberlist.Memberlist
	logger *zap.SugaredLogger
}

// NewRaftBadgerService 初始化服务
func NewRaftBadgerService(nodeID, raftDir string, logger *zap.SugaredLogger) (*Service, error) {
	raftMgr, err := NewRaftManager(nodeID, raftDir, logger)
	if err != nil {
		return nil, err
	}
	list, err := memberlist.Create(memberlist.DefaultLocalConfig())
	if err != nil {
		return nil, err
	}

	logger.Infof("member %v", list.ProtocolVersion())
	return &Service{
		raft:   raftMgr,
		nodeID: nodeID,
		member: list,
	}, nil
}

func (s *Service) Write(key, value []byte) error {
	logData := fmt.Sprintf("Write Operation: Key=%s, Value=%s", key, value)
	return s.raft.ApplyLog([]byte(logData))
}

// Close 关闭服务
func (s *Service) Close() error {
	err1 := s.raft.raftNode.Shutdown()
	err2 := s.member.Shutdown()
	return errors.Join(err1.Error(), err2)
}
