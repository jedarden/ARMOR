# bf-6bs48: RBAC Blocker Verification

## Date: 2026-07-11

## Task
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in devimprint namespace.

## Blocker Status: CONFIRMED

### Issue
The kubectl-proxy for ord-devimprint has read-only RBAC that permits:
- ✅ LIST secrets (enumerate secret names)
- ❌ GET secrets (read secret data)

### Verification
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

**Error:**
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot 
get resource "secrets" in API group "" in the namespace "devimprint"
```

### Authentication Check
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i get secrets -n devimprint
# Output: no
```

### Available Access
- **ord-devimprint**: Read-only proxy only (no kubeconfig with secret access)
- **Kubeconfigs available**: Only `iad-acb.kubeconfig` and `iad-ci.kubeconfig` (wrong clusters)

### Related Beads
- `bf-enpyd` (parent): Verified LIST access only, not GET access
- `bf-2778z`: Documents missing kubeconfig for ord-devimprint
- `bf-112tt`: BLOCKED for LITESTREAM_SECRET_ACCESS_KEY (same issue)
- `bf-2p1wr`: Prerequisite bead for obtaining write-access kubeconfig (OPEN)

## Resolution Required
This bead cannot be completed until one of the following occurs:
1. Prerequisite bead `bf-2p1wr` provides a kubeconfig with secret read access
2. Cluster administrator provides secret values directly
3. RBAC permissions are updated to allow secret read access

## Status
**BLOCKED** - Task cannot be completed with current access permissions.
