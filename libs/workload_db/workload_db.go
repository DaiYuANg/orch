package workload_db

import "github.com/dgraph-io/badger/v3"

type WorkloadDB struct {
	internal *badger.DB
}
