# Task bf-2y15n - Cannot Complete: RBAC Blocker

## Task
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in ord-devimprint cluster

## Attempted Commands

1. Using kubeconfig (doesn't exist):
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
# Error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory
```

2. Using kubectl-proxy (correct method for ord-devimprint):
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
# Error from server (Forbidden): secrets "armor-writer" is forbidden: 
# User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
# in API group "" in the namespace "devimprint"
```

## Blocker
The ord-devimprint kubectl-proxy runs with read-only RBAC that **explicitly denies access to secrets**. The proxy's ServiceAccount (`devpod-observer:devpod-observer`) does not have secret access permissions.

## Resolution Required
This task requires either:
1. A read-write kubeconfig for ord-devimprint (if one exists)
2. RBAC changes to grant the devpod-observer ServiceAccount secret access
3. An alternative method to retrieve the secret value

## Status
**BLOCKED** - Cannot complete task without secret access permissions.

## Related Documentation
- Recent commits document this RBAC blocker: "docs(bf-2y15n): document RBAC blocker - no secret access on ord-devimprint"
- CLAUDE.md notes that ord-devimprint proxy "Access is **read-only** — cannot create, delete, or modify resources" but doesn't explicitly mention secret denial
