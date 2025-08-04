package metadata_db

import (
	"bytes"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/samber/lo"
	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

type Repository[T any] struct {
	db     *bbolt.DB
	bucket string
	log    *zap.SugaredLogger
}

func NewRepository[T any](db *bbolt.DB, bucket string) *Repository[T] {
	return &Repository[T]{db: db, bucket: bucket}
}

func (r *Repository[T]) Set(key string, value T) error {
	bts, err := r.encode(value)
	if err != nil {
		return err
	}
	return r.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(r.bucket))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), bts)
	})
}
func (r *Repository[T]) Get(key string) (T, error) {
	var zero T
	var value T
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := r.getBucket(tx)
		if err != nil {
			return err
		}
		v := b.Get([]byte(key))
		if v == nil {
			return fmt.Errorf("key not found")
		}
		value, err = r.decode(v)
		return err
	})
	if err != nil {
		return zero, err
	}
	return value, nil
}

func (r *Repository[T]) Delete(key string) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Delete([]byte(key))
	})
}

func (r *Repository[T]) Exists(key string) (bool, error) {
	var exists bool
	err := r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		v := b.Get([]byte(key))
		exists = v != nil
		return nil
	})
	return exists, err
}

func (r *Repository[T]) ListKeys() ([]string, error) {
	var keys []string
	err := r.db.View(func(tx *bbolt.Tx) error {
		b, err := r.getBucket(tx)
		if err != nil {
			return err
		}
		return b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})
	return lo.Uniq(keys), err
}

func (r *Repository[T]) WithTx(fn func(tx *bbolt.Tx) error) error {
	return r.db.Update(fn)
}

func (r *Repository[T]) Count() (int, error) {
	var count int
	err := r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.ForEach(func(k, v []byte) error {
			count++
			return nil
		})
	})
	return count, err
}

func (r *Repository[T]) Update(key string, value T) error {
	bts, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.Put([]byte(key), bts)
	})
}

func (r *Repository[T]) BatchSet(items map[string]T) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(r.bucket))
		if err != nil {
			return err
		}
		for key, value := range items {
			bts, err := json.Marshal(value)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(key), bts); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository[T]) BatchDelete(keys []string) error {
	return r.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		for _, key := range keys {
			if err := b.Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository[T]) ForEach(fn func(key string, value T) error) error {
	return r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket not found")
		}
		return b.ForEach(func(k, v []byte) error {
			var value T
			if err := json.Unmarshal(v, &value); err != nil {
				return err
			}
			return fn(string(k), value)
		})
	})
}

func (r *Repository[T]) ForEachByPrefix(prefix string, fn func(key string, value T) error) error {
	return r.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(r.bucket))
		if b == nil {
			return fmt.Errorf("bucket %q not found", r.bucket)
		}
		c := b.Cursor()
		seek := []byte(prefix)
		for k, v := c.Seek(seek); k != nil && bytes.HasPrefix(k, seek); k, v = c.Next() {
			var t T
			if err := json.Unmarshal(v, &t); err != nil {
				return fmt.Errorf("prefix unmarshal (%q): %w", k, err)
			}
			if err := fn(string(k), t); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository[T]) ListKeysByPrefix(prefix string) ([]string, error) {
	var keys []string
	err := r.ForEachByPrefix(prefix, func(key string, _ T) error {
		keys = append(keys, key)
		return nil
	})
	return keys, err
}

func (r *Repository[T]) ListByPrefix(prefix string) ([]T, error) {
	var result []T
	err := r.ForEachByPrefix(prefix, func(_ string, v T) error {
		result = append(result, v)
		return nil
	})
	return result, err
}

func (r *Repository[T]) encode(v T) ([]byte, error) {
	return json.Marshal(v)
}

func (r *Repository[T]) decode(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

func (r *Repository[T]) getBucket(tx *bbolt.Tx) (*bbolt.Bucket, error) {
	b := tx.Bucket([]byte(r.bucket))
	if b == nil {
		return nil, fmt.Errorf("bucket %q not found", r.bucket)
	}
	return b, nil
}
