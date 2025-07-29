package store

import (
	"github.com/dgraph-io/badger/v3"
	"go.uber.org/zap"
)

// BadgerDB 包装器
type BadgerDB struct {
	db     *badger.DB
	logger *zap.SugaredLogger
}

// NewBadgerDB 初始化 BadgerDB 存储
func NewBadgerDB(path string, logger *zap.SugaredLogger) (*BadgerDB, error) {

	option :=
		badger.DefaultOptions(path).
			WithLogger(&badgerLogger{logger: logger}).
			WithMetricsEnabled(true)

	db, err := badger.Open(option)
	if err != nil {
		return nil, err
	}
	return &BadgerDB{db: db, logger: logger}, nil
}

// Write 存储数据
func (b *BadgerDB) Write(key, value []byte) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, value)
	})
}

// Read 读取数据
func (b *BadgerDB) Read(key []byte) ([]byte, error) {
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
func (b *BadgerDB) Close() error {
	return b.db.Close()
}
