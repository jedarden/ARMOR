# bf-2y15n: Blocking Issue - No Secret Access on ord-devimprint

## Problem

The task requires retrieving `LITESTREAM_ACCESS_KEY_ID` from the `armor-writer` secret in the `devimprint` namespace on the ord-devimprint cluster.

## Root Cause

**ord-devimprint cluster has no kubeconfig with secret access.**

According to CLAUDE.md, ord-devimprint only provides:
- Read-only kubectl-proxy at `http://kubectl-proxy-ord-devimprint:8001`
- Proxy runs in `devpod-observer` namespace with **read-only RBAC**
- Explicitly **cannot create, delete, or modify resources** (including secrets)

## Evidence

```bash
# Attempt 1: Direct kubeconfig (doesn't exist)
$ kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory

# Attempt 2: Via proxy (Forbidden)
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Comparison with Other Clusters

Other clusters have direct kubeconfigs with elevated access:
- **ardenone-manager**: `/home/coding/.kube/ardenone-manager.kubeconfig` (cluster-admin)
- **rs-manager**: `/home/coding/.kube/rs-manager.kubeconfig` (cluster-admin)
- **iad-options**: `/home/coding/.kube/iad-options.kubeconfig` (cloudspace-admin)
- **iad-ci**: `/home/coding/.kube/iad-ci.kubeconfig` (cluster-admin)

**ord-devimprint**: No kubeconfig exists, only read-only proxy

## Resolution Required

To complete this task, one of the following must be provided:

1. **Create a kubeconfig with secret access** for ord-devimprint (similar to other clusters)
2. **Upgrade the devpod-observer ServiceAccount** to include secret get/list permissions
3. **Use an alternative cluster** that has the same secret replicated
4. **Use OpenBao API directly** if credentials are available (bypasses kubectl)

## Status

**BLOCKED** - Cannot proceed without infrastructure changes to enable secret access on ord-devimprint cluster.
