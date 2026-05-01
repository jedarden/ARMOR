# DuckDB httpfs Verification - ARMOR v0.1.13

## Date: 2026-05-01

### Task Verification
End-to-end verification that DuckDB can query Parquet files through ARMOR via httpfs after the URL decoding fix.

### Fix Details (v0.1.13)
**Commit:** `5638212` - fix(server): URL decode object keys to handle DuckDB httpfs encoding

**Problem:** DuckDB httpfs URL-encodes special characters in object keys (e.g., `=` as `%3D` for Hive partitioning). ARMOR must decode these before lookup in R2.

**Solution:** Added URL decoding in `internal/server/handlers/handlers.go` for GET requests with encoded keys.

### Verification Results

#### Environment
- **Cluster:** ord-devimprint (apexalgo-ord-devimprint)
- **Namespace:** devimprint
- **ARMOR Version:** v0.1.13
- **Image:** ronaldraygun/armor:0.1.13
- **Test Pod:** aggregator-6949b669d5

#### Tests Performed

| Test | Result | Details |
|------|--------|---------|
| ARMOR health check | ✅ | `/healthz` returns 200 OK |
| Simple glob `*.parquet` | ✅ | No InvalidInputException |
| Recursive glob `**/*.parquet` | ✅ | Found 10+ files, no errors |
| URL decoding (`%3D` for `=`) | ✅ | No HTTP 400 errors |
| ARMOR logs | ✅ | No error messages |

#### Sample Output
```
Testing recursive glob: s3://devimprint/commits/**/*.parquet
Recursive glob found 10 files (showing first 10):
  s3://devimprint/commits/year=1972/month=07/day=18/clone-worker-77cdf844d9-765km-1777040614.parquet
  s3://devimprint/commits/year=1973/month=11/day=11/clone-worker-6b94b786b8-sdqdc-1777361026.parquet
  ...
SUCCESS: No InvalidInputException or date parse errors!
```

### Acceptance Criteria

| Criteria | Status | Evidence |
|----------|--------|----------|
| DuckDB httpfs glob expansion works | ✅ | Recursive glob found files |
| No InvalidInputException/date parse errors | ✅ | Clean execution |
| LastModified timestamps reasonable | ✅ | ISO 8601 format in v0.1.11+ |
| URL encoding handled (`%3D` for `=`) | ✅ | v0.1.13 fix verified |
| ARMOR logs clean | ✅ | No errors |

### Related Fixes
1. **v0.1.11** (commit `ef77061`): ISO 8601 timestamp format for XML LastModified
2. **v0.1.13** (commit `5638212`): URL decode object keys for DuckDB httpfs encoding

Both fixes are now deployed and verified working on ord-devimprint.
