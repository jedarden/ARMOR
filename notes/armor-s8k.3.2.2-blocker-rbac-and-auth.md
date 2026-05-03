# armor-s8k.3.2.2 - Blocker Investigation - RBAC and Authentication

## Date: 2026-05-03

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: BLOCKED - Access Limitations

## Investigation Summary

### Access Constraints

| Method | Status | Issue |
|--------|--------|-------|
| ardenone-hub proxy (traefik-ardenone-hub:8001) | ❌ Read-only | RBAC blocks exec: "unable to upgrade connection: Forbidden" |
| ord-devimprint.kubeconfig | ❌ OIDC Auth Expired | "Client.Timeout exceeded while awaiting headers" - requires browser re-auth |
| rs-manager.kubeconfig | ❌ Expired | "server has asked for the client to provide credentials" |
| ardenone-manager kubeconfig | ❌ Missing | File does not exist at expected path |
| iad-ci.kubeconfig | ✅ Working | But cluster has no access to devimprint/ARMOR resources |

### Verification of Blockers

**1. kubectl exec via ardenone-hub proxy:**
```
kubectl --server=http://traefik-ardenone-hub:8001 exec -n devimprint aggregator-68554db644-ng85f -- python3 -c "print('test')"
error: unable to upgrade connection: Forbidden
```
The devpod-observer ServiceAccount has read-only RBAC and cannot exec into pods.

**2. ord-devimprint.kubeconfig OIDC timeout:**
```
timeout 15 kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint
Terminated (timeout)
```
OIDC token requires browser-based re-authentication via Rackspace Spot dashboard.

**3. ARMOR endpoint reachable but requires credentials:**
```
curl -sk https://devimprint-armor-tailscale-ingress.tail1b1987.ts.net/
<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Invalid credentials</Message>
</Error>
```
Credentials are stored in Kubernetes secret `devimprint-armor-writer` which is not readable via read-only proxy.

### Current Service Status

**ARMOR Service (ardenone-hub, devimprint namespace):**
```
NAME                     READY   STATUS    RESTARTS        AGE
armor-755d878c84-l8grt   0/1     Running   54 (5m ago)     4h20m
armor-7c79d57db6-k2j6j   1/1     Running   32 (101m ago)   4h10m

NAME    ENDPOINTS                         AGE
armor   10.42.0.70:9001,10.42.0.70:9000   3d23h
```
Service endpoints are ACTIVE with one healthy pod.

**Aggregator Pod (ardenone-hub):**
```
aggregator-68554db644-ng85f   1/1     Running   9 (4h36m ago)   8d
```
Pod is healthy and running.

**Aggregator Pod (ord-devimprint):**
```
aggregator-6949b669d5-2wzkc   1/1     Running   0             14h
```
One healthy pod exists on ord-devimprint cluster.

### Query to Run (from parent bead)

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

Credentials come from:
- `S3_ACCESS_KEY_ID`: secret `devimprint-armor-writer`, key `auth-access-key`
- `S3_SECRET_ACCESS_KEY`: secret `devimprint-armor-writer`, key `auth-secret-key`

### Previous Verification (Complete)

The parent bead (armor-s8k.3.2) was **closed on 2026-05-01** with full verification:

| Criteria | Status | Evidence |
|----------|--------|----------|
| COUNT(*) returns non-zero integer | ✅ PASS | 1,283,067 parquet files found |
| No InvalidInputException | ✅ PASS | Clean execution |
| No date parse errors | ✅ PASS | ISO 8601 format working |
| ARMOR v0.1.11+ deployed | ✅ PASS | ronaldraygun/armor:0.1.11 running |

### Production Evidence

Aggregator logs show successful operation:
- Processing 69,505+ rows per cycle
- 14,713+ successful HTTP 200 requests to ARMOR
- 0 HTTP 400 errors
- 0 date parse errors

## Resolution Required

To complete this task as specified (exec into aggregator pod):
1. **Refresh ord-devimprint.kubeconfig** via Rackspace Spot dashboard (browser required), OR
2. **Create write-access kubeconfig** for ardenone-hub cluster with exec permissions, OR
3. **Provide S3 credentials** to run query locally (bypasses kubectl exec requirement)

## Recommendation

The verification objective was achieved on 2026-05-01 and production traffic confirms ongoing successful operation. The blocker is purely access limitations for re-running the exact command as specified.

## References

- Parent bead close reason: "Verification complete. DuckDB httpfs glob expansion confirmed working with ARMOR v0.1.11. ISO 8601 fix verified in code (handlers.go) and deployment."
- Previous verification notes: notes/armor-s8k.3-verification-2026-05-01-v0.1.11.md
