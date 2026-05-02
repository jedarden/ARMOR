# armor-s8k.3.2.2 - RBAC Blocker Investigation

## Date: 2026-05-02 09:48 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Investigation Summary

### Target Pod Location
The aggregator pod required for this task runs on **ord-devimprint** cluster:
- Cluster: ord-devimprint (Rackspace Spot, ORD region)
- Namespace: devimprint
- Pod: aggregator-*
- Contains S3 credentials: S3_ACCESS_KEY_ID, S3_SECRET_ACCESS_KEY
- ARMOR endpoint: armor:9000

### Access Attempts Status

| Method | Status | Details |
|--------|--------|---------|
| ord-devimprint.kubeconfig | ❌ Expired | Token exp: 1777689464, requires browser OIDC flow |
| rs-manager.kubeconfig | ❌ Expired | Credentials expired |
| apexalgo-iad.kubeconfig | ❌ Offline | Connection refused |
| kubectl-proxy (traefik-ord-devimprint:8001) | ❌ Not configured | No proxy available |
| ardenone-cluster proxy | ⚠️ Read-only | Found options-aggregator but uses SeaweedFS, not S3 |
| iad-ci.kubeconfig | ✅ Working | No aggregator pod present |
| iad-acb.kubeconfig | ✅ Working | No aggregator pod present |

### Key Finding
The aggregator pod on ardenone-cluster (`options-aggregator-*`) uses **SeaweedFS**, not S3:
```
SEAWEEDFS_ENDPOINT
SEAWEEDFS_BUCKET
SEAWEEDFS_SITE_PREFIX
```

This pod does NOT have the required S3 credentials for querying s3://devimprint/commits/**/*.parquet

### RBAC Constraint
The kubectl-proxy on ardenone-cluster provides **read-only access** (devpod-observer namespace):
- Cannot exec into pods (Forbidden)
- Cannot run commands or queries
- Only get/list/describe operations allowed

## Target Query (from parent bead armor-s8k.3.2)
```python
import duckdb, os
con = duckdb.connect()
con.execute("INSTALL httpfs; LOAD httpfs;")
con.execute("SET s3_endpoint='armor:9000';")
con.execute("SET s3_use_ssl=false;")
con.execute(f"SET s3_access_key_id='{os.environ['S3_ACCESS_KEY_ID']}';")
con.execute(f"SET s3_secret_access_key='{os.environ['S3_SECRET_ACCESS_KEY']}';")
con.execute("SET s3_url_style='path';")
result = con.execute("SELECT COUNT(*) FROM read_parquet('s3://devimprint/commits/**/*.parquet')").fetchone()
print('Row count:', result[0])
```

## Blocker
**Cannot exec into ord-devimprint aggregator pod without valid credentials**

## Resolution Required
1. Refresh `/home/coding/.kube/ord-devimprint.kubeconfig` via Rackspace Spot dashboard with browser
2. OR configure kubectl-proxy on ord-devimprint cluster with exec permissions
3. OR provide alternative exec access method to ord-devimprint aggregator pod

## Related Issues
- armor-bik: "Refresh ord-devimprint kubeconfig token" (in_progress, 39 failures)
