# Bead bf-2fdy0: RBAC Blocker - Cannot Retrieve SECRET_ACCESS_KEY

## Issue
Cannot retrieve LITESTREAM_SECRET_ACCESS_KEY from armor-writer secret in devimprint namespace due to RBAC restrictions.

## Attempts Made

### 1. Direct kubeconfig (FAILED)
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```
**Error:** Kubeconfig file `/home/coding/.kube/ord-devimprint.kubeconfig` does not exist.

### 2. Kubectl proxy (FAILED)
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
```
**Error:** `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

## Root Cause
- The ord-devimprint cluster only has read-only kubectl-proxy access via the `devpod-observer` ServiceAccount
- This ServiceAccount explicitly denies secret access (stricter than other clusters' observers)
- No direct kubeconfig exists for ord-devimprint with elevated permissions

## Resolution Options
1. Request elevated RBAC permissions for devpod-observer SA (not recommended - breaks security model)
2. Create a dedicated kubeconfig with secret access for devimprint cluster
3. Use ExternalSecret/EternalSecret pattern to sync secrets to a cluster with appropriate access
4. Obtain the secret value through an alternative authorized channel

## Latest Verification (2026-07-12)

Attempted retrieval via kubectl-proxy:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'
```

**Result:** Exit code 1
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Status
**BLOCKED** - Cannot proceed without elevated permissions or alternative secret access method. The RBAC blockade persists; secret access is explicitly denied for the devpod-observer ServiceAccount on ord-devimprint cluster.

## Related Beads
- bf-520v: Similar RBAC blocker documented
- Various session beads noting ExternalSecret sync issues
