# ARMOR CrashLoopBackOff Investigation (armor-l64)

## Issue Summary

Investigated reported "CrashLoopBackOff on ord-devimprint" with symptoms:
- ARMOR pod reportedly restarting after ~90s uptime
- Exit code 0 (SIGTERM)
- 67+ restarts over 19h

## Root Cause

**NOT a CrashLoopBackOff.** The problematic pod was **Evicted due to memory pressure**:

```
Status: Failed
Reason: Evicted
Message: The node was low on resource: memory. Threshold quantity: 100Mi,
available: 496Ki. Container armor was using 349704Ki, request is 256Mi.
Exit Code: 137 (SIGKILL, not SIGTERM)
```

Pod: `armor-8659dcf6fd-j2nn2` on node `prod-instance-17768682057986746`

## Current State (2026-05-01)

All ARMOR pods are healthy:
- `armor-569df984f4-x6t4h`: 1/1 Ready, 0 restarts, 3h54m old
- `armor-569df984f4-pxwmr`: 1/1 Ready, 0 restarts, 3m32s old
- `armor-569df984f4-t4g6n`: 1/1 Ready, 0 restarts, 2m27s old

Canary status: **Healthy** on all pods
- Upload latency: ~500ms
- Download latency: ~70ms
- Decrypt verified: true
- HMAC verified: true

## Key Findings

1. **Liveness probe (`/healthz`)**: Pure liveness check, always returns 200
   - No backend checks
   - Would kill pod after 10 + 5×30 = 160s of failures

2. **Readiness probe (`/readyz`)**: Uses in-memory canary status
   - Returns 503 when canary is unhealthy
   - Does NOT kill pods (only affects traffic routing)

3. **Canary behavior**:
   - Runs immediately on startup
   - Completes in ~500ms (upload + download)
   - Retries 3× with 10s delay on failure
   - Interval: 5 minutes

4. **Pod readiness timing**:
   - `initialDelaySeconds: 5`
   - `periodSeconds: 60`
   - Pods become Ready within ~60-90s of startup

## Resolution

The issue was resolved by the rolling deployment that replaced the old replica set (`armor-8659dcf6fd`) with a new one (`armor-569df984f4`). The evicted pod remains in `ContainerStatusUnknown` but is not serving traffic.

## Recommendations

1. **Monitor node memory**: The eviction was caused by node memory pressure. Consider:
   - Increasing node memory capacity
   - Reducing pod memory requests/limits
   - Adding more nodes to the cluster

2. **No ARMOR code changes needed**: The ARMOR code is functioning correctly. The health probes, canary monitor, and shutdown handlers are all working as designed.

3. **Investigation artifact**: The old evicted pod (`armor-8659dcf6fd-j2nn2`) can be deleted manually if needed, or will be cleaned up by Kubernetes garbage collection.
