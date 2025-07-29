package metadata_db

import (
	"fmt"
	"github.com/goccy/go-json"
	"go.etcd.io/bbolt"
)

// Set stores a value in the specified bucket/key (using JSON serialization)
func Set[T any](db *MetadataDB, bucket, key string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal error: %w", err)
	}

	return db.internal.Update(func(tx *bbolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %w", err)
		}
		return bkt.Put([]byte(key), data)
	})
}

// Get retrieves a value by key and bucket into the given generic type
func Get[T any](db *MetadataDB, bucket, key string) (T, error) {
	var result T

	err := db.internal.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(bucket))
		if bkt == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}
		data := bkt.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key not found: %s", key)
		}
		return json.Unmarshal(data, &result)
	})

	return result, err
}

// Delete removes a key from a bucket
func Delete(db *MetadataDB, bucket, key string) error {
	return db.internal.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(bucket))
		if bkt == nil {
			return nil
		}
		return bkt.Delete([]byte(key))
	})
}

func ListKeys(db *MetadataDB, bucket string) ([]string, error) {
	var keys []string
	err := db.internal.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(bucket))
		if bkt == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}
		return bkt.ForEach(func(k, _ []byte) error {
			keys = append(keys, string(k))
			return nil
		})
	})
	return keys, err
}

func List[T any](db *MetadataDB, bucket string) ([]T, error) {
	var results []T
	err := db.internal.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(bucket))
		if bkt == nil {
			return fmt.Errorf("bucket %s not found", bucket)
		}
		return bkt.ForEach(func(_, v []byte) error {
			var val T
			if err := json.Unmarshal(v, &val); err != nil {
				return fmt.Errorf("unmarshal error: %w", err)
			}
			results = append(results, val)
			return nil
		})
	})
	return results, err
}

func Exists(db *MetadataDB, bucket, key string) (bool, error) {
	var exists bool
	err := db.internal.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket([]byte(bucket))
		if bkt == nil {
			exists = false
			return nil
		}
		val := bkt.Get([]byte(key))
		exists = val != nil
		return nil
	})
	return exists, err
}

func DeleteBucket(db *MetadataDB, bucket string) error {
	return db.internal.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket([]byte(bucket))
	})
}

func WithTx(db *MetadataDB, writable bool, fn func(tx *bbolt.Tx) error) error {
	if writable {
		return db.internal.Update(fn)
	} else {
		return db.internal.View(fn)
	}
}
