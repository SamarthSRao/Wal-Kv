package wal

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"os"
	"sync"
)

type WalWriter struct {
	file   *os.File
	mu     sync.Mutex
	closed bool
	path   string
}

func NewWalWriter(path string) (*WalWriter, error) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &WalWriter{file: file,
		path: path,
	}, nil
}

func (w *WalWriter) Append(e *Entry) error {
	w.mu.Lock()

	defer w.mu.Unlock()
	if w.closed {
		return os.ErrClosed
	}
	buf := new(bytes.Buffer)

	err := binary.Write(buf, binary.BigEndian, e.Op)

	if err != nil {
		return err
	}

	if err := binary.Write(buf, binary.BigEndian, uint32(len(e.Key))); err != nil {
		return err
	}

	if err := binary.Write(buf, binary.BigEndian, uint32(len(e.Value))); err != nil {
		return err
	}

	// Raw Key and Value bytes
	buf.Write(e.Key)
	buf.Write(e.Value)

	if err := binary.Write(buf, binary.BigEndian, e.Timestamp); err != nil {
		return err
	}

	checksum := crc32.ChecksumIEEE(buf.Bytes())
	if err := binary.Write(buf, binary.BigEndian, checksum); err != nil {
		return err
	}

	if _, err := w.file.Write(buf.Bytes()); err != nil {
		return err
	}

	return w.file.Sync()
}

func (w *WalWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return os.ErrClosed
	}
	w.closed = true
	return w.file.Close()
}
