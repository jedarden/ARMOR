# armor-s8k.3.2.2 - RBAC Blocker Investigation - 2026-05-03

## Task
Exec into aggregator pod and run DuckDB httpfs COUNT(*) query over s3://devimprint/commits/**/*.parquet

## Status: BLOCKED - RBAC Access Limitations

## Investigation Summary

### Access Constraints

| Method | Status | Issue |
|--------|--------|-------|
| ardenone-hub proxy (traefik-ardenone-hub:8001) | ❌ Read-only | RBAC blocks exec: "unable to upgrade connection: Forbidden" |
| ardenone-cluster proxy (traefik-ardenone-cluster:8001) | ❌ Read-only | RBAC blocks exec: "unable to upgrade connection: Forbidden" |
| ord-devimprint.kubeconfig | ❌ OIDC Auth | Requires browser-based re-authentication (not available in CLI environment) |
| ardenone-manager.kubeconfig | ❌ Missing | No write-access kubeconfig for ardenone-hub cluster |

### Verification Attempts

**1. ardenone-hub aggregator pod (aggregator-68554db644-ng85f):**
```
kubectl --server=http://traefik-ardenone-hub:8001 exec -n devimprint aggregator-68554db644-ng85f -- python3 -c "print('test')"
error: unable to upgrade connection: Forbidden
```
Result: RBAC blocks exec via read-only proxy.

**2. ardenone-cluster aggregator pod (aggregator-86dc959987-k6x2f):**
```
kubectl --server=http://traefik-ardenone-cluster:8001 exec -n devimprint aggregator-86dc959987-k6x2f -- python3 -c "print('test')"
error: unable to upgrade connection: Forbidden
```
Result: Same RBAC blocker.

**3. ord-devimprint.kubeconfig:**
```
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH
Please visit the following URL in your browser manually: http://localhost:8000/
error: get-token: authcode-browser error: context deadline exceeded
```
Result: Requires browser-based OIDC authentication.

### Current Service Status

**ardenone-cluster (devimprint namespace):**
```
NAME                          READY   STATUS    RESTARTS   AGE
aggregator-86dc959987-k6x2f   1/1     Running   0          14h
armor-68c6ddc78b-27cq6        1/1     Running   0          19h
armor-68c6ddc78b-6krfq        1/1     Running   0          19h
```

ARMOR service is healthy with 2 pods running. The aggregator pod is also running.

**ardenone-hub (devimprint namespace):**
```
NAME                          READY   STATUS    RESTARTS   AGE
aggregator-68554db644-ng85f   1/1     Running   9 (4h36m ago)   8d
```

Aggregator exists but ARMOR service was migrated to ardenone-cluster.

### Previous Verification (Complete)

The parent bead (armor-s8k.3.2) was **closed on 2026-05-01** with full verification:

| Criteria | Status | Evidence |
|----------|--------|----------|
| COUNT(*) returns non-zero integer | ✅ PASS | 1,283,067 parquet files found |
| No InvalidInputException | ✅ PASS | Clean execution |
| No date parse errors | ✅ PASS | ISO 8601 format working |
| ARMOR v0.1.11+ deployed | ✅ PASS | ronaldraygun/armor:0.1.11 running |

## Root Cause

The `devpod-observer` ServiceAccount used by the kubectl-proxy pods has read-only RBAC permissions. This prevents `kubectl exec` into pods, which is required by this task.

## Resolution Options

To complete this task as specified (exec into aggregator pod):

1. **Refresh ord-devimprint.kubeconfig** via Rackspace Spot dashboard (requires browser access)
2. **Create write-access kubeconfig** for ardenone-hub/ardenone-cluster with exec permissions
3. **Modify RBAC** for devpod-observer ServiceAccount to allow pod exec (not recommended for security)
4. **Run the query via alternative method** (e.g., create a Job with proper credentials)

## Recommendation

Since the verification objective was achieved on 2026-05-01 and production traffic confirms ongoing successful operation, consider:
- Accepting the previous verification results
- Updating the task to allow alternative verification methods
- Providing write-access credentials for re-verification
