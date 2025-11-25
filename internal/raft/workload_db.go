package raft

import (
	"errors"

	"github.com/dgraph-io/badger/v3"
	"github.com/goccy/go-json"
)

type WorkloadDB struct {
	internal *badger.DB
}

type WorkloadRepo[T any] struct {
	DB        *badger.DB
	KeyPrefix []byte
}

func NewWorkloadRepo[T any](db *badger.DB, keyPrefix string) *WorkloadRepo[T] {
	return &WorkloadRepo[T]{
		DB:        db,
		KeyPrefix: []byte(keyPrefix),
	}
}

func (r *WorkloadRepo[T]) prefixed(key string) []byte {
	if len(r.KeyPrefix) == 0 {
		return []byte(key)
	}
	return append(r.KeyPrefix, []byte(":"+key)...)
}

// Set 或 Update
func (r *WorkloadRepo[T]) Set(key string, value T) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(r.prefixed(key), raw)
	})
}

func (r *WorkloadRepo[T]) Get(key string) (T, error) {
	var zero T
	rawKey := r.prefixed(key)
	err := r.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get(rawKey)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return err
			}
			return err
		}
		val, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &zero)
	})
	return zero, err
}

func (r *WorkloadRepo[T]) Exists(key string) (bool, error) {
	rawKey := r.prefixed(key)
	err := r.DB.View(func(txn *badger.Txn) error {
		_, e := txn.Get(rawKey)
		if e != nil && !errors.Is(e, badger.ErrKeyNotFound) {
			return e
		}
		return nil
	})
	return err == nil, nil
}

func (r *WorkloadRepo[T]) Delete(key string) error {
	rawKey := r.prefixed(key)
	return r.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete(rawKey)
	})
}

// 返回整个 DB 中（或限定前缀后）的所有 keys。
// 注意：对于整库扫描，性能差，建议配合分页或 stream API。

func (r *WorkloadRepo[T]) ListKeys() ([]string, error) {
	var list []string
	err := r.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 只要 key
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := r.KeyPrefix
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			k := it.Item().KeyCopy(nil)
			if len(prefix) > 0 {
				list = append(list, string(k[len(prefix)+1:])) // 跳过 "prefix:"
			} else {
				list = append(list, string(k))
			}
		}
		return nil
	})
	return list, err
}

// 遍历 key/value 并反序列化为 T。
func (r *WorkloadRepo[T]) ForEach(fn func(key string, rawJson T) error) error {
	return r.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := r.KeyPrefix
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.KeyCopy(nil)
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			var parsed T
			if err := json.Unmarshal(v, &parsed); err != nil {
				return err
			}
			var keyStr string
			if len(prefix) > 0 {
				keyStr = string(k[len(prefix)+1:])
			} else {
				keyStr = string(k)
			}
			if err := fn(keyStr, parsed); err != nil {
				return err
			}
		}
		return nil
	})
}

// 返回条目数（仅做 rough count）。
func (r *WorkloadRepo[T]) Count() (int, error) {
	count := 0
	err := r.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		prefix := r.KeyPrefix
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}
		return nil
	})
	return count, err
}

// 支持读写事务嵌套调用。
func (r *WorkloadRepo[T]) WithTx(fn func(txn *badger.Txn) error) error {
	return r.DB.Update(fn)
}

// 支持批量写操作。
func (r *WorkloadRepo[T]) BatchSet(items map[string]T) error {
	return r.DB.Update(func(txn *badger.Txn) error {
		for k, v := range items {
			raw, err := json.Marshal(v)
			if err != nil {
				return err
			}
			if err := txn.Set(r.prefixed(k), raw); err != nil {
				return err
			}
		}
		return nil
	})
}

// 支持前缀扫描，分页处理数据。
func (r *WorkloadRepo[T]) PrefixScan(prefixStr string, callback func(key string, val T) (stop bool, err error)) error {
	return r.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		rawPrefix := append(r.KeyPrefix, []byte(":"+prefixStr)...) // 或自定义规则
		rawKey := rawPrefix
		for it.Seek(rawKey); it.ValidForPrefix(rawPrefix); it.Next() {
			item := it.Item()
			v, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			var parsed T
			if err := json.Unmarshal(v, &parsed); err != nil {
				return err
			}
			fullKey := string(item.KeyCopy(nil))
			trimKey := fullKey[len(r.KeyPrefix)+1:]
			stop, err := callback(trimKey, parsed)
			if err != nil {
				return err
			}
			if stop {
				break
			}
		}
		return nil
	})
}
