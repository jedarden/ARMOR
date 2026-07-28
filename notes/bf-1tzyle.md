# Task bf-1tzyle: Real Artifact-Class Assertions

## Status: Already Completed

This task was already implemented in commit `db412cbd feat(restoreverifier): real artifact-class assertions (SQLite/Parquet/tar.gz)`.

## Implementation Summary

All three artifact-class assertions in `internal/restoreverifier/verifier.go` have full implementations:

### SQLiteAssertion (lines 139-246)
- Validates "SQLite format 3" magic header
- Writes plaintext to temp file
- Opens database with modernc.org/sqlite driver (read-only, immutable mode)
- Runs `PRAGMA integrity_check` and reports any corruption
- Optional row-count probe via `x-amz-meta-armor-sqlite-table` metadata
- SQL injection protection via double-quote rejection

### ParquetAssertion (lines 248-299)
- Validates leading and trailing "PAR1" magic bytes
- Parses footer metadata using parquet-go
- Extracts row count and row group count
- Validates row count against `x-amz-meta-armor-parquet-rows` metadata (when present)
- Rejects files with zero row groups (empty/truncated artifacts)

### TarGzAssertion (lines 301-363)
- Parses gzip wrapper
- Walks every tar entry (detects truncated/corrupt streams)
- Samples every 8th entry (configurable via `tarGzSampleEvery`)
- Fully extracts sampled entries and verifies size matches header
- Enforces 100,000 entry limit (prevents decompression bombs)
- Rejects empty archives

## Test Coverage

### Unit Tests (verifier_test.go)
- `TestSQLiteAssertion`: Valid/corrupt detection, empty rejection, bad magic
- `TestSQLiteAssertionRowCountProbe`: Table presence, safety checks
- `TestParquetAssertion`: Valid/corrupt, row count matching/mismatch
- `TestTarGzAssertion`: Valid/corrupt, bad gzip, empty archive

### Integration Tests
- `TestVerifyObject_DualPathDetectsCorruption`: Proves both restore paths agree and assertions catch corruption
- `TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath`: Proves direct-only DR drill works
- `TestVerifyObject_DRDrill_MultipartDigestEnforced`: Validates bf-1v2ehf multipart digest enforcement

### Fixtures (testdata/)
- `valid.sqlite` / `corrupt.sqlite`: SQLite fixtures (magic intact, mid-file corruption)
- `valid.parquet` / `corrupt.parquet`: Parquet fixtures (footer clobbered)
- `valid.tar.gz` / `corrupt.tar.gz`: tar.gz fixtures (mid-payload corruption)

## Verification

```bash
$ go test ./internal/restoreverifier/... -v
PASS
ok      github.com/jedarden/armor/internal/restoreverifier    0.260s
```

All tests pass. Corrupted fixtures are detected on both code paths (dual-path ModeDual and direct-only ModeDRDrill).
