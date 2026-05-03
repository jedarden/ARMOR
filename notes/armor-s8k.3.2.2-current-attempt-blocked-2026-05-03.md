# armor-s8k.3.2.2 - Current Attempt Blocked - 2026-05-03

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: BLOCKED - Multiple Issues

## Investigation Findings

### 1. ARMOR Migration Status
- **ardenone-hub**: ARMOR service NO LONGER EXISTS here
- **ardenone-cluster**: ARMOR is running (2 pods, Ready: 1/1)
- **aggregator-68554db644-ng85f**: Still pointing to `armor-svc:9000` on ardenone-hub (non-existent)

### 2. Access Constraints

| Method | Status | Issue |
|--------|--------|-------|
| ardenone-hub proxy (traefik-ardenone-hub:8001) | ❌ Read-only | RBAC blocks exec: "unable to upgrade connection: Forbidden" |
| ardenone-cluster proxy (traefik-ardenone-cluster:8001) | ❌ Read-only | RBAC blocks exec, no aggregator pod exists here |
| ord-devimprint.kubeconfig | ❌ Expired | OIDC token expired, requires browser re-auth |
| rs-manager.kubeconfig | ❌ Wrong cluster | No access to ardenone-hub or ardenone-cluster |

### 3. Service Discovery

**ardenone-hub:**
```
$ kubectl --server=http://traefik-ardenone-hub:8001 get svc -n devimprint
No resources found
```

**ardenone-cluster:**
```
$ kubectl --server=http://traefik-ardenone-cluster:8001 get svc -n devimprint
NAME    TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)             AGE
armor   ClusterIP   10.43.160.134   <none>        9000/TCP,9001/TCP   16h
```

### 4. Aggregator Logs (ardenone-hub)
```
urllib3.exceptions.NewConnectionError: AWSHTTPConnection(host='armor-svc', port=9000):
Failed to establish a new connection: [Errno 111] Connection refused
```

The aggregator is failing to connect to the non-existent ARMOR service on ardenone-hub.

## Root Cause
The aggregator pod on ardenone-hub is configured to connect to `armor-svc:9000` which no longer exists after the ARMOR migration to ardenone-cluster. The aggregator needs to be updated to point to the new ARMOR location.

## Resolution Required

To complete this task as specified (exec into aggregator pod on ardenone-hub):

1. **Refresh ord-devimprint.kubeconfig** via Rackspace Spot dashboard (browser required), OR
2. **Provide write-access kubeconfig** for ardenone-hub cluster, OR
3. **Provide S3 credentials** to run query locally or on iad-ci cluster, OR
4. **Update aggregator deployment** on ardenone-hub to point to ARMOR on ardenone-cluster via Tailscale

## Alternative Approach
Since the parent bead (armor-s8k.3.2) was already closed with successful verification on 2026-05-01, this task may be obsolete. The DuckDB httpfs COUNT(*) query was verified to work correctly at that time.

## Verification Status (from armor-s8k.3.2.2-attempt-2026-05-02-15.md)
- ✅ COUNT(*) returns non-zero integer (verified 2026-05-01)
- ✅ No InvalidInputException (clean execution)
- ✅ ARMOR v0.1.11+ deployed
- ✅ ISO 8601 timestamps in handlers.go
- ✅ Production evidence: 69,505+ rows processed, 0 date parse errors
