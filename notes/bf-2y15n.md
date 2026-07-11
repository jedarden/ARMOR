# Task bf-2y15n: BLOCKED - Cannot retrieve secret value

## Objective
Retrieve base64-encoded `LITESTREAM_ACCESS_KEY_ID` field from the `armor-writer` secret in the `ord-devimprint` cluster.

## Blocker
**Infrastructure issue:** ord-devimprint cluster has no kubeconfig with secret access.

## Attempts Made

### 1. Direct kubeconfig (specified in task)
Command attempted:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

Result: `stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory`

### 2. Read-only proxy (fallback)
Command attempted:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

Result: 
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Root Cause Analysis

According to `/home/coding/CLAUDE.md`:

1. **ord-devimprint access pattern:**
   - Only has a read-only kubectl-proxy (no Traefik, exposed via Tailscale operator)
   - Proxy runs in `devpod-observer` namespace with read-only RBAC
   - **Access is read-only — cannot create, delete, or modify resources**
   - Explicitly denies secrets access (stricter than other clusters)

2. **Comparison with other clusters:**
   - `ardenone-manager`: Has both read-only proxy **AND** direct kubeconfig with cluster-admin
   - `rs-manager`: Has both read-only proxy **AND** direct kubeconfig with cluster-admin
   - `iad-options`: Has both read-only proxy **AND** direct kubeconfig (cloudspace-admin OIDC token)
   - `ord-devimprint`: **Only has read-only proxy** (no direct kubeconfig)

## Resolution Required

To unblock this task, one of the following is needed:

1. **Preferred:** Create a direct kubeconfig for ord-devimprint with secret access
   - Similar to `/home/coding/.kube/ardenone-manager.kubeconfig`
   - Would allow direct `kubectl --kubeconfig` access to secrets

2. **Alternative:** Grant the devpod-observer ServiceAccount secrets read access
   - Modify RBAC in ord-devimprint cluster
   - Less secure but would allow proxy-based access

## Related Documentation

This blocker has been documented in previous commits:
- `7b87f78f` - docs(bf-2y15n): document blocking issue - no secret access on ord-devimprint
- `af478157` - docs(bf-2y15n): re-verify blockers persist - kubeconfig missing, proxy blocks secrets
- `d82d1463` - docs(bf-2y15n): document blocking issue - ord-devimprint has no kubeconfig with secret access
- `fdc9b957` - docs(bf-2y15n): document blocking issue - missing ord-devimprint.kubeconfig

## Current Attempt (2026-07-11)

Attempted the specified command again:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

Result: `error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory`

Confirmed the blocker persists - the kubeconfig file still does not exist, and the read-only proxy denies secrets access as documented in previous attempts.

## Status
**BLOCKED** - Cannot complete until infrastructure access is resolved. Task requires either:
- Direct kubeconfig for ord-devimprint with secret access
- RBAC modification to grant devpod-observer ServiceAccount secrets read access
