package raft

import (
	"errors"
	"fmt"
	"io"

	"github.com/dgraph-io/badger/v3"
	"github.com/goccy/go-json"
	"github.com/hashicorp/raft"
	"go.etcd.io/bbolt"
)

type FSM struct {
	badger *badgerDB
	bblot  *bbolt.DB
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	cmd, err := decodeCommand(log.Data)
	if err != nil {
		return err
	}

	switch cmd.Type {
	case commandTypeSet:
		return f.applySet(cmd)
	case commandTypeDelete:
		return f.applyDelete(cmd)
	default:
		return fmt.Errorf("unsupported raft command type: %s", cmd.Type)
	}
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	payload := snapshotPayload{
		Buckets: map[string]map[string][]byte{},
	}

	err := f.bblot.View(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, bucket *bbolt.Bucket) error {
			items := map[string][]byte{}
			if err := bucket.ForEach(func(k, v []byte) error {
				items[string(k)] = append([]byte(nil), v...)
				return nil
			}); err != nil {
				return err
			}
			if len(items) > 0 {
				payload.Buckets[string(name)] = items
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return &fsmSnapshot{payload: payload}, nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	payload := snapshotPayload{}
	if err := json.NewDecoder(rc).Decode(&payload); err != nil {
		return err
	}

	if err := f.bblot.Update(func(tx *bbolt.Tx) error {
		var buckets [][]byte
		if err := tx.ForEach(func(name []byte, _ *bbolt.Bucket) error {
			buckets = append(buckets, append([]byte(nil), name...))
			return nil
		}); err != nil {
			return err
		}
		for _, name := range buckets {
			if err := tx.DeleteBucket(name); err != nil {
				return err
			}
		}

		for bucketName, items := range payload.Buckets {
			bucket, err := tx.CreateBucketIfNotExists([]byte(bucketName))
			if err != nil {
				return err
			}
			for key, value := range items {
				if err := bucket.Put([]byte(key), value); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := f.badger.db.DropAll(); err != nil {
		return err
	}
	for bucketName, items := range payload.Buckets {
		for key, value := range items {
			if err := f.badger.Write(cacheKey(bucketName, key), value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *FSM) Read(bucket, key string) ([]byte, error) {
	value, err := f.badger.Read(cacheKey(bucket, key))
	if err == nil {
		return value, nil
	}
	if err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return nil, err
	}

	value, err = f.readFromBolt(bucket, key)
	if err != nil {
		return nil, err
	}
	if cacheErr := f.badger.Write(cacheKey(bucket, key), value); cacheErr != nil {
		return nil, cacheErr
	}
	return value, nil
}

func (f *FSM) applySet(cmd command) error {
	if err := f.bblot.Update(func(tx *bbolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(cmd.Bucket))
		if err != nil {
			return err
		}
		return bucket.Put([]byte(cmd.Key), cmd.Value)
	}); err != nil {
		return err
	}
	return f.badger.Write(cacheKey(cmd.Bucket, cmd.Key), cmd.Value)
}

func (f *FSM) applyDelete(cmd command) error {
	if err := f.bblot.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(cmd.Bucket))
		if bucket == nil {
			return nil
		}
		return bucket.Delete([]byte(cmd.Key))
	}); err != nil {
		return err
	}
	return f.badger.Delete(cacheKey(cmd.Bucket, cmd.Key))
}

func (f *FSM) readFromBolt(bucket, key string) ([]byte, error) {
	var value []byte
	err := f.bblot.View(func(tx *bbolt.Tx) error {
		target := tx.Bucket([]byte(bucket))
		if target == nil {
			return fmt.Errorf("bucket %q not found", bucket)
		}
		raw := target.Get([]byte(key))
		if raw == nil {
			return fmt.Errorf("key %q not found", key)
		}
		value = append([]byte(nil), raw...)
		return nil
	})
	return value, err
}

func (f *FSM) close() error {
	var err1 error
	var err2 error
	if f.bblot != nil {
		err1 = f.bblot.Close()
	}
	if f.badger != nil {
		err2 = f.badger.Close()
	}
	return errors.Join(err1, err2)
}

type snapshotPayload struct {
	Buckets map[string]map[string][]byte `json:"buckets"`
}

type fsmSnapshot struct {
	payload snapshotPayload
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	raw, err := json.Marshal(f.payload)
	if err != nil {
		_ = sink.Cancel()
		return err
	}
	if _, err = sink.Write(raw); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (f *fsmSnapshot) Release() {}
