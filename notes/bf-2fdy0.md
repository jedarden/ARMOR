# Task bf-2fdy0: Cannot Retrieve SECRET_ACCESS_KEY

## Issue

Unable to retrieve `LITESTREAM_SECRET_ACCESS_KEY` from the `armor-writer` secret in the `devimprint` namespace.

## Root Cause

The `ord-devimprint` cluster has two access methods:

1. **Proxy (kubectl-proxy-ord-devimprint:8001)**: Read-only access via ServiceAccount `devpod-observer:devpod-observer`
   - **Blocks secret access** - returns Forbidden error

2. **Direct kubeconfig**: Does not exist at `/home/coding/.kube/ord-devimprint.kubeconfig`
   - Only kubeconfigs available: `iad-acb.kubeconfig`, `iad-ci.kubeconfig`

## Error Output

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Resolution Path

This task requires either:
1. A direct kubeconfig for `ord-devimprint` with secret-read permissions
2. Elevated permissions on the proxy ServiceAccount
3. Alternative access method (e.g., cached secret from previous retrieval)

## Status

**BLOCKED** - Cannot complete without additional cluster credentials or RBAC changes.
