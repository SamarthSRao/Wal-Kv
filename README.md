# Wal-Kv

A small Go module that provides a write-ahead log (WAL) writer intended for a KV-store style workload.

This repository currently contains a minimal WAL writer implementation under `pkg/wal`.

## What this project does

Wal-Kv provides the building blocks for writing durable key-value updates to a write-ahead log. A WAL is an append-only file that records changes before they are applied to the main data store. If the process crashes, the WAL can later be replayed to recover the latest updates.

In this repository, the WAL writer is responsible for:

- opening or creating the WAL file
- serializing entries into a binary format
- appending entries safely from concurrent callers
- forcing writes to disk so data is not only kept in memory

## Flow diagram

```text
Client code
   |
   v
Create WAL writer
   |
   v
Build WAL entry
(op, key, value, timestamp)
   |
   v
Call Append(entry)
   |
   v
Lock writer mutex
   |
   v
Serialize entry into bytes
   |
   +--> write op
   +--> write key length
   +--> write value length
   +--> write key bytes
   +--> write value bytes
   +--> write timestamp
   +--> compute CRC32 checksum
   +--> append checksum
   |
   v
Write bytes to WAL file
   |
   v
Sync file to disk
   |
   v
Unlock mutex and return
```

## Detailed flow

### 1. Open or create the WAL file

`NewWalWriter(path)` opens the file with append permissions. If the file does not exist, it is created. The writer keeps the file handle open so subsequent appends are fast and sequential.

### 2. Build an entry

The application creates a `wal.Entry` with:

- `Op`: the operation type, such as `wal.OpSet` or `wal.OpDel`
- `Key`: the key being modified
- `Value`: the value associated with the key
- `Timestamp`: the time the operation was recorded

### 3. Serialize the entry

When `Append()` is called, the writer converts the entry into a binary payload. The fields are written in big-endian order:

1. operation code
2. key length
3. value length
4. key bytes
5. value bytes
6. timestamp
7. CRC32 checksum

This format makes the log compact and deterministic.

### 4. Protect concurrent writes

A mutex is used around the append operation. This ensures that if multiple goroutines try to write at the same time, their entries do not interleave and corrupt the log.

### 5. Compute integrity checksum

Before writing the final payload, the writer computes a CRC32 checksum over the serialized bytes. The checksum helps detect accidental corruption when the file is later read back.

### 6. Write and sync to disk

The writer writes the complete entry to the WAL file and immediately calls `Sync()`.

This is important because it pushes buffered data to disk, improving crash durability. The trade-off is that each append may be slower than batching writes.

### 7. Close the writer

When the caller is done, `Close()` marks the writer as closed and closes the file descriptor. After this, further appends return `os.ErrClosed`.

## Features

- Append-only WAL file writer (`WalWriter`)
- Simple entry format with operation type, key/value lengths, raw key/value bytes, timestamp, and CRC32 checksum
- Thread-safe appends via an internal mutex
- Flushes data to disk on every append (`fsync` via `file.Sync()`)

## Package layout

- `pkg/wal/entry.go`: WAL entry structure and operation constants
- `pkg/wal/writer.go`: WAL writer that appends entries to a file

## Entry format (writer)

`Append()` serializes an entry in big-endian order as:

1. `op` (uint8)
2. `key_len` (uint32)
3. `value_len` (uint32)
4. `key` (bytes)
5. `value` (bytes)
6. `timestamp` (int64)
7. `checksum` (uint32, CRC32 of all prior serialized bytes for the entry)

Notes:
- The current code writes the checksum, but does not include a reader/validator yet.
- `Entry.Checksum` is not used by the writer during serialization (the checksum is computed from the serialized bytes).

## Install

```bash
go get github.com/samarthsrao/wal-kv
```

## Usage

```go
package main

import (
	"time"

	"github.com/samarthsrao/wal-kv/pkg/wal"
)

func main() {
	w, err := wal.NewWalWriter("./data.wal")
	if err != nil {
		panic(err)
	}
	defer w.Close()

	e := &wal.Entry{
		Op:        wal.OpSet,
		Key:       []byte("name"),
		Value:     []byte("sam"),
		Timestamp: time.Now().UnixNano(),
	}

	if err := w.Append(e); err != nil {
		panic(err)
	}
}
```

## Development

```bash
go test ./...
```

## License

No license file is currently present in this repository. If you intend others to use or contribute to this code, consider adding a `LICENSE`.
