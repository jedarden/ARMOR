# RBAC Blocker: Cannot Retrieve Secret via Read-Only Proxy

## Task
Retrieve base64-encoded SECRET_ACCESS_KEY from Kubernetes secret `armor-writer` in `devimprint` namespace.

## Attempted Commands

```bash
# Via direct kubeconfig (file doesn't exist)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint

# Via kubectl-proxy (read-only access)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
```

## Result
**Error from server (Forbidden):** secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"

## Root Cause
The `ord-devimprint` cluster has **read-only RBAC** configured for the `devpod-observer` service account used by kubectl-proxy. This proxy explicitly denies access to secrets for security reasons.

## Resolution Options
1. Use cluster-admin kubeconfig if available for `ord-devimprint`
2. Have cluster admin create a limited ServiceAccount with secret read access
3. Use alternative secret retrieval method (e.g., ExternalSecret, vault, etc.)

## Status
**BLOCKED** - Cannot complete without elevated credentials or RBAC changes.
