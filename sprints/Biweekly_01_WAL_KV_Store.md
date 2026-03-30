# Biweekly Project 1 — WAL-Backed KV Store
## Write-Ahead Log from Scratch

**Timeline:** Weeks 1–2  
**Language:** Go  
**What it mirrors:** PostgreSQL WAL · SQLite WAL mode · RocksDB Write Path  

---

## 1. What This Teaches

WAL design, crash recovery, and log compaction — the exact internals PostgreSQL, SQLite, and RocksDB use for durability. Most engineers use databases that depend on a WAL without ever understanding why. After this project, you can explain in an interview exactly what happens between a `COMMIT` statement and the data being safe on disk.

---

## 2. The Problem It Solves

Your existing in-memory KV store loses all data on restart. This version adds durability using a Write-Ahead Log — the same mechanism PostgreSQL uses internally. Every write appends a log entry to an append-only file **before** modifying the in-memory map. If the process crashes mid-write, the checksum catches the partial write on recovery.

---

## 3. What You Build

### 3.1 Components

| Component | Responsibility |
|---|---|
| WAL Writer | Append-only log with checksums. Every write durably recorded before in-memory update. |
| Recovery Engine | Replay WAL from last good checkpoint on startup. Store returns to exact pre-crash state. |
| Log Compaction | Snapshot current in-memory state to new file, delete all WAL entries before the snapshot. |
| HTTP API | `GET /key`, `PUT /key`, `DELETE /key` — same interface as the original in-memory store. |

### 3.2 WAL Entry Format

```
┌─────────────────────────────────────────────────────┐
│  op (1 byte) │ key_len (4 bytes) │ val_len (4 bytes) │
│  key (n bytes) │ value (m bytes)                     │
│  timestamp (8 bytes) │ checksum (4 bytes CRC32)      │
└─────────────────────────────────────────────────────┘
```

- **op:** `0x01` = SET, `0x02` = DEL
- **checksum:** CRC32 over all preceding bytes in the entry. If checksum doesn't match on read → partial write → stop replay at this entry.

### 3.3 Recovery Algorithm

```
On startup:
  1. Load latest snapshot file (if exists) → populate in-memory map
  2. Open WAL file at snapshot LSN (Log Sequence Number)
  3. For each entry: verify checksum → apply to map → advance LSN
  4. Stop at first checksum failure (partial write boundary)
  5. Store is now in exact pre-crash state
```

### 3.4 Log Compaction

WAL files grow unboundedly without compaction. The compaction mechanism:

1. Serialise current in-memory map to a new snapshot file (atomic rename)
2. Record the snapshot LSN
3. Delete all WAL segments written before the snapshot LSN
4. Same mechanism as Redis RDB + AOF hybrid

---

## 4. Key Concepts Demonstrated

- **Durability before acknowledgement** — `fsync()` called before returning success to the caller. Without this, the OS page cache can lose data on power failure.
- **Idempotent recovery** — replaying the same WAL twice produces the same result. This is why WAL entries must be deterministic.
- **Checksum for partial write detection** — a 4GB write to a spinning disk can be interrupted at any byte boundary. The checksum catches this.
- **Snapshot + WAL hybrid** — pure WAL grows forever. Pure snapshots lose recent writes. The combination is the correct production pattern (Redis, PostgreSQL, SQLite all use it).

---

## 5. Implementation Checklist

- [ ] `WALWriter`: `Append(entry) error` — writes entry + calls `fsync()`
- [ ] `WALReader`: `ReadAll() ([]Entry, error)` — reads and validates checksums
- [ ] `Recovery`: `Replay(snapshot, wal) (map[string]string, error)`
- [ ] `Compaction`: `Compact(map, lsn) error` — atomic snapshot write
- [ ] Crash simulation test: kill process mid-write, verify recovery is correct
- [ ] `go test -race ./...` passes
- [ ] Benchmark: throughput with `fsync` vs without (document the durability/performance tradeoff)

---

## 6. Benchmarks to Document

| Metric | Target |
|---|---|
| Write throughput (fsync on) | Baseline — document the number |
| Write throughput (fsync off) | Should be 10–100x faster — document the tradeoff |
| Recovery time (1M entries) | < 5 seconds |
| Compaction time (1M entries) | < 2 seconds |
| Storage overhead vs raw data | Document WAL entry size vs raw key/value size |

---

## 7. Interview Value

- **Uber / Rippling system design:** *"How does PostgreSQL guarantee durability?"* → You can answer from implementation depth, not just theory.
- **Zomato / Swiggy technical round:** *"What happens if your database server loses power mid-transaction?"* → WAL + fsync + recovery replay.
- **DoorDash DevEx:** *"How would you make a KV store crash-safe?"* → This project is the answer.

---

## 8. ADR to Write

**"WAL vs Snapshot-only vs No persistence"**  
Decision: WAL + periodic snapshots.  
Alternatives considered: snapshot-only (loses recent writes), no persistence (restart loses everything).  
Tradeoff: fsync adds ~10ms latency per write but guarantees D in ACID.
