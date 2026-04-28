package raftsvc

import (
	"context"

	"github.com/arcgolabs/storx/badgerx"
	"github.com/arcgolabs/storx/codec"
	"github.com/arcgolabs/storx/keycodec"
	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// storxBadgerLogStore implements hraft.LogStore using storx badgerx with JSON codecs.
type storxBadgerLogStore struct {
	ns *badgerx.Namespace[uint64, hraft.Log]
}

func newStorxBadgerLogStore(db *badgerx.DB) *storxBadgerLogStore {
	ns := badgerx.NewNamespaceWithDB(
		db,
		"orch/raft/logs/",
		keycodec.Uint64BE(),
		codec.JSON[hraft.Log](),
	)
	return &storxBadgerLogStore{ns: ns}
}

func bg() context.Context {
	return context.Background()
}

func (s *storxBadgerLogStore) FirstIndex() (uint64, error) {
	e, ok, err := s.ns.First(bg())
	if err != nil || !ok {
		return 0, err
	}
	return e.Key, nil
}

func (s *storxBadgerLogStore) LastIndex() (uint64, error) {
	e, ok, err := s.ns.Last(bg())
	if err != nil || !ok {
		return 0, err
	}
	return e.Key, nil
}

func (s *storxBadgerLogStore) GetLog(index uint64, log *hraft.Log) error {
	v, ok, err := s.ns.Get(bg(), index)
	if err != nil {
		return err
	}
	if !ok {
		return hraft.ErrLogNotFound
	}
	*log = v
	return nil
}

func (s *storxBadgerLogStore) StoreLog(log *hraft.Log) error {
	if log == nil {
		return oopsx.B("raft").New("nil log")
	}
	return s.ns.Set(bg(), log.Index, *log)
}

func (s *storxBadgerLogStore) StoreLogs(logs []*hraft.Log) error {
	for _, lg := range logs {
		if lg == nil {
			return oopsx.B("raft").New("nil log entry")
		}
		if err := s.ns.Set(bg(), lg.Index, *lg); err != nil {
			return err
		}
	}
	return nil
}

func (s *storxBadgerLogStore) DeleteRange(min, max uint64) error {
	if max < min {
		return nil
	}
	keys, err := s.ns.Keys(bg(),
		badgerx.WithStart(min),
		badgerx.WithEnd(max),
		badgerx.WithLimit[uint64](0),
	)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := s.ns.Delete(bg(), k); err != nil {
			return err
		}
	}
	return nil
}

var _ hraft.LogStore = (*storxBadgerLogStore)(nil)
