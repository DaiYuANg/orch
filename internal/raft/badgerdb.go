package raft

import (
	"log/slog"

	"github.com/dgraph-io/badger/v3"
)

// badgerDB 包装器
type badgerDB struct {
	db     *badger.DB
	logger *slog.Logger
}

func newBadgerDB(path string, logger *slog.Logger) (*badgerDB, error) {

	option :=
		badger.DefaultOptions(path).
			WithLogger(&badgerLogger{logger: logger}).
			WithMetricsEnabled(true)

	db, err := badger.Open(option)
	if err != nil {
		return nil, err
	}
	return &badgerDB{db: db, logger: logger}, nil
}

// Write 存储数据
func (b *badgerDB) Write(key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Read 读取数据
func (b *badgerDB) Read(key []byte) ([]byte, error) {
	var valCopy []byte
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		valCopy, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return valCopy, nil
}

// Close 关闭数据库连接
func (b *badgerDB) Close() error {
	return b.db.Close()
}
