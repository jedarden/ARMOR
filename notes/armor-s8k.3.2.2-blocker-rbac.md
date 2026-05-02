# armor-s8k.3.2.2 - Blocked: RBAC Constraints Prevent kubectl exec

## Date: 2026-05-02 08:52 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Current Blocker
**RBAC constraints on read-only kubectl-proxy prevent exec into pods**

### Environment
- **Cluster:** ardenone-hub (devimprint namespace migrated from ord-devimprint)
- **Aggregator Pod:** aggregator-68554db644-ng85f (Running ✓)
- **Access Method:** kubectl-proxy via Tailscale (http://traefik-ardenone-hub:8001)

### Aggregator Pod Configuration
```
Container: aggregator
Image: ronaldraygun/devimprint-aggregator:latest
Environment:
  S3_ENDPOINT = http://armor-svc:9000
  S3_BUCKET = devimprint
  AGGREGATE_INTERVAL_SECS = 1800
  S3_ACCESS_KEY_ID = (from secretRef: devimprint-armor-writer)
ServiceAccount: default
```

### Access Attempts

1. **Read-only kubectl-proxy (RBAC blocks exec)**
   ```bash
   kubectl --server=http://traefik-ardenone-hub:8001 exec -n devimprint aggregator-68554db644-ng85f -- python3 -c "..."
   # Error: unable to upgrade connection: Forbidden
   ```

2. **ord-devimprint kubeconfig (credentials expired)**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint
   # Error: You must be logged in to the server (OIDC token expired)
   ```

3. **ardenone-hub kubeconfig (does not exist)**
   - No write-access kubeconfig available for ardenone-hub cluster
   - rs-manager kubeconfig only grants access to rs-manager cluster

### Root Cause
The devimprint namespace migrated from ord-devimprint cluster to ardenone-hub cluster. However:
- No write-access kubeconfig exists for ardenone-hub
- The read-only proxy (serviceaccount: devpod-observer) intentionally blocks exec
- ord-devimprint kubeconfig credentials are expired (OIDC)

### ArgoCD Verification
```bash
curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/clusters
# Confirms ardenone-hub is reachable: https://ardenone-hub.ardenone.com:6443
# ConnectionState: Successful
```

## Resolution Required
To complete armor-s8k.3.2.2, need ONE of:
1. **Write kubeconfig for ardenone-hub** with cluster-admin or exec permissions
2. **kubectl-proxy with exec access** on ardenone-hub (upgrade devpod-observer RBAC)
3. **Fresh OIDC token** for ord-devimprint kubeconfig (requires browser auth)
4. **MinIO credentials** to run DuckDB query locally instead of via pod exec

## Query to Execute (when access is resolved)
```python
import duckdb

con = duckdb.connect('''
INSTALL httpfs;
LOAD httpfs;
SET s3_region='us-east-1';
SET s3_endpoint='http://armor-svc:9000';
SET s3_access_key_id='<from devimprint-armor-writer secret>';
SET s3_secret_access_key='<from devimprint-armor-writer secret>';
''')

result = con.execute('''
    SELECT COUNT(*) FROM read_parquet('s3://devimprint/commits/**/*.parquet')
''').fetchone()

print(f'COUNT(*): {result[0]}')
```

## Related
- Parent bead: armor-s8k.3.2.1 (verified aggregator pod Running on ardenone-hub)
- Notes: armor-s8k.3.2.2-blocker-expired-credentials.md (ord-devimprint credential issues)
