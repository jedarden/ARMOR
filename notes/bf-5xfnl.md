# Bead bf-5xfnl: RBAC Blocker Preventing Secret Access

## Task
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in ord-devimprint cluster.

## Blocker Identified
RBAC on the ord-devimprint kubectl-proxy prevents secret access:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Root Cause
- The ord-devimprint kubectl-proxy (`http://kubectl-proxy-ord-devimprint:8001`) runs with 
  read-only ServiceAccount `devpod-observer` in the `devpod-observer` namespace
- This SA explicitly lacks secrets read permissions (stricter RBAC than other clusters)
- No read/write kubeconfig is documented for ord-devimprint in CLAUDE.md

## Options to Unblock
1. **ExternalSecret direct access**: Access the underlying secret source (e.g., OpenBao, 1Password) 
   instead of kubectl
2. **Kubeconfig with secrets access**: Obtain or create a kubeconfig with secret read permissions 
   for ord-devimprint
3. **Alternative cluster**: If the same secret exists in another accessible cluster, retrieve it 
   from there

## Status
**BLOCKED** - Cannot retrieve LITESTREAM_ACCESS_KEY_ID without elevated permissions or alternative 
access method. The infrastructure blocker must be resolved before this bead can proceed.
