# armor-s8k.3.2.2 - Blocked: Expired Kubeconfig Credentials

## Date: 2026-05-02 08:40 UTC

## Attempt Summary
Attempted to exec into aggregator pod on ord-devimprint cluster to run DuckDB httpfs COUNT(*) query. Blocked by expired kubeconfig credentials and no available kubectl-proxy.

## Access Attempts

### 1. Direct kubeconfig (ngpc-user context)
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint -l app=aggregator
# Error: You must be logged in to the server (the server has asked for the client to provide credentials)
```
Token expired: `exp: 1777689464` (embedded in kubeconfig)

### 2. OIDC authentication attempt
```bash
kubectl oidc-login get-token --oidc-issuer-url=https://login.spot.rackspace.com/ ...
# Error: could not open the browser: xdg-open not found in $PATH
# Requires manual browser interaction: http://localhost:8000/
```
Requires interactive browser - cannot complete in automated context.

### 3. Tailscale kubectl-proxy
```bash
kubectl --server=http://traefik-ord-devimprint:8001 get pods -n devimprint
# No such host: traefik-ord-devimprint
```
No proxy configured for ord-devimprint cluster.

### 4. Direct Tailscale API access
```bash
host ord-devimprint.tail1b1987.ts.net
# 100.116.10.78

curl -k https://ord-devimprint.tail1b1987.ts.net:6443/...
# Connection refused
```
Tailscale route exists but API server not accessible.

## Alternative Approaches Considered

### Local DuckDB with direct S3 access
Attempted to run query locally with DuckDB Python:
```python
import duckdb
con = duckdb.connect()
con.execute("INSTALL httpfs; LOAD httpfs;")
con.execute("SET s3_region='us-east-1'")
result = con.execute("SELECT COUNT(*) FROM 's3://devimprint/commits/**/*.parquet'").fetchone()
```
Result: `NoSuchBucket: The specified bucket does not exist`
- The `devimprint` bucket is an ARMOR-encrypted B2 bucket, not direct S3
- Requires ARMOR proxy credentials which are only available in-cluster

### S3 credential lookup
Checked declarative-config for S3 credentials:
```bash
grep -r "S3_ACCESS_KEY_ID\|S3_SECRET_ACCESS_KEY" ~/declarative-config/
```
Credentials are referenced as Kubernetes Secret references, not stored in plain text.

## Task Requirements (from armor-s8k.3.2)
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
Requires:
- Exec into aggregator pod (for env vars)
- Network access to armor:9000 service
- Valid ARMOR S3 credentials

## Blocker Summary
| Method | Status | Reason |
|--------|--------|--------|
| Direct kubeconfig | ❌ | Token expired |
| OIDC refresh | ❌ | Requires browser |
| kubectl-proxy | ❌ | Not configured |
| Local execution | ❌ | No ARMOR credentials |

## Resolution Required
1. **Refresh kubeconfig credentials** - Re-run OIDC login flow with browser
2. **Deploy kubectl-proxy** - Add traefik-ord-devimprint proxy pod like other clusters
3. **Provide credential file** - Alternative access method

## Related Beads
- Parent: armor-s8k.3.2 (DuckDB httpfs test through ARMOR)
- Prerequisite: armor-s8k.3.1 (ARMOR v0.1.8 deployed and verified)

## Files Referenced
- `~/declarative-config/k8s/ord-devimprint/devimprint/aggregator-deployment.yml`
- `~/declarative-config/k8s/ord-devimprint/devimprint/armor-deployment.yml`
- `~/.kube/ord-devimprint.kubeconfig`
