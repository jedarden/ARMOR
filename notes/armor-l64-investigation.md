# ARMOR CrashLoopBackOff Investigation - ord-devimprint

## Issue Description (from bug report)
- ARMOR pod on ord-devimprint (namespace: devimprint) was in CrashLoopBackOff with 67+ restarts over 19h
- ARMOR starts fine, logs show successful startup listening on :9000
- After ~90s, receives SIGTERM and exits cleanly (exit code 0)
- Liveness probe: http://:9000/healthz, initialDelay=10s, period=30s, failureThreshold=3 → would kill after ~100s

## Current State (2026-05-01 19:30 UTC)

### Deployment Status
- **Image:** ronaldraygun/armor:0.1.11 (upgraded from v0.1.8)
- **Replicas:** 3/3 Ready, 3/3 Available
- **Restart counts:** 0 for active pods

### Pod Status
| Pod | Started | Restarts | Status |
|-----|---------|----------|--------|
| armor-68c76f9499-22qbb | 2026-05-01T18:49:16Z | 0 | Running |
| armor-68c76f9499-bjngg | 2026-05-01T16:35:46Z | 0 | Running |
| armor-68c76f9499-h8n9w | 2026-05-01T16:30:32Z | 0 | Running |
| armor-68c76f9499-mrxjq | 2026-05-01T16:36:49Z | 1 | Failed (exit 137) |
| armor-8659dcf6fd-j2nn2 | 2026-04-30T16:35:14Z | 1 | Failed (exit 137) |

### Canary Status
```json
{
  "status": "healthy",
  "last_check": "2026-05-01T19:24:20.315157164Z",
  "upload_latency_ms": 72,
  "download_latency_ms": 66,
  "decrypt_verified": true,
  "hmac_verified": true,
  "cloudflare_cache_hit": false
}
```

### Website-Builder Status
- **Status:** Running (website-builder-768c784df5-s6nj7)
- **Builds:** Successfully completing digest builds
- **Connectivity:** Successfully connecting to armor:9000

### Configuration
- **B2 Region:** us-west-002
- **ARMOR_MANIFEST_ENABLED:** false
- **Liveness probe:** failureThreshold=5 (not 3 as in bug report)
- **Readiness probe:** period=60s, failureThreshold=5

## Findings

1. **Issue appears resolved:** The deployment is healthy with 3/3 replicas running and 0 restarts on active pods.

2. **Version upgrade:** The current image is 0.1.11, which is newer than v0.1.8 shown in deployment history.

3. **ExternalSecret update:** Events show that armor-credentials secret was updated at 19:23:59Z, which may have resolved credential issues.

4. **Probe configuration change:** The current deployment has `failureThreshold=5` for liveness, while the bug report mentioned `failureThreshold=3`.

5. **No SIGTERM observed:** The failed pods show exit code 137 (SIGKILL), not exit code 0 (SIGTERM) as mentioned in the bug report.

## Possible Root Causes (if issue recurs)

1. **B2 backend reachability:** If ORD cluster cannot reach B2 backend, canary checks would fail
2. **Missing credentials:** Expired or missing B2 credentials would cause immediate failures
3. **Goroutine panic:** An unhandled panic could kill the HTTP server, making /healthz unreachable
4. **Resource exhaustion:** Memory/CPU limits being hit

## Verification Steps

```bash
# Check pod status
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get pods -n devimprint -l app=armor

# Check canary status (via port-forward)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig port-forward -n devimprint armor-68c76f9499-22qbb 9001:9001
curl http://localhost:9001/armor/canary

# Check logs for panics
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig logs -n devimprint armor-68c76f9499-22qbb --tail=100
```

## Notes

- Kubeconfig token status: Working (no expiration issues observed)
- The pods are handling requests successfully (PUT and GET operations visible in logs)
- No evidence of the original CrashLoopBackOff issue in current state
