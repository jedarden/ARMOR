# armor-s8k.3.2.2 - Blocker Investigation - 2026-05-02 14:20 UTC

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: BLOCKED - Access Limitations

## Investigation Summary

### ord-devimprint.kubeconfig - EXPIRED OIDC TOKEN

Attempts to access ord-devimprint cluster via kubeconfig fail due to expired OIDC token:

```bash
# API server is reachable (ping 107ms, curl returns 403 for anon, 401 for expired token)
$ timeout 60 kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint
Exit code 124 (timeout)

# Direct API call with token:
$ curl -sk -H "Authorization: Bearer <token>" https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com/api/v1/namespaces/devimprint/pods
{"kind":"Status","status":"Failure","message":"Unauthorized","code":401}

# Token refresh attempt:
$ kubectl oidc-login get-token --oidc-issuer-url=https://login.spot.rackspace.com/ ...
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH
Please visit the following URL in your browser manually: http://localhost:8000/
```

**Root cause**: OIDC token embedded in kubeconfig has expired. The `kubectl oidc-login get-token` plugin requires interactive browser authentication to refresh, which is not available in this CLI-only environment.

**Cached token directory is empty**:
```
~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm/
total 8
-rw------- 1 coding coding    0 May  2 00:04 90e1f62b22c246b31866092f2b59453a9d1218ab22f4e4bb659eae988cbd2cc6.lock
```

### ardenone-hub proxy - READ-ONLY RBAC

The ardenone-hub kubectl-proxy provides read-only access via `devpod-observer` ServiceAccount:

```bash
$ kubectl --server=http://traefik-ardenone-hub:8001 exec -n devimprint aggregator-68554db644-ng85f -- python3 -c "print('test')"
error: unable to upgrade connection: Forbidden
```

RBAC explicitly blocks exec, port-forward, and any write operations through the proxy.

### Current Service Status (ardenone-hub, devimprint namespace)

**ARMOR Deployment:**
```
NAME                      READY   STATUS             RESTARTS        AGE
armor-755d878c84-l8grt    0/1     CrashLoopBackOff   64 (64s ago)    5h6m
armor-7c79d57db6-k2j6j    1/1     Running            32 (153m ago)   4h56m
```

**ARMOR Service Endpoints:**
```
NAME    ENDPOINTS                         AGE
armor   10.42.0.70:9001,10.42.0.70:9000   4d
```

**Aggregator Pods:**
```
NAME                          READY   STATUS    RESTARTS        AGE
aggregator-5d58d6c67-7gl9m    0/1     Pending   0               4d
aggregator-68554db644-ng85f   1/1     Running   9 (6h16m ago)   8d
```

Service endpoints are **ACTIVE** - the healthy pod (armor-7c79d57db6-k2j6j) is serving traffic despite one pod being in CrashLoopBackOff.

## Query to Run (from parent bead armor-s8k.3.2)

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

## Resolution Required

To complete this task as specified (exec into aggregator pod):
1. **Refresh ord-devimprint.kubeconfig** via Rackspace Spot dashboard with browser authentication
2. **Create write-access kubeconfig** for ardenone-hub cluster with exec privileges
3. **Provide alternative access method** (e.g., S3 credentials for local query, or bastion host with browser)

## Previous Verification Status

The parent bead (armor-s8k.3.2) was closed on 2026-05-01 with full verification of DuckDB httpfs functionality. This current bead is attempting to re-run the same verification query, but access limitations prevent execution.

## Network Diagnostics

- ord-devimprint API server: **reachable** (ping 107ms avg latency)
- API server responds to curl: **yes** (403 for anon, 401 for expired token)
- Route to API server: **healthy** (mtr shows clean path through Hetzner → Zayo → Rackspace)
- kubectl via kubeconfig: **times out** (likely waiting for token refresh prompt)
