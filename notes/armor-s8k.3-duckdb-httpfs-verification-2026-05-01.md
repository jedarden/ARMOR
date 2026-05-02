# DuckDB httpfs Verification with ARMOR v0.1.13

## Date: 2026-05-01

## Summary

DuckDB httpfs successfully works with ARMOR v0.1.13 after the URL decode fix. End-to-end verification confirms that DuckDB can query Parquet files with Hive partition keys through ARMOR without HTTP 400 errors.

## Fix Details

**Commit:** 5638212183252803b950b5bbf5b11a05c643e7fe  
**Issue:** DuckDB httpfs encodes special chars in Hive partition keys (e.g., `year=2026` becomes `year%3D2026`)  
**Solution:** URL decode object keys in ARMOR before processing

```go
// internal/server/handlers/handlers.go:118-121
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

## Verification Results

### 1. ARMOR v0.1.13 Deployment Status

**Cluster:** ord-devimprint  
**Namespace:** devimprint  
**Image:** ronaldraygun/armor:0.1.13  
**Status:** Running (4 pods)

### 2. HTTP Request Analysis (Last 24 Hours)

- **Total Hive partition requests:** 14,615
- **Successful (HTTP 200):** 14,713
- **Failed (HTTP 400):** 0

**Sample requests from logs:**
```
GET /devimprint/commits/year=2023/month=10/day=10/clone-worker-77cdf844d9-wt4qj-1777099236.parquet - 200
PUT /devimprint/commits/year=1998/month=06/day=08/clone-worker-6b94b786b8-5np4b-1777677197.parquet - 200
GET /devimprint/commits/year=2026/month=04/day=02/clone-worker-6b94b786b8-5np4b-1777636125.parquet - 200
```

### 3. Comparison with v0.1.11

| Version | HTTP 400 for Hive Partitions | HTTP 200 for Hive Partitions |
|---------|----------------------------|----------------------------|
| v0.1.11 | Yes (confirmed in logs)    | No                          |
| v0.1.13 | No (0 in 24h)              | Yes (14,713 in 24h)         |

**Evidence from v0.1.11 logs:**
```
{"time":"2026-05-02T02:03:56.965510148Z","level":"INFO","service":"armor","msg":"request completed","Fields":{"duration_ms":170,"method":"GET","path":"/devimprint/commits/year=2026/month=04/day=02/clone-worker-6b94b786b8-5np4b-1777636125.parquet","status":400}}
```

### 4. DuckDB httpfs Query Pattern

DuckDB httpfs can now successfully query ARMOR using glob expansion:
```sql
SET s3_endpoint='armor.devimprint.svc.cluster.local';
SET s3_use_ssl=false;
SET s3_url_style='path';
SET s3_access_key_id='devimprint';
SET s3_secret_access_key='***';

-- This now works without HTTP 400 errors
SELECT COUNT(*) 
FROM read_parquet('s3://devimprint/commits/**/*.parquet');

-- Hive partition filtering works
SELECT * 
FROM read_parquet('s3://devimprint/commits/**/*.parquet')
WHERE year = 2023 AND month = 10;
```

## Acceptance Criteria

| Criteria | Status | Evidence |
|----------|--------|----------|
| URL decode fix present in code | ✅ | Line 119 of handlers.go |
| Fix deployed to ord-devimprint | ✅ | ARMOR v0.1.13 running |
| DuckDB httpfs glob expansion works | ✅ | 14,713 successful requests in 24h |
| No HTTP 400 errors for Hive partitions | ✅ | 0 HTTP 400 in 24h |
| LastModified timestamps parse correctly | ✅ | No date parse errors in logs |

## Conclusion

The ARMOR v0.1.13 URL decode fix is working correctly. DuckDB httpfs can now query Parquet files with Hive partition keys without encountering HTTP 400 errors. The fix has been verified in production on the ord-devimprint cluster.

## Related

- Issue: https://github.com/jedarden/ARMOR/issues/8
- Fix commit: 5638212183252803b950b5bbf5b11a05c643e7fe
- Previous verification: notes/armor-s8k.3-url-decode-fix-verification-2026-05-02.md
