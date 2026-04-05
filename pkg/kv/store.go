package kv

import (
	"encoding/binary"
	"io"
	"sync"
)

// Store is a simple in-memory key-value store.
// It is safe for concurrent use.
type Store struct {
	Mu             sync.RWMutex
	m              map[string]string
	LastAppliedLSN int64 // The WAL offset of the last applied entry
}

// NewStore creates and returns an initialized Store.
func NewStore() *Store {
	return &Store{
		m: make(map[string]string),
	}
}

// Set stores the provided value for the given key.
func (s *Store) Set(key, value string) {
	s.Mu.Lock()
	s.m[key] = value
	s.Mu.Unlock()
}

// Delete removes a key from the store.
func (s *Store) Delete(key string) {
	s.Mu.Lock()
	delete(s.m, key)
	s.Mu.Unlock()
}

// Get returns the value for a key and a boolean indicating whether the key exists.
func (s *Store) Get(key string) (string, bool) {
	s.Mu.RLock()
	v, ok := s.m[key]
	s.Mu.RUnlock()
	return v, ok
}

// Snapshot writes the entire store state to the provided writer in a simple binary format.
func (s *Store) Snapshot(w io.Writer) error {
	// Let the caller handle locking to allow for larger atomic operations.

	// 1. Write the LastAppliedLSN.
	if err := binary.Write(w, binary.BigEndian, s.LastAppliedLSN); err != nil {
		return err
	}

	// 2. Write the count of items in the map.
	count := uint32(len(s.m))
	if err := binary.Write(w, binary.BigEndian, count); err != nil {
		return err
	}

	// 2. Iterate and write each key-value pair.
	for k, v := range s.m {
		keyLen := uint32(len(k))
		valLen := uint32(len(v))

		if err := binary.Write(w, binary.BigEndian, keyLen); err != nil {
			return err
		}
		if _, err := w.Write([]byte(k)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, valLen); err != nil {
			return err
		}
		if _, err := w.Write([]byte(v)); err != nil {
			return err
		}
	}
	return nil
}

// Load clears the current store state and reads a new state from the provided reader.
func (s *Store) Load(r io.Reader) error {
	// Let the caller handle locking to allow for larger atomic operations.

	// 1. Read the LastAppliedLSN.
	var lsn int64
	if err := binary.Read(r, binary.BigEndian, &lsn); err != nil {
		return err
	}

	// 2. Read the total count of items.
	var count uint32
	if err := binary.Read(r, binary.BigEndian, &count); err != nil {
		return err
	}

	// 3. Reset the map and LSN.
	s.m = make(map[string]string, count)
	s.LastAppliedLSN = lsn

	// 3. Read each item and store it in the map.
	for i := uint32(0); i < count; i++ {
		var keyLen uint32
		if err := binary.Read(r, binary.BigEndian, &keyLen); err != nil {
			return err
		}
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(r, key); err != nil {
			return err
		}

		var valLen uint32
		if err := binary.Read(r, binary.BigEndian, &valLen); err != nil {
			return err
		}
		value := make([]byte, valLen)
		if _, err := io.ReadFull(r, value); err != nil {
			return err
		}

		s.m[string(key)] = string(value)
	}
	return nil
}

