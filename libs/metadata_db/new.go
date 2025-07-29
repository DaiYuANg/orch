package metadata_db

import (
	"fmt"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
	"os"
	"time"
)

// Open or create the BoltDB database
func NewMetadataDB(path string, logger *zap.SugaredLogger) (*MetadataDB, error) {
	if err := os.MkdirAll(getDir(path), 0700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open bolt db: %w", err)
	}

	return &MetadataDB{internal: db, logger: logger}, nil
}

func getDir(path string) string {
	if idx := len(path) - len(".db"); idx > 0 {
		return path[:idx]
	}
	return "."
}
