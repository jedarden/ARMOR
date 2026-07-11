# Bead bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID

## Status: FAILED - Prerequisite incomplete

## Issue Summary
This child bead cannot complete because its prerequisite (retrieving the base64-encoded value from Kubernetes) failed due to RBAC restrictions.

## Root Cause
The kubectl-proxy for `ord-devimprint` cluster runs with read-only RBAC that **explicitly blocks secret access**. The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in the `devimprint` namespace.

### Error Details
```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

### Command Attempted (by previous bead)
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Evidence
File `/tmp/litestream_key_id.b64` contains only the RBAC error message, not the actual base64 value:
```
RBAC BLOCKER: Cannot retrieve secret value
Error from server (Forbidden): secrets "armor-writer" is forbidden...
```

## Acceptance Criteria Not Met
- ❌ Successfully decoded the base64 value to plain text - **Cannot decode, file contains error not value**
- ❌ Decoded value is not empty - **N/A, no value retrieved**
- ❌ Value appears valid (starts with AKIA...) - **N/A, no value retrieved**
- ❌ Value is human-readable - **N/A, no value retrieved**

## Resolution Path
To complete this bead, the parent bead must first successfully retrieve the base64 value. Options:

1. **Use direct kubeconfig** (if available for ord-devimprint):
   - Bypass the read-only proxy
   - Requires ServiceAccount with secret read permissions

2. **Use ExternalSecrets cached values** (if synced):
   - Check if ExternalSecrets has already synced to a readable location
   - May accept cached values as valid (see bf-520v learning)

3. **Alternative cluster access**:
   - Check if the secret exists in another cluster with better access
   - ArgoCD may have synced it elsewhere

4. **Manual provision**:
   - Manually provide the base64 value through an alternative channel
   - Accept external input (not from Kubernetes)

## Historical Context
Git history shows 10+ previous attempts all failed with the same RBAC blocker:
- e7631bc: "Document retry - RBAC blocker prevents decode, prerequisite failed"
- 5c5f4bd: "Document RBAC blocker preventing decode completion"
- 22cd44d: "Document verification - RBAC blocker persists"
- 4f3973c: "Document final attempt failure - RBAC blocker prevents secret retrieval"
- ... (multiple similar commits)

The pattern indicates this is a persistent access constraint that cannot be bypassed through the read-only proxy.

## Cluster Access Notes
According to CLAUDE.md, ord-devimprint is documented as:
- **Read-only via proxy**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **Access**: Read-only — cannot create, delete, or modify resources
- **No direct kubeconfig documented**: Unlike other clusters (ardenone-manager, rs-manager, iad-ci), ord-devimprint has no documented read-write kubeconfig path

This suggests ord-devimprint may only be accessible via the read-only proxy, making secret retrieval inherently blocked.

## Recommendation
**Do NOT close this bead.** Release it for retry after:
1. The parent bead's prerequisite is completed with alternative access method, OR
2. An alternative secret retrieval method is implemented, OR
3. The secret value is provided through an external channel

## Timestamp
2026-07-11 13:21 UTC (initial attempt)
2026-07-11 ~14:00 UTC (updated with comprehensive analysis)
2026-07-11 ~14:15 UTC (retry attempt - same RBAC blocker confirmed, no changes to situation)
2026-07-11 ~14:30 UTC (another retry - same RBAC blocker, file contains error text not base64 value)
2026-07-11 ~14:45 UTC (latest retry - same RBAC blocker persists, prerequisite still not met)
2026-07-11 ~20:05 UTC (retry #12 - same RBAC blocker, situation unchanged, file still contains error text not base64 data)
2026-07-11 ~21:30 UTC (retry #13 - same RBAC blocker, prerequisite still not met, no progress)
2026-07-11 ~22:00 UTC (retry #14 - same RBAC blocker persists, file /tmp/litestream_key_id.b64 still contains error text not base64 data, no progress)
2026-07-11 ~23:30 UTC (retry #15 - same RBAC blocker, base64 -d fails with "invalid input" because file contains error text not base64 data, prerequisite still not met, no progress)
2026-07-12 ~00:15 UTC (retry #16 - same RBAC blocker persists, prerequisite still not met, no progress - file still contains RBAC error message not base64 data)
