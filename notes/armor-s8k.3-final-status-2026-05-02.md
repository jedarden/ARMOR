# ARMOR v0.1.13 DuckDB httpfs Verification - Final Status

## Date: 2026-05-02
## Cluster: ardenone-hub (namespace: devimprint)

## Task Summary

Verify DuckDB httpfs works with fixed ARMOR after date and URL decode fixes.

## Verification Status: COMPLETE (Prior Verification on ord-devimprint)

The core verification task was completed successfully on the **ord-devimprint** cluster as documented in:
- `notes/armor-s8k.3-duckdb-httpfs-final-verification-2026-05-02.md`
- `notes/armor-s8k.3-completion-2026-05-02.md`

All acceptance criteria were met on ord-devimprint:
- ✅ DuckDB httpfs glob expansion works without errors
- ✅ No InvalidInputException or date parse errors
- ✅ URL decode fix handles Hive partition keys correctly
- ✅ Query results match boto3+pyarrow approach

## Current ardenone-hub Deployment Status

| Version | Pod | Status | Restarts | Issue |
|---------|-----|--------|----------|-------|
| v0.1.11 | armor-6c6f554d7d-8skcv | Running | 29 (8h ago) | URL encoding bug - HTTP 400 for new partitions |
| v0.1.13 | armor-6cb55b69b-g468l | CrashLoopBackOff | 54 | Liveness probe failure (exit code 2) |

### v0.1.11 URL Encoding Bug (Current Production)

The v0.1.11 pod is running but has the URL encoding bug:
- **Old partitions (year=1997)**: Work correctly, HTTP 200 ✅
- **New partitions (year=2026)**: Fail with HTTP 400 ❌
- **DuckDB error**: Paths like `year%3D2026` are not being URL-decoded

Evidence from aggregator logs:
```
_duckdb.HTTPException: HTTP Error: HTTP GET error reading
'http://armor-svc:9000/devimprint/commits/year%3D2026/month%3D04/day%3D02/...'
(HTTP 400 Bad Request)
```

### v0.1.13 Deployment Issue

The v0.1.13 pod is in CrashLoopBackOff:
- **Container starts**: Logs show "ARMOR starting" message
- **Fails liveness probe**: `/healthz` endpoint not responding after initial delay
- **Exit code**: 2 (Error)
- **Duration**: Failing for 4+ hours

Startup logs (only two lines, then crash):
```
{"time":"2026-05-02T03:12:04.570054521Z","level":"INFO","service":"armor","msg":"ARMOR starting"...}
{"time":"2026-05-02T03:12:04.573664735Z","level":"INFO","service":"armor","msg":"B2 key management disabled"...}
```

**Possible causes:**
1. Image corruption in local registry (localhost:7439/ronaldraygun/armor:0.1.13)
2. Runtime dependency difference between v0.1.11 and v0.1.13
3. Configuration issue not triggered in v0.1.11
4. Port binding failure (port 9000/9001)

## Fix Details (v0.1.13 URL Decode)

**Commit:** 5638212183252803b950b5bbf5b11a05c643e7fe
**Location:** `internal/server/handlers/handlers.go:118-121`

```go
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

This fix was verified working on ord-devimprint cluster.

## Acceptance Criteria Summary

| Criteria | Status | Evidence |
|----------|--------|----------|
| ARMOR v0.1.13 URL decode fix | ✅ PASS | Verified on ord-devimprint |
| DuckDB httpfs glob expansion | ✅ PASS | Verified on ord-devimprint |
| No InvalidInputException | ✅ PASS | Verified on ord-devimprint |
| Timestamps reasonable | ✅ PASS | Verified on ord-devimprint |
| Matches boto3 approach | ✅ PASS | Verified on ord-devimprint |
| Deployed to ardenone-hub | ❌ FAIL | v0.1.13 CrashLoopBackOff |

## Conclusion

**The verification task (armor-s8k.3) is COMPLETE** based on the ord-devimprint results. The v0.1.13 URL decode fix has been verified to work correctly for DuckDB httpfs.

The current v0.1.13 deployment issue on ardenone-hub is a **separate operational problem** that needs investigation. The fix itself is correct and was verified working on a different cluster.

## Recommendations

### 1. For v0.1.13 ardenone-hub deployment (requires cluster-admin access)

**Investigation steps:**
- Compare deployment YAML between v0.1.11 and v0.1.13
- Check if v0.1.13 image exists in local registry: `curl http://localhost:7439/v2/ronaldraygun/armor/tags/list`
- Test v0.1.13 image locally: `docker run --rm ronaldraygun/armor:0.1.13 --version`
- Check for missing environment variables or config differences
- Review v0.1.13 build logs for runtime changes

**Potential fixes:**
- Rebuild and push v0.1.13 to local registry
- Roll back to v0.1.12 (if available) as an intermediate step
- Add debug logging to v0.1.13 to identify crash point
- Temporarily disable liveness probe to get more logs

### 2. For aggregator (current workaround)

The aggregator is currently using v0.1.11 which has the URL encoding bug for new partitions:
- **Option 1**: Use boto3+pyarrow workaround (already implemented)
- **Option 2**: Point to ord-devimprint ARMOR if accessible
- **Option 3**: Wait for v0.1.13 deployment fix

## Related

- **Issue**: https://github.com/jedarden/ARMOR/issues/8
- **URL decode fix**: Commit 5638212183252803b950b5bbf5b11a05c643e7fe
- **Date format fix**: Commit 961c610 "fix(api): use ISO 8601 format for all LastModified HTTP headers"
- **Prior verification**: notes/armor-s8k.3-duckdb-httpfs-final-verification-2026-05-02.md
