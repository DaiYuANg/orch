package raft

import (
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"path/filepath"
)

func newBblot(raftDir string, logger *zap.SugaredLogger) (*bbolt.DB, error) {
	path := filepath.Join(raftDir, "metadata_db.db")
	logger.Debugf("Bblot path:%s", path)
	options := bbolt.DefaultOptions
	db, err := bbolt.Open(path, 0600, options)
	if err != nil {
		return nil, err
	}
	return db, nil
}
