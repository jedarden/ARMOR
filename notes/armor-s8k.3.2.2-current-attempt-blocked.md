# armor-s8k.3.2.2 - Current Attempt: Still Blocked by Expired Credentials

## Date: 2026-05-02 08:42 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Current Attempt Results

### Access Attempts Summary
All access methods remain blocked as documented in previous attempts:

1. **ord-devimprint.kubeconfig** - Credentials expired (token exp: 1777689464)
2. **OIDC refresh** - Requires browser (xdg-open not available)
3. **kubectl-proxy** - traefik-ord-devimprint:8001 not configured
4. **ardenone-hub proxy** - Connection reset errors (unstable)

### Verified Cluster Status
Via read-only proxies:
- **traefik-ardenone-manager:8001** ✅ Working
- **traefik-ardenone-hub:8001** ⚠️  Unstable (connection resets)
- **traefik-rs-manager:8001** ✅ Working
- **traefik-ord-devimprint:8001** ❌ Not configured

### Aggregator Pod Location
The aggregator pod runs in the **ord-devimprint** cluster (not ardenone-hub):
- Cluster: ord-devimprint
- Namespace: devimprint
- Pod: aggregator-*

### Target Query (from parent bead armor-s8k.3.2)
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
**Cannot exec into aggregator pod without valid ord-devimprint cluster credentials**

The task requires kubectl exec access to run the query in-cluster with ARMOR S3 credentials. All available access methods are blocked:
- Expired kubeconfig cannot be refreshed without browser
- No kubectl-proxy available for ord-devimprint
- Local execution fails without in-cluster ARMOR credentials

## Resolution Required
1. Refresh `/home/coding/.kube/ord-devimprint.kubeconfig` via OIDC browser flow, OR
2. Configure kubectl-proxy on ord-devimprint cluster, OR
3. Provide alternative exec access method

## Related Documentation
- Previous blocker note: armor-s8k.3.2.2-credential-blocker-attempt.md
- Original blocker: armor-s8k.3.2.2-blocker-expired-credentials.md
- Parent bead: armor-s8k.3.2 (DuckDB httpfs test through ARMOR)
