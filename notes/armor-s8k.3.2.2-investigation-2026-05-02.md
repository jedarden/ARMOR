# armor-s8k.3.2.2 Investigation Summary

**Date:** 2026-05-02 10:47 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Findings

### 1. Aggregator Pod Location
- **Cluster:** ardenone-hub
- **Namespace:** devimprint
- **Pod:** aggregator-68554db644-ng85f (Running, but with issues)

### 2. RBAC Blocker - Read-Only Proxy Only
ardenone-hub cluster is only accessible via read-only kubectl-proxy:
```bash
kubectl --server=http://traefik-ardenone-hub:8001
# or via Tailscale IP: http://100.90.7.50:8001
```

**Blocked operations:**
- `kubectl exec` - Forbidden by devpod-observer RBAC
- `kubectl port-forward` - Forbidden
- Secret access - Forbidden (`devimprint-armor-writer` secret is not readable)

### 3. Service Outage - MinIO/Armor Down
The armor pods (MinIO-compatible S3 service) are **CrashLoopBackOff**:
```
armor-755d878c84-l8grt   0/1     CrashLoopBackOff   16 (85s ago)   66m
armor-7c79d57db6-k2j6j   0/1     CrashLoopBackOff   14 (97s ago)   57m
```

**Armor service endpoints:** EMPTY (no ready pods)
```
NAME        ENDPOINTS   AGE
armor-svc               19d
```

**Aggregator logs show connection failures:**
```
botocore.exceptions.EndpointConnectionError: Could not connect to the endpoint URL: "http://armor-svc:9000/devimprint/state/backfill_cursor.txt"
WARNING no commit data in 30d window — skipping this cycle
```

**Armor pod logs show it starts but immediately crashes:**
```
{"time":"2026-05-02T14:41:51.679101242Z","level":"INFO","service":"armor","msg":"ARMOR starting"...}
# Then liveness probe fails repeatedly
```

**Likely cause:** Missing or invalid secrets (devimprint-armor-mek, devimprint-armor-writer, devimprint-armor-readonly)

### 4. ord-devimprint Kubeconfig - Expired Credentials
Located at `/home/coding/.kube/ord-devimprint.kubeconfig`:
- **ngpc-user token:** Expired on 2026-04-27 (5 days ago)
- **OIDC context:** Requires browser-based authentication
- **OIDC refresh command:**
  ```bash
  kubectl oidc-login get-token --force-refresh \
    --oidc-issuer-url=https://login.spot.rackspace.com/ \
    --oidc-client-id=mwG3lUMV8KyeMqHe4fJ5Bb3nM1vBvRNa \
    --oidc-auth-request-extra-params=organization=org_KsELolwAOxl3Zxfm
  ```
- **Blocker:** Opens browser on localhost:18000 - ADB phone cannot access server localhost

### 5. Missing ardenone-hub Write Kubeconfig
CLAUDE.md documents `~/.kube/ardenone-manager.kubeconfig` but:
- File exists but is empty/invalid (0 lines)
- No cluster-admin access to ardenone-hub available

### 6. ArgoCD Read-Only API Confirms Cluster
```bash
curl -sk https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/clusters
# Returns: {"name": "ardenone-hub", "server": "https://ardenone-hub.ardenone.com:6443"}
```

## Root Cause Analysis
This task is blocked by **three independent issues**:

1. **RBAC:** ardenone-hub only has read-only proxy access (intentional security restriction)
2. **Service outage:** armor/MinIO pods are CrashLoopBackOff (likely invalid/missing secrets)
3. **Credential expiration:** ord-devimprint kubeconfig token expired, OIDC refresh requires browser

## Resolution Required (One of)

### Option A: Fix armor pods (addresses root cause of service outage)
Requires:
- Write access to ardenone-hub cluster
- Ability to read/create secrets `devimprint-armor-mek`, `devimprint-armor-writer`, `devimprint-armor-readonly`
- Diagnosis of why armor pods are crashing (likely secret validation failure)

### Option B: Fresh ord-devimprint OIDC token
Requires:
- Interactive browser session (cannot be done via ADB phone)
- Visit to `http://localhost:18000` for OIDC callback

### Option C: ardenone-hub write kubeconfig
Requires:
- New kubeconfig with cluster-admin or exec permissions on ardenone-hub
- Credential refresh via Spot Rackspace console

### Option D: Direct S3 credentials
Requires:
- S3 endpoint URL accessible from this server
- S3 access key and secret key
- The armor-svc needs to be running first (currently down)

## Query to Execute (when access resolved)
```python
import duckdb

con = duckdb.connect()
con.execute('''
    INSTALL httpfs;
    LOAD httpfs;
    SET s3_region='us-west-002';
    SET s3_endpoint='http://armor-svc:9000';
    SET s3_access_key_id='<from devimprint-armor-writer secret>';
    SET s3_secret_access_key='<from devimprint-armor-writer secret>';
''')

result = con.execute('''
    SELECT COUNT(*) FROM read_parquet('s3://devimprint/commits/**/*.parquet')
''').fetchone()

print(f'COUNT(*): {result[0]}')
```

## Acceptance Criteria
- Non-zero COUNT(*) result
- No InvalidInputException or date parse errors in output
- Full output copied to bead comment

## Status
**BLOCKED** - Cannot complete task without resolving one of the above options.

**Critical finding:** The S3 service itself (armor/MinIO) is down due to pod crashes. Even if exec access were granted, the query would fail because the S3 backend is unavailable.

## Recommendation
Priority should be fixing the armor/MinIO pod CrashLoopBackOff issue. The aggregator is already failing to connect to S3, indicating a broader service outage affecting the devimprint namespace.
