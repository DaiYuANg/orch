package metadata_db

import (
	"errors"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

type MetadataDB struct {
	internal *bbolt.DB
	logger   *zap.SugaredLogger
}

// Close closes the underlying DB
func (m *MetadataDB) Close() error {
	if m.internal == nil {
		return errors.New("db already closed")
	}
	return m.internal.Close()
}
