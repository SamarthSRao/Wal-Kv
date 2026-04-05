package kv

import (
	"bytes"
	"fmt"
	"testing"
)

func TestSnapshotAndLoad(t *testing.T) {
	s1 := NewStore()

	// 1. Populate the store with sample data.
	data := map[string]string{
		"user:1":    "John Doe",
		"user:2":    "Jane Smith",
		"config":    "{\"theme\": \"dark\"}",
		"last_seen": "2026-04-03",
		"empty":     "",
	}

	for k, v := range data {
		s1.Set(k, v)
	}

	// 2. Perform the Snapshot into an in-memory buffer.
	var buf bytes.Buffer
	s1.Mu.RLock()
	if err := s1.Snapshot(&buf); err != nil {
		s1.Mu.RUnlock()
		t.Fatalf("Snapshot failed: %v", err)
	}
	s1.Mu.RUnlock()

	// 3. Create a second empty store and Load from the buffer.
	s2 := NewStore()
	s2.Mu.Lock()
	if err := s2.Load(&buf); err != nil {
		s2.Mu.Unlock()
		t.Fatalf("Load failed: %v", err)
	}
	s2.Mu.Unlock()

	// 4. Verify the data in the second store matches the original data.
	for k, expected := range data {
		actual, ok := s2.Get(k)
		if !ok {
			t.Errorf("expected key %q to exist in s2, but it was missing", k)
			continue
		}
		if actual != expected {
			t.Errorf("mismatch for key %q: expected %q, got %q", k, expected, actual)
		}
	}

	// 5. Verify the counts match exactly.
	s2.Mu.RLock()
	actualCount := len(s2.m)
	s2.Mu.RUnlock()
	if actualCount != len(data) {
		t.Errorf("expected count %d, got %d", len(data), actualCount)
	}
}

func TestLargeSnapshot(t *testing.T) {
	s1 := NewStore()
	count := 1000

	// Populate with 1000 items.
	for i := 0; i < count; i++ {
		s1.Set(fmt.Sprintf("key:%d", i), fmt.Sprintf("val:%d", i))
	}

	var buf bytes.Buffer
	s1.Mu.RLock()
	if err := s1.Snapshot(&buf); err != nil {
		s1.Mu.RUnlock()
		t.Fatalf("Snapshot failed: %v", err)
	}
	s1.Mu.RUnlock()

	s2 := NewStore()
	s2.Mu.Lock()
	if err := s2.Load(&buf); err != nil {
		s2.Mu.Unlock()
		t.Fatalf("Load failed: %v", err)
	}
	s2.Mu.Unlock()

	for i := 0; i < count; i++ {
		k := fmt.Sprintf("key:%d", i)
		expected := fmt.Sprintf("val:%d", i)
		if val, _ := s2.Get(k); val != expected {
			t.Fatalf("mismatch at key %q", k)
		}
	}
}
