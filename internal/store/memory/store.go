package memory

import "github.com/arcgolabs/collectionx/mapping"

type Store struct {
	items *mapping.ConcurrentMap[string, any]
}

func New() *Store {
	return &Store{
		items: mapping.NewConcurrentMap[string, any](),
	}
}

func (s *Store) Get(key string) (any, bool) {
	return s.items.Get(key)
}

func (s *Store) Set(key string, value any) {
	s.items.Set(key, value)
}
