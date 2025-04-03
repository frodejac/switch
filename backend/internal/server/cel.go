package server

import (
	"encoding/json"

	"github.com/dgraph-io/badger/v4"
)

// preWarmCache loads all flags and pre-compiles their CEL expressions
func (s *Server) preWarmCache() error {
	return s.store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var flagData struct {
				Value      interface{} `json:"value"`
				Expression string      `json:"expression,omitempty"`
			}
			if err := json.Unmarshal(value, &flagData); err != nil {
				continue // Skip invalid entries
			}

			// Skip if no expression
			if flagData.Expression == "" {
				continue
			}

			// Compile and cache the program
			ast, issues := s.celEnv.Compile(flagData.Expression)
			if issues != nil && issues.Err() != nil {
				continue // Skip invalid expressions
			}

			program, err := s.celEnv.Program(ast)
			if err != nil {
				continue // Skip programs that fail to compile
			}

			// Store in cache
			s.celCache.Store(string(item.Key()), program)
		}

		return nil
	})
}
