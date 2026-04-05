package wal

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

type WalReader struct {
	file *os.File
}

// Sentinel errors for recovery-level decisions.
var ErrChecksumMismatch = errors.New("checksum mismatch")
var ErrPartialEntry = errors.New("partial entry")

func NewWalReader(path string) (*WalReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &WalReader{file: file}, nil
}

func (r *WalReader) Close() error {
	return r.file.Close()
}

// File returns the underlying os.File so callers (e.g. Recovery) can inspect
// offsets, truncate, and sync as needed.
func (r *WalReader) File() *os.File {
	return r.file
}

func (r *WalReader) ReadEntry() (*Entry, error) {
	// 1. Read Op (1 byte)
	var op uint8
	if err := binary.Read(r.file, binary.BigEndian, &op); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}

	// 2. Read Lengths (4+4 bytes)
	var keyLen, valLen uint32
	if err := binary.Read(r.file, binary.BigEndian, &keyLen); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}
	if err := binary.Read(r.file, binary.BigEndian, &valLen); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}

	// 3. Read Key/Value raw bytes
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(r.file, key); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}
	value := make([]byte, valLen)
	if _, err := io.ReadFull(r.file, value); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}

	// 4. Read Timestamp (8 bytes)
	var timestamp int64
	if err := binary.Read(r.file, binary.BigEndian, &timestamp); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}

	// 5. Verification — Calculate Checksum over what we read
	hash := crc32.NewIEEE()
	binary.Write(hash, binary.BigEndian, op)
	binary.Write(hash, binary.BigEndian, keyLen)
	binary.Write(hash, binary.BigEndian, valLen)
	hash.Write(key)
	hash.Write(value)
	binary.Write(hash, binary.BigEndian, timestamp)
	expectedChecksum := hash.Sum32()

	// 6. Read and Compare stored Checksum (4 bytes)
	var actualChecksum uint32
	if err := binary.Read(r.file, binary.BigEndian, &actualChecksum); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, ErrPartialEntry
		}
		return nil, err
	}

	if expectedChecksum != actualChecksum {
		return nil, ErrChecksumMismatch
	}

	// 7. Success! Return the structured Entry
	return &Entry{
		Op:        op,
		Key:       key,
		Value:     value,
		Timestamp: timestamp,
		Checksum:  actualChecksum,
	}, nil
}
