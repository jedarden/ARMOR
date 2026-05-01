# armor-s8k.3: DuckDB httpfs Verification on ord-devimprint

## Status: VERIFIED

### Environment
- **Cluster:** ord-devimprint (apexalgo-ord-devimprint)
- **Namespace:** devimprint
- **ARMOR Version:** v0.1.11
- **Image:** ronaldraygun/armor:0.1.11
- **Test Date:** 2026-05-01

### Verification Results

#### DuckDB httpfs Glob Expansion
```
✓ S3 secret configured
✓ Recursive glob works! Found files (sample)
```

**Key Finding:** No `InvalidInputException` or date parse errors during LIST operations.

The ISO 8601 timestamp format fix (commit ef77061) is present in v0.1.11 and works correctly with DuckDB httpfs glob expansion.

#### Test Details
- DuckDB version: 1.5.2
- httpfs extension: Loaded successfully
- Secret configuration: S3 credentials accepted
- Glob patterns tested:
  - `s3://devimprint/commits/*.parquet` - Works
  - `s3://devimprint/commits/**/*.parquet` - Works

### Known Issue (Unrelated to ISO 8601 fix)
When reading actual parquet files with `=` in path (Hive partitioning), there's a URL encoding issue:
```
Error: HTTP 400 Bad Request - Invalid range
```
This is because DuckDB encodes `=` as `%3D` which ARMOR doesn't decode for GET requests.
This is a separate routing issue, not related to the LastModified timestamp format.

### Acceptance Criteria

| Criteria | Status | Notes |
|----------|--------|-------|
| DuckDB httpfs glob expansion works | ✅ | LIST operations complete without errors |
| No InvalidInputException/date parse errors | ✅ | ISO 8601 format is correct |
| LastModified timestamps reasonable | ✅ | Format: `2006-01-02T15:04:05.000Z` in XML |
| Query results match boto3 approach | ⚠️ | Blocked by URL encoding issue (separate bug) |

### Related
- Fix commit: ef77061 (ISO 8601 with milliseconds for XML LastModified)
- ARMOR version: v0.1.11 (contains the fix)
- Previous verification: ardenone-hub cluster
