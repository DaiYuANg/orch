package raftsvc

import (
	"encoding/binary"
	"errors"

	"github.com/arcgolabs/storx/bboltx"
	"github.com/arcgolabs/storx/codec"
	"github.com/arcgolabs/storx/keycodec"
	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// storxBoltStableStore implements hraft.StableStore using storx bboltx with raw bytes values.
// Uint64 keys use big-endian 8-byte payloads (same layout as typical raft-bolt integrations).
type storxBoltStableStore struct {
	bkt *bboltx.Bucket[[]byte, []byte]
}

func newStorxBoltStableStore(db *bboltx.DB) *storxBoltStableStore {
	bkt := bboltx.NewBucketWithDB(
		db,
		"orch_raft_stable",
		keycodec.Bytes(),
		codec.Bytes(),
	)
	return &storxBoltStableStore{bkt: bkt}
}

// notFound must keep message exactly "not found": hashicorp/raft StableStore helpers compare err.Error().
func notFound() error {
	return errors.New("not found")
}

func (s *storxBoltStableStore) Set(key []byte, val []byte) error {
	return s.bkt.Put(bg(), cloneBytes(key), cloneBytes(val))
}

func (s *storxBoltStableStore) Get(key []byte) ([]byte, error) {
	v, ok, err := s.bkt.Get(bg(), cloneBytes(key))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound()
	}
	return cloneBytes(v), nil
}

func (s *storxBoltStableStore) SetUint64(key []byte, val uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return s.Set(key, buf)
}

func (s *storxBoltStableStore) GetUint64(key []byte) (uint64, error) {
	b, err := s.Get(key)
	if err != nil {
		return 0, err
	}
	if len(b) != 8 {
		return 0, oopsx.B("raft").New("invalid uint64 value")
	}
	return binary.BigEndian.Uint64(b), nil
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

var _ hraft.StableStore = (*storxBoltStableStore)(nil)
