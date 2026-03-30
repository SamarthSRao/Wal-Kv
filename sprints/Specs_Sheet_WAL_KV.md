# Technical Specification: WAL-Backed KV Store (Wal-Kv)

**Status:** Draft
**Project Reference:** Biweekly Project 1 (Weeks 1-2)
**Objective:** Implement a crash-safe, durable Key-Value store using a Write-Ahead Log (WAL) and Periodic Snapshotting.

---

## 1. System Overview
The `Wal-Kv` store is a durable version of a standard in-memory hash map. It ensures that every write operation is recorded on physical storage *before* it is acknowledged to the client. In the event of a power failure or process crash, the system can reconstruct its state by replaying the WAL.

---

## 2. Technical Requirements

### 2.1 Durability & Atomicity
- **Write-Ahead Principle:** No update to the in-memory map occurs until the corresponding log entry is successfully flushed to disk.
- **Fsync Policy:** The system must support an `fsync` configuration (Every write / Periodic / None) to explore the durability vs. performance tradeoff.
- **Partial Write Detection:** use CRC32 checksums to detect and ignore partial or corrupted writes at the end of the log file during recovery.

### 2.2 Storage Components
- **WAL (Write-Ahead Log):** An append-only binary file storing all state-changing operations.
- **Checkpoint/Snapshot:** A binary representation of the full in-memory state captured at a specific Log Sequence Number (LSN).
- **Compaction Engine:** Logic to truncate the WAL after a successful snapshot to save disk space.

---

## 3. Data Structures

### 3.1 Binary WAL Entry Format
Each entry in the WAL must follow this exact byte-level layout for consistency:

| Field | Size | Data Type | Description |
|---|---|---|---|
| **Op** | 1 Byte | UInt8 | `0x01` for SET, `0x02` for DELETE |
| **Key Length** | 4 Bytes | UInt32 (BE) | Length of the key string |
| **Value Length** | 4 Bytes | UInt32 (BE) | Length of the value string |
| **Key** | N Bytes | String | Raw key data |
| **Value** | M Bytes | String | Raw value data (empty for DELETE) |
| **Timestamp** | 8 Bytes | Int64 (BE) | Unix nanoseconds |
| **Checksum** | 4 Bytes | UInt32 (BE) | CRC32 of all preceding bytes in this entry |

### 3.2 In-Memory State
- **Map:** `map[string]string` for core storage.
- **LSN (Log Sequence Number):** An atomic counter representing the current offset/index in the log.

---

## 4. Operational Workflows

### 4.1 Write Path (PUT/DELETE)
1. Receive request.
2. Serialize binary entry (including checksum).
3. Append entry to WAL file.
4. Call `fsync()` (if configured).
5. Update in-memory map.
6. Return success to client.

### 4.2 Recovery Path (Startup)
1. Load `snapshot.bin` (if exists) into memory.
2. Identify the last LSN from the snapshot.
3. Open `wal.log` and seek to the snapshot LSN.
4. Iterate through entries:
    - Read header and body.
    - Compute CRC32.
    - If checksum fails: **STRICT STOP** (this is the crash boundary).
    - If checksum passes: Apply operation to map.
5. Store is now ready for traffic.

### 4.3 Compaction Path
1. Lock map for reading (or use Copy-On-Write).
2. Serialize current map to `snapshot.tmp`.
3. Atomic `os.Rename("snapshot.tmp", "snapshot.bin")`.
4. Delete old WAL segments or truncate the WAL up to the serialized LSN.

---

## 5. Performance Targets & Benchmarks
- **Ingestion:** 100K sequential writes with `fsync` on.
- **Recovery Time:** < 5 seconds for a 1GB WAL file.
- **Compaction:** Must not block the write path for more than 500ms (via atomic rename).
- **Documentation:** `BENCHMARKS.md` must compare `fsync` vs `fdatasync` vs `No-sync`.

---

## 6. Advanced Features (Stretch Goals)
- **Tenant Isolation:** Each tenant has a separate WAL segment; enforce 100MB quotas per segment.
- **Batched Fsync:** Group multiple writes into a single disk flush to improve throughput without losing much durability.
- **Bloom Filters:** Implement a Bloom filter on the recovery path to speed up existence checks (conceptually useful for LSM-tree transition later).

---

## 7. ADR (Architecture Decision Records) Required
1. **Decision:** Why WAL + Snapshot vs. Snapshot-only?
2. **Decision:** Choice of binary serialization over JSON/Protobuf for the log.
3. **Decision:** Use of `fsync` vs `O_DIRECT`.
