# Wal-Kv

A small Go module that provides a write-ahead log (WAL) writer intended for a KV-store style workload.

This repository currently contains a minimal WAL writer implementation under `pkg/wal`.

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
