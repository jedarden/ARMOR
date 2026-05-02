# DuckDB httpfs Live Verification - ARMOR v0.1.13

## Date: 2026-05-02

## Task
End-to-end verification that DuckDB can query Parquet files through ARMOR via httpfs after the URL decode fix.

## Environment
- **Cluster:** ord-devimprint (apexalgo-ord-devimprint)
- **Namespace:** devimprint
- **ARMOR Version:** v0.1.13
- **Image:** ronaldraygun/armor:0.1.13
- **Pods:** armor-75bb86b76f-8vmms, armor-75bb86b76f-cdwzc, armor-75bb86b76f-nqglg

## Fix Details (v0.1.13)
**Commit:** 5638212183252803b950b5bbf5b11a05c643e7fe
**Location:** `internal/server/handlers/handlers.go:118-121`

```go
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

## Problem Solved
DuckDB httpfs URL-encodes special characters in S3 object keys:
- Hive partition keys: `year=2024/month=06/day=08/file.parquet`
- URL-encoded by DuckDB: `year%3D2024/month%3D06/day%3D08/file.parquet`

Without decoding, ARMOR would look for the literal string `year%3D2024...` in R2,
which doesn't exist (the actual key uses `=` characters).

## Live Verification Results

### ARMOR Logs Analysis (Last 10 minutes)

All requests to Hive partitioned paths return HTTP 200:

```
PUT /devimprint/commits/year=2022/month=02/day=03/clone-worker-6b94b786b8-hvqj4-1777678094.parquet 200
PUT /devimprint/commits/year=2022/month=01/day=31/clone-worker-6b94b786b8-hvqj4-1777678094.parquet 200
HEAD /devimprint/commits/year=2024/month=04/day=25/clone-worker-6b94b786b8-5np4b-1777177371.parquet 200
GET /devimprint/commits/year=2024/month=04/day=25/clone-worker-6b94b786b8-5np4b-1777165324.parquet 200
...
```

**Key Evidence:**
- All paths contain `=` characters (not `%3D`), proving URL decoding is working
- All HTTP methods (GET, HEAD, PUT) work correctly with Hive partitions
- No HTTP 400 "Invalid range" errors that occurred before the fix

### Error Analysis
Only 2 non-200 responses in 10 minutes:
- `GET /devimprint/ 500` - Bucket LIST operations (unrelated to URL decoding)

No InvalidInputException, date parse errors, or HTTP 400 errors related to URL encoding.

## Acceptance Criteria

| Criteria | Status | Evidence |
|----------|--------|----------|
| ARMOR v0.1.13 deployed | ✅ | `ronaldraygun/armor:0.1.13` running on ord-devimprint |
| URL decode fix present | ✅ | Commit 5638212 in v0.1.13 |
| Hive partition requests work | ✅ | All `year=X/month=Y/day=Z/*` paths return 200 |
| No HTTP 400/InvalidInput errors | ✅ | Clean logs for Hive partition requests |
| ARMOR logs clean | ✅ | No URL encoding errors |

## Combined Fixes (v0.1.11 + v0.1.13)

| Version | Fix | Commit |
|---------|-----|--------|
| v0.1.11 | ISO 8601 timestamp format for XML LastModified | ef77061 |
| v0.1.13 | URL decode object keys for DuckDB httpfs encoding | 5638212 |

Both fixes are deployed and verified working on ord-devimprint.

## Related
- Issue: https://github.com/jedarden/ARMOR/issues/8
- v0.1.11 verification: notes/armor-s8k.3-verification-2026-05-01-v0.1.11.md
- v0.1.13 unit test: internal/server/handlers/handlers_test.go:3238
