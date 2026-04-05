package wal

import (
	"os"
	"testing"
	"time"

	"github.com/samarthsrao/wal-kv/pkg/kv"
)

func TestWALRecovery(t *testing.T) {
	tmpFile := "test_wal_recovery.log"
	defer os.Remove(tmpFile)

	// Phase 1: Simulate writing to a fresh WAL
	writer, err := NewWalWriter(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create WAL writer: %v", err)
	}

	entries := []*Entry{
		{Op: OpSet, Key: []byte("user:1"), Value: []byte("John Doe"), Timestamp: time.Now().Unix()},
		{Op: OpSet, Key: []byte("user:2"), Value: []byte("Jane Doe"), Timestamp: time.Now().Unix()},
		{Op: OpDel, Key: []byte("user:1"), Timestamp: time.Now().Unix()}, // Delete first user
		{Op: OpSet, Key: []byte("user:3"), Value: []byte("Sam Smith"), Timestamp: time.Now().Unix()},
	}

	for _, e := range entries {
		if _, err := writer.Append(e); err != nil {
			t.Fatalf("Failed to append entry: %v", err)
		}
	}
	writer.Close()

	// Phase 2: Simulate a restart and recovery
	store := kv.NewStore()
	recovery, err := NewRecovery(tmpFile, store)
	if err != nil {
		t.Fatalf("Failed to create recovery engine: %v", err)
	}

	if err := recovery.Replay(); err != nil {
		t.Fatalf("Failed to replay WAL: %v", err)
	}

	// Phase 3: Verify the state of the store matches expectations
	if _, ok := store.Get("user:1"); ok {
		t.Errorf("Expected user:1 to be deleted")
	}

	val, ok := store.Get("user:2")
	if !ok || val != "Jane Doe" {
		t.Errorf("Expected user:2 to be Jane Doe, got %v", val)
	}

	val, ok = store.Get("user:3")
	if !ok || val != "Sam Smith" {
		t.Errorf("Expected user:3 to be Sam Smith, got %v", val)
	}
}
