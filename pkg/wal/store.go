package wal

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/samarthsrao/wal-kv/pkg/kv"
)

// Recovery performs WAL replay and truncation on corrupted/partial tails.
type Recovery struct {
	path   string
	reader *WalReader
	store  *kv.Store
}

// NewRecovery creates a Recovery instance for the WAL at path and a target store.
// If the WAL file does not exist it will be created empty and opened for reading.
func NewRecovery(path string, store *kv.Store) (*Recovery, error) {
	r, err := NewWalReader(path)
	if err != nil {
		// If the file doesn't exist, create an empty one and reopen.
		if errors.Is(err, os.ErrNotExist) {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				return nil, err
			}
			_ = f.Close()
			r, err = NewWalReader(path)
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	return &Recovery{path: path, reader: r, store: store}, nil
}
func (r *Recovery) Compact() error {
	r.store.Mu.Lock()
	defer r.store.Mu.Unlock()

	// 1. Define paths.
	dir := filepath.Dir(r.path)
	snapshotPath := filepath.Join(dir, "snapshot.bin")
	tmpPath := snapshotPath + ".tmp"

	// 2. Clear old tmp file if exists.
	_ = os.Remove(tmpPath)

	// 3. Create a temporary file for the snapshot.
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath) // Clean up tmp on error
	}()

	// 4. Write the snapshot to the temporary file.
	// LastAppliedLSN is preserved in the snapshot.
	if err := r.store.Snapshot(tmpFile); err != nil {
		return err
	}

	// 5. Sync and close the temporary file.
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// 6. Rename the temporary file to the snapshot file (Atomic).
	if err := os.Rename(tmpPath, snapshotPath); err != nil {
		return err
	}

	// 7. Success! Now truncate the current WAL.
	// Note: In a production system we might close and reopen writer here.
	if err := os.Truncate(r.path, 0); err != nil {
		return err
	}

	// Reset LSN because the WAL is now empty.
	r.store.LastAppliedLSN = 0

	return nil
}

// Replay scans the WAL from the beginning, validates each entry, applies valid
// entries to the store, and stops at the first corrupted or partial entry.
// When a corrupted or partial entry is encountered the WAL file is truncated
// back to the last known good offset and Replay returns nil (recovery succeeded
// up to truncation point). Fatal I/O errors are returned.
func (r *Recovery) Replay() error {
	// 1. Check for snapshot.bin and load it if it exists.
	snapshotPath := filepath.Join(filepath.Dir(r.path), "snapshot.bin")
	if f, err := os.Open(snapshotPath); err == nil {
		defer f.Close()
		if err := r.store.Load(f); err != nil {
			return err
		}
	}

	// 2. Open WAL reader (or use existing if possible).
	// We need to ensure we have a fresh reader for the Replay operation.
	reader, err := NewWalReader(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No WAL to replay.
		}
		return err
	}
	defer reader.Close()

	f := reader.File()

	// If the snapshot tells us we already applied some LSN, we should seek.
	// However, if we truncate on compaction, the WAL starts at 0.
	// If the snapshot LSN > 0, and current WAL size is 0, we are good.
	// If current WAL size > 0, we replay everything from the WAL.
	
	// Start replaying from the beginning of the CURRENT WAL.
	// (Since we truncate on compaction, the current WAL entries are all new).

	for {
		// Record the starting offset for this entry so we know where to truncate
		// if the entry turns out to be partial or corrupted.
		start, err := f.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}

		entry, err := r.reader.ReadEntry()
		if err == nil {
			// Apply the entry to the store and track the LSN.
			r.store.LastAppliedLSN = start
			switch entry.Op {
			case OpSet:
				r.store.Set(string(entry.Key), string(entry.Value))
			case OpDel:
				r.store.Delete(string(entry.Key))
			}
			// Continue to next entry.
			continue
		}

		// Clean EOF: nothing more to read.
		if errors.Is(err, io.EOF) {
			return nil
		}

		// If it's a partial read (truncated tail) or checksum mismatch, truncate
		// the file back to the last good offset and return success.
		if errors.Is(err, ErrPartialEntry) || errors.Is(err, ErrChecksumMismatch) {
			if truncErr := f.Truncate(start); truncErr != nil {
				return truncErr
			}
			// Persist metadata so the truncation is durable.
			if syncErr := f.Sync(); syncErr != nil {
				return syncErr
			}
			return nil
		}

		// Propagate any other unexpected/fatal errors.
		return err
	}
}
