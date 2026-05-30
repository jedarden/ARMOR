# Bead bf-6c8o: Locate Manifest Test Files

## Summary

Located all manifest-related test files in `/home/coding/ARMOR/internal/manifest/`.

## Test Files Found

### 1. `manifest_test.go` (365 lines)
Core Index operations tests:
- Basic CRUD: Put, Get, Delete, Len
- Snapshot marshaling/unmarshaling (roundtrip)
- Delta operations (put, delete) with marshaling/unmarshaling
- Delta applied on top of snapshot
- Merge functionality with last-write-wins semantics
- Sequence counter operations (IncSeq, SetSeq, Seq)
- Key generation helpers (DeltaKey, SnapshotKey, WriterPrefix, DeltaSeqFromKey)

### 2. `loader_test.go` (357 lines)
Load function tests (restoring index from storage):
- Empty store, snapshot-only, deltas-only, snapshot+deltas scenarios
- Multi-writer support (merging across writer IDs)
- Sequence number handling for current writer vs other writers
- Pagination support
- Error propagation (fetch errors, list errors)

### 3. `writer_test.go` (298 lines)
Writer tests (persisting index changes as delta files):
- Put and Delete operations
- Batching behavior with configurable buffer size
- Sequence incrementing per flush
- Padded filename generation (10-digit zero-padding)
- Context cancellation handling
- Custom prefix support
- Writer ID inclusion in keys

### 4. `compaction_test.go` (458 lines)
Compaction tests (combines deltas into snapshot, deletes old deltas):
- Snapshot upload and delta deletion verification
- No-op when seq is zero
- Idempotency (running compaction twice on same state)
- Error propagation (upload errors, delete errors)
- Threshold-based triggering with NotifyDelta()
- Delta count reset after compaction
- Concurrent delta survival (seq > compaction point)

### 5. `roundtrip_test.go` (309 lines)
End-to-end integration tests:
- Writer → persist → Load → restore cycle
- Snapshot + deltas scenario (compaction output)
- Sequence order verification (deltas applied in ascending seq regardless of listing order)
- Delete-then-put pattern testing

## Test Organization

- All files use `package manifest_test` except `compaction_test.go` (uses `package manifest`)
- Shared helper: `sampleEntry()` creates test Entry structs
- Mock implementations: `mockStore`, `captureUploader`, `compactionStore`
- Tests use table-driven and scenario-based approaches
