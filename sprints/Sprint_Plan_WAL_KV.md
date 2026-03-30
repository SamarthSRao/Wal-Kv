# Project: WAL-Backed KV Store — Personal Sprint Plan

**Owner: Samarth**
**Timeline: Weeks 1–2 (Biweekly 01)**

> **Project Name: Wal-Kv** (Write-Ahead Log Powered Key-Value Store)
> 
> This sprint is about building the durability layer for your in-memory storage from scratch. By the end of Week 2, your store will survive sudden process kills, recover its state perfectly, and handle unbound log growth through atomic snapshots. You are implementing the "D" in ACID (Durability) just like the internals of PostgreSQL or RocksDB.

---

## The Goal

Build a production-grade Write-Ahead Log (WAL) system in Go. Every write must be durably recorded on disk before the memory state is updated. If the system crashes, your recovery engine must replay the log, verify data integrity via CRC32 checksums, and resume operations without data loss.

**Success Standard:**
- Start the server, perform 1,000 writes, `kill -9` the process manually. Restart the server.
- The recovery engine should log "Validated 1,000 entries, replaying...", and a `GET` request for the last key should return the correct value.
- **Recovery performance:** Replay a 1GB WAL file in under 5 seconds.
- **Data Integrity:** Catch partial writes at the byte boundary using CRC32.

---

## Where You Are Starting

- You have a basic in-memory KV store (`map[string]string`) with an HTTP API.
- Data is lost every time the server restarts.
- No persistence, no recovery, no durability.
- You have the spec for the WAL entry format (op, key_len, val_len, key, value, timestamp, checksum).

---

## What You Ship This Sprint

| Ticket | What | Priority | Days |
|--------|------|----------|------|
| WAL-101 | `WALWriter` Implementation — Entry serialization, disk appends, and `fsync`/`fdatasync` logic | P0 | 1-2 |
| WAL-102 | `WALReader` & Recovery Engine — Log replaying, CRC32 validation, and crash boundary detection | P0 | 3-4 |
| WAL-103 | Store Integration — Wiring the WAL into the HTTP API (`PUT`/`DELETE` flow) | P0 | 5 |
| WAL-104 | Log Compaction — Atomic snapshots, LSN tracking, and old log cleanup | P0 | 6-7 |
| WAL-108 | **Tenant Isolation** — Multi-segment WALs and 100MB per-tenant quotas | P1 | 8 |
| WAL-105 | Crash/Simulation Suite — Automated `SIGKILL` tests and data integrity verification | P1 | 9 |
| WAL-106 | Benchmarking & ADR — Performance analysis (`fsync` vs `fdatasync` vs none) and decision doc | P1 | 10 |

---

### WAL-101: `WALWriter` Implementation
**Priority: P0 | Days 1-2**

Implement the core logic for appending entries to the disk. Every write must follow the exact binary format specified in the project doc.

**Done when:**
- `Append(entry Entry) error` successfully writes the binary representation to `wal.log`.
- Supported sync modes: `fsync` (metadata + data) and `fdatasync` (data only).
- The entry includes a 4-byte CRC32 checksum over the operation, key, and value.
- Unit tests verify that bytes on disk match the expected binary layout.

---

### WAL-102: `WALReader` & Recovery Engine
**Priority: P0 | Days 3-4**

Build the "Time Machine" for your database. On startup, the system reads the WAL from beginning to end to rebuild the state.

**Done when:**
- `Replay()` reads a WAL file, validates the checksum for every entry, and applies valid entries to the map.
- The engine stops exactly at the first corrupted entry (mirroring a partial write during a crash).
- The store returns to the exact state it was in before the process was terminated.

---

### WAL-103: Store Integration
**Priority: P0 | Day 5**

Wire the WAL into the existing HTTP API. This ensures that no write is "successful" unless it's safe on disk.

**Done when:**
- `PUT /key` appends to WAL **before** updating the map.
- `DELETE /key` appends a "DEL" operation to the WAL.
- The API returns 200 only after the WAL write succeeds.
- The server initializes by calling `Recovery.Replay()` before listening for requests.

---

### WAL-104: Log Compaction
**Priority: P0 | Days 6-7**

Prevent the WAL from growing to infinity. Implement the Snapshot + WAL hybrid pattern.

**Done when:**
- `Compact()` serializes the current map to a `snapshot.bin` file.
- Uses an "Atomic Rename" strategy: write to `snapshot.tmp`, then `os.Rename` to `snapshot.bin`.
- After a successful snapshot, delete all WAL entries written before the snapshot LSN.

---

### WAL-108: Tenant Isolation (Storage Level)
**Priority: P1 | Day 8**

Implement isolation so one noisy tenant doesn't crash the whole system. Required by the "Backend 2026" roadmap.

**Done when:**
- Each tenant's data is stored in a separate WAL segment file (e.g., `tenant_1.log`).
- Storage quotas are enforced: Reject writes if a tenant's segment exceeds 100MB.
- `PUT /key` accepts a tenant ID to route to the correct log.

---

### WAL-105: Crash Simulation Suite
**Priority: P1 | Day 9**

Prove your system is "Crash-Safe."

**Done when:**
- A script spawns the server, sends randomized writes, and sends `SIGKILL` at random intervals.
- After restart, a verification script confirms that no data was lost or corrupted.
- `go test -race ./...` passes to ensure no concurrency bugs.

---

### WAL-106: Benchmarking & ADR
**Priority: P1 | Day 10**

Document the "Cost of Durability."

**Done when:**
- Benchmark results are recorded for: `fsync` vs `fdatasync` vs `No-sync`.
- Analysis of "Write Amplification" during log compaction.
- ADR (Architecture Decision Record) written explaining why `fdatasync` might be preferred in certain workloads.

---

## Environment & Setup

**Tech Stack:** Go (Standard Library), `hash/crc32`, `encoding/binary`.

**Local Setup:**
```bash
go mod init github.com/samarth/wal-kv
```

**Workflow:**
- Work in small commits.
- Verify every feature with a test case before moving to the next ticket.
- Use `hexdump -C wal.log` to inspect your binary format during development.

---

## Why This Matters — For Your Portfolio

By completing this independently, you are demonstrating:
1. **Low-Level Systems Knowledge:** You understand binary serialization, endianness, and disk I/O at a level most web developers never touch.
2. **Data Integrity Expertise:** You know how to use checksums and atomic file operations to prevent data corruption.
3. **Database Architecture Mastery:** You've built the core durability mechanism used by world-class databases like PostgreSQL, SQLite, and RocksDB.
4. **Multi-Tenancy at Storage Layer:** You understand how to isolate workloads at the disk level, a critical requirement for scaling SaaS platforms.

---

## Demo Script

1. **The Setup:** Show an empty data directory.
2. **The Writes:** Run a script sending 500 `PUT` requests across 2 tenants.
3. **The Crash:** `kill -9` the server.
4. **The Proof:** Show the binary bytes in `tenant_1.log` and `tenant_2.log`.
5. **The Recovery:** Start the server and show the logs replaying entries.
6. **The Result:** Verify `GET` returns correct values.
7. **The Quota:** Show a write being rejected when a tenant's log reaches 100MB.
