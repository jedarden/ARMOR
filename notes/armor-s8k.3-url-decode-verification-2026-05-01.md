# ARMOR v0.1.13 DuckDB httpfs URL Decode Verification

## Date
2026-05-01

## Summary
Verified that ARMOR v0.1.13 correctly handles URL-encoded object keys from DuckDB httpfs.

## Fix Details
**Commit:** 5638212183252803b950b5bbf5b11a05c643e7fe
**Location:** `internal/server/handlers/handlers.go:118-121`

```go
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

## Problem
DuckDB httpfs URL-encodes special characters in S3 object keys:
- Hive partition keys: `year=2024/month=06/day=08/file.parquet`
- URL-encoded by DuckDB: `year%3D2024/month%3D06/day%3D08/file.parquet`

Without decoding, ARMOR would look for the literal string `year%3D2024...` in R2,
which doesn't exist (the actual key uses `=` characters).

## Test Coverage
Added `TestURLDecodeHivePartitionKeys` in `handlers_test.go`:

```bash
$ go test -v -run TestURLDecodeHivePartitionKeys ./internal/server/handlers/
=== RUN   TestURLDecodeHivePartitionKeys
    handlers_test.go:3242: ✓ URL-encoded Hive partition key (year%3D2024/month%3D06/day%3D08/test.parquet) correctly decoded and served
--- PASS: TestURLDecodeHivePartitionKeys (0.00s)
```

All handler tests pass:
```bash
$ go test ./internal/server/handlers/
PASS
ok  	github.com/jedarden/armor/internal/server/handlers	0.498s
```

## Deployment Verification
Based on production logs from ord-devimprint cluster (see `armor-s8k.3-verification-summary.md`):
- ARMOR v0.1.13 successfully serving requests with URL-encoded keys
- Aggregator actively reading Parquet files with Hive partitioning
- No errors related to key encoding

## Acceptance Criteria

| Criteria | Status |
|----------|--------|
| Fix correctly implemented | ✅ |
| Unit test coverage added | ✅ |
| All existing tests pass | ✅ |
| Production deployment verified | ✅ (via existing documentation) |

## Technical Notes
- `url.PathUnescape()` handles `%3D` → `=` correctly
- Error is ignored on decode failure (falls back to original key)
- Fix applies to all operations (GET, HEAD, PUT, DELETE, etc.)
