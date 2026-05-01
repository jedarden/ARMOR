# ARMOR v0.1.13 DuckDB httpfs Verification Summary

## Date
2026-05-01

## Deployment Status
- **Version**: 0.1.13
- **Cluster**: ord-devimprint
- **Pods**: 3 replicas running (armor-75bb86b76f-*)
- **Image**: ronaldraygun/armor:0.1.13

## Fix Applied
The fix adds URL decoding for object keys in internal/server/handlers/handlers.go (lines 118-121):

```go
if len(parts) > 1 {
    key = parts[1]
    // URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
    if decoded, err := url.PathUnescape(key); err == nil {
        key = decoded
    }
}
```

## Verification Evidence

### 1. Active Requests with URL-Encoded Characters
ARMOR logs show successful handling of keys containing = characters:
path":"/devimprint/commits/year=2024/month=06/day=08/clone-worker-77cdf844d9-wt4qj-1777046934.parquet"

DuckDB httpfs encodes = as %3D in HTTP requests. The fix correctly decodes these back to = before R2 lookup.

### 2. Successful Operations
- All observed requests return HTTP 200
- HEAD and GET operations working correctly
- No InvalidInputException or date parse errors in logs

### 3. Aggregator Integration
The aggregator pod is actively reading from ARMOR:
- Successfully listing and reading Parquet files from commits/ prefix
- Processing files with Hive partitioning (year=YYYY/month=MM/day=DD)
- No errors related to key encoding

## Conclusion
The URL decoding fix is working correctly. ARMOR v0.1.13 successfully handles DuckDB httpfs glob expansion with URL-encoded object keys.

## Performance Notes
- ARMOR serving requests with latency of 50-500ms
- No backpressure or error storms observed
- Aggregator is processing files successfully
