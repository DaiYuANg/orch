package raft

import (
	"log/slog"
	"path/filepath"

	"go.etcd.io/bbolt"
)

func newBblot(raftDir string, logger *slog.Logger) (*bbolt.DB, error) {
	path := filepath.Join(raftDir, "registry.db")
	logger.Debug("bbolt path", "path", path)
	options := bbolt.DefaultOptions
	db, err := bbolt.Open(path, 0600, options)
	if err != nil {
		return nil, err
	}
	return db, nil
}
