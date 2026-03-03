package raft

import (
	"log/slog"
)

func newFsm(raftDir string, logger *slog.Logger) (*FSM, error) {

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
