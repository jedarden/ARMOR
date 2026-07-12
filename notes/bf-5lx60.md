# BF-5LX60: Secret Extraction Blocked by RBAC

## Task
Extract base64-encoded LITESTREAM_ACCESS_KEY_ID from the armor-writer secret in ord-devimprint cluster.

## Command Attempted
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Result
❌ **RBAC restriction:** The `devpod-observer` ServiceAccount cannot read secrets in the `devimprint` namespace

Error:
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Root Cause
The ord-devimprint cluster is only exposed via a read-only kubectl-proxy (`devpod-observer` serviceaccount) that explicitly denies secret access. This is consistent with security best practices for observer proxies.

## Prerequisite Status
The bead prerequisite stated "Previous child bead complete (kubeconfig works, secret exists)" — however, the verification bead (bf-69dku) was **unable to complete** due to this same RBAC blocker. The secret's existence could not be verified.

## Acceptance Criteria Status
- ❌ Successfully retrieved the base64-encoded value (RBAC blocked)
- ❌ Value is not empty (cannot retrieve)
- ❌ kubectl command completed without error (exit code 1)

## Resolution Required
This bead cannot complete without one of the following:
1. A direct kubeconfig with cluster-admin or secret-read permissions for ord-devimprint
2. RBAC modification to grant the devpod-observer SA secret read access in the devimprint namespace
3. Alternative access method to ord-devimprint cluster

## Related Beads
- **bf-69dku:** Verification bead — blocked by same RBAC issue
- **bf-4rqy0:** Another ord-devimprint operation — likely blocked by same RBAC issue

## Re-attempt (2026-07-11 ~20:13)

Re-tried the same command to verify if RBAC restrictions had changed:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

**Result:** Same RBAC error - the restriction persists. The `devpod-observer` ServiceAccount still cannot read secrets in the `devimprint` namespace.

Error output:
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

This confirms the RBAC blocker is a permanent limitation for ord-devimprint cluster access via the kubectl-proxy.

## Re-attempt #2 (2026-07-11 ~20:16)

Third verification attempt - same command, same result:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

**Result:** Identical RBAC error - no change in access permissions.

Exit code: 1
Error: `secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

## Recommendation
Document this as a known RBAC blocker for ord-devimprint cluster operations. Future beads requiring secret access on this cluster should account for this limitation.

---
*Initial Date: 2026-07-11*
*Re-attempt #1: 2026-07-11 ~20:13*
*Re-attempt #2: 2026-07-11 ~20:16*
*Cluster: ord-devimprint*
*Proxy: kubectl-proxy-ord-devimprint:8001 (read-only, no secret access)*
