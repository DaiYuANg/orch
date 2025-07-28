package store

import "github.com/dgraph-io/badger/v3"

// BadgerDB 包装器
type BadgerDB struct {
	db *badger.DB
}

// NewBadgerDB 初始化 BadgerDB 存储
func NewBadgerDB(path string) (*BadgerDB, error) {
	db, err := badger.Open(badger.DefaultOptions(path))
	if err != nil {
		return nil, err
	}
	return &BadgerDB{db: db}, nil
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
func (b *BadgerDB) Close() {
	if err := b.db.Close(); err != nil {
		log.Fatal("Failed to close BadgerDB:", err)
	}
}
