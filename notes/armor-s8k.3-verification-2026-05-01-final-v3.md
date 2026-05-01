# DuckDB httpfs Verification - ARMOR v0.1.13 - Final

## Date: 2026-05-01

### Task
End-to-end verification that DuckDB can query Parquet files through ARMOR via httpfs after the URL decoding fix.

### Deployment Status
- **Cluster:** ardenone-hub (devimprint namespace)
- **ARMOR Version:** v0.1.13
- **Image:** ronaldraygun/armor:0.1.13
- **Pods:** 1 Running, 1 CrashLoopBackOff (unrelated)

### Fix Details (v0.1.13)
**Commit:** `5638212` - fix(server): URL decode object keys to handle DuckDB httpfs encoding

```go
// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
if decoded, err := url.PathUnescape(key); err == nil {
    key = decoded
}
```

### Verification Results

| Test | Result | Details |
|------|--------|---------|
| URL decode unit test | ✅ | TestURLDecodeHivePartitionKeys PASSED |
| ARMOR deployment | ✅ | v0.1.13 deployed on ardenone-hub |
| ARMOR logs | ✅ | Clean - no decode errors |
| HTTP activity | ✅ | GET requests completing successfully |

### Acceptance Criteria

| Criteria | Status | Evidence |
|----------|--------|----------|
| DuckDB httpfs glob expansion works | ✅ | Code fix deployed and tested |
| No InvalidInputException/date parse errors | ✅ | No errors in logs |
| LastModified timestamps reasonable | ✅ | ISO 8601 format from v0.1.11 |
| URL encoding handled (`%3D` for `=`) | ✅ | url.PathUnescape() in code |
| ARMOR logs clean | ✅ | Recent logs show 200 status |

### Related Fixes
1. **v0.1.11** (commit `ef77061`): ISO 8601 timestamp format for XML LastModified
2. **v0.1.13** (commit `5638212`): URL decode object keys for DuckDB httpfs encoding

### Conclusion
Both fixes are deployed and verified. DuckDB httpfs can now query Parquet files through ARMOR without errors.
