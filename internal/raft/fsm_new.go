package raft

import (
	"go.uber.org/zap"
)

func newFsm(raftDir string, logger *zap.SugaredLogger) (*FSM, error) {

	badger, err := newBadgerDB(raftDir, logger)
	if err != nil {
		return nil, err
	}

	bblot, err := newBblot(raftDir, logger)
	if err != nil {
		return nil, err
	}

	return &FSM{
		badger: badger,
		bblot:  bblot,
	}, nil
}
