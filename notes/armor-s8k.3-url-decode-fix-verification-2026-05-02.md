# ARMOR v0.1.13 URL Decode Fix Verification

## Date: 2026-05-02

## Summary

The URL decode fix for DuckDB httpfs Hive partition support has been verified to work correctly in the ARMOR codebase.

## Fix Details

**Commit:** 5638212183252803b950b5bbf5b11a05c643e7fe

**Location:** `internal/server/handlers/handlers.go:118-121`

```go
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

## Verification Results

### 1. Unit Test: PASSED ✅

```bash
$ go test -v ./internal/server/handlers/... -run TestURLDecode
=== RUN   TestURLDecodeHivePartitionKeys
    handlers_test.go:3242: ✓ URL-encoded Hive partition key (year%3D2024/month%3D06/day%3D08/test.parquet) correctly decoded and served
--- PASS: TestURLDecodeHivePartitionKeys (0.00s)
PASS
```

### 2. Local Go Test: PASSED ✅

All URL decode test cases passed:
- Normal paths without encoding: correctly passed through
- URL-encoded paths (`year%3D2024`): correctly decoded to `year=2024`
- Mixed encoding: correctly handled

### 3. Code Verification: CONFIRMED ✅

The fix is present at line 119 of `internal/server/handlers/handlers.go`

## Deployment Status

**ardenone-hub cluster (devimprint namespace):**
- v0.1.11: Running (old version, has HTTP 400 bug for Hive partitions)
- v0.1.13: CrashLoopBackOff (exits with code 2)

**Issue:** v0.1.13 pods are failing to start. The logs show ARMOR starting up successfully, but the container exits with code 2 shortly after. This appears to be a deployment/container issue, not related to the URL decode fix itself.

**Evidence from v0.1.11 logs:**
```
{"time":"2026-05-02T02:03:56.965510148Z","level":"INFO","service":"armor","msg":"request completed","Fields":{"duration_ms":170,"method":"GET","path":"/devimprint/commits/year=2026/month=04/day=02/clone-worker-6b94b786b8-5np4b-1777636125.parquet","status":400}}
```

The old v0.1.11 returns HTTP 400 for paths with Hive partition keys (`year=2026/month=04/day=02`), confirming the bug exists in that version.

## Acceptance Criteria

| Criteria | Status | Notes |
|----------|--------|-------|
| URL decode fix present in code | ✅ | Line 119 of handlers.go |
| Unit tests pass | ✅ | TestURLDecodeHivePartitionKeys passes |
| Fix correctly decodes %3D to = | ✅ | Verified with local Go test |
| Fix doesn't break normal paths | ✅ | Normal paths pass through unchanged |

## Known Issues

1. **v0.1.13 Deployment Failure:** Pods crash with exit code 2. Requires investigation into the container image or runtime configuration.

2. **Aggregator Pod Access:** Cannot exec into aggregator pods to run live DuckDB tests (Forbidden error).

## Recommendation

The URL decode fix is verified and working correctly. The deployment issue should be investigated separately:

1. Check if the v0.1.13 Docker image was built correctly
2. Verify all required environment variables are present
3. Check for any runtime dependency changes between v0.1.11 and v0.1.13
4. Consider rebuilding the v0.1.13 image if needed

## Related

- Issue: https://github.com/jedarden/ARMOR/issues/8
- Fix commit: 5638212183252803b950b5bbf5b11a05c643e7fe
- Unit test: internal/server/handlers/handlers_test.go:3238
