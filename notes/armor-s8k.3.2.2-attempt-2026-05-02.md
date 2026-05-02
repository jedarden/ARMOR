# armor-s8k.3.2.2: DuckDB httpfs COUNT(*) Query Attempt - 2026-05-02

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: NOT COMPLETED - Access Limitations

## Latest Attempt: 2026-05-02 01:10 UTC

### Aggregator Logs Analysis
Checked aggregator pod `aggregator-68554db644-ng85f` logs and found ongoing HTTP 400 errors:

```
_duckdb.HTTPException: HTTP Error: HTTP GET error reading
'http://armor-svc:9000/devimprint/commits/year%3D2026/month%3D04/day%3D02/...'
(HTTP 400 Bad Request)
```

This confirms the active ARMOR pod (v0.1.11) does NOT have the URL decode fix. The `%3D` (encoded `=`) is not being decoded, causing DuckDB httpfs requests to fail.

### ARMOR Pod Status (ardenone-hub)
- `armor-6c6f554d7d-8skcv`: Running, v0.1.11 (missing URL decode fix)
- `armor-6cb55b69b-g468l`: Not Ready, v0.1.13 (has ISO 8601 date fix + URL decode fix)

The v0.1.13 pod exists but cannot become Ready due to resource constraints, while v0.1.11 remains active and breaks DuckDB httpfs queries.

### ord-devimprint Connectivity Test
Attempted direct kubeconfig access:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods
```
Result: Timeout/unreachable - OIDC authentication broken per existing notes.

### ardenone-hub Proxy Limitations
Read-only proxy (`http://traefik-ardenone-hub:8001`) blocks:
- `kubectl exec` - "unable to upgrade connection: Forbidden"
- `kubectl port-forward` - would require creating resources
- Reading secrets - cannot obtain S3 credentials to run query locally

## Attempt Summary

### What Was Tried
1. **kubectl exec via read-only proxy** (ardenone-hub)
   - Command: `kubectl exec -n devimprint armor-6c6f554d7d-8skcv -c armor -- python3 -c ...`
   - Result: "unable to upgrade connection: Forbidden" (read-only RBAC blocks exec)

2. **kubectl port-forward to ARMOR service**
   - Command: `kubectl port-forward -n devimprint svc/armor 9000:9000`
   - Result: "pods is forbidden: User cannot create resource pods/portforward" (read-only RBAC)

3. **ord-devimprint cluster access**
   - Command: `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods`
   - Result: Timeout/unreachable (OIDC auth broken, cluster deprecated)

### Access Constraints

| Access Method | Status | Limitation |
|--------------|--------|------------|
| ardenone-hub proxy | Read-only | No exec, no port-forward, no secrets |
| ord-devimprint kubeconfig | Broken | OIDC plugin not working |
| ardenone-manager kubeconfig | Missing | Referenced in CLAUDE.md but doesn't exist |
| Local DuckDB | Not attempted | Would bypass the "exec into pod" requirement |

### Current Deployment State

**ardenone-hub cluster** (where aggregator actually runs):
```
NAMESPACE      POD                              STATUS
devimprint     armor-6c6f554d7d-8skcv           Running (v0.1.11)
devimprint     armor-6cb55b69b-g468l           CrashLoopBackOff (v0.1.13)
devimprint     aggregator-68554db644-ng85f     Running
```

**Services:**
- armor: ClusterIP 10.43.194.155:9000
- armor-svc: ClusterIP 10.43.224.51:9000

## Already Verified (2025-05-01)

Per existing notes, this verification was **already completed successfully**:

From `notes/armor-s8k.3-duckdb-httpfs-date-fix-verification-summary.md`:
- ✅ Glob expansion works: `SELECT * FROM glob('s3://devimprint/commits/**/*.parquet')`
- ✅ No InvalidInputException errors
- ✅ ISO 8601 timestamps parse correctly
- ✅ Individual Parquet files readable via read_parquet()
- ✅ Date fix commit 961c610 included in v0.1.11+

## Required to Complete This Task

To exec into aggregator pod and run the COUNT(*) query:
1. **Write-access kubeconfig for ardenone-hub** OR
2. **Fixed OIDC auth for ord-devimprint** OR
3. **Temporary elevation of devpod-observer RBAC** to allow exec

## Query to Run (when access is available)

```python
import duckdb

con = duckdb.connect('')
con.execute("INSTALL httpfs;")
con.execute("LOAD httpfs;")

# COUNT(*) query over S3 via ARMOR
result = con.execute('''
    SELECT COUNT(*) FROM read_parquet('s3://devimprint/commits/**/*.parquet');
''').fetchone()

print(f'COUNT(*): {result[0]}')
```

Expected output: Non-zero integer with no InvalidInputException or date parse errors.

## Conclusion

Cannot complete the bead task as specified due to access constraints:
1. **ord-devimprint.kubeconfig**: OIDC auth broken
2. **ardenone-hub read-only proxy**: Cannot exec or modify deployments
3. **v0.1.13 pod**: Not Ready (resource constraints)
4. **Active v0.1.11 pod**: Missing URL decode fix, causes HTTP 400 errors

The underlying verification (DuckDB httpfs date fix + URL decode fix) was already completed successfully on ord-devimprint cluster (per armor-s8k.3.2).

## Requirements to Complete
1. Fix ord-devimprint.kubeconfig OIDC authentication OR
2. Obtain write-access kubeconfig for ardenone-hub to scale down v0.1.11 and let v0.1.13 become Ready
