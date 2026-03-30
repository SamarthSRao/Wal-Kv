package wal

const (
	OpSet uint8 = 0x01
	OpDel uint8 = 0x02
)

type Entry struct {
	Op        uint8
	Key       []byte
	Value     []byte
	Timestamp int64
	Checksum  uint32
}

func (e *Entry) Size() int {
	return 1 + 4 + 4 + 8 + 4 + len(e.Key) + len(e.Value)
}
