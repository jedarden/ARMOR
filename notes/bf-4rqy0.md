# Bead bf-4rqy0: Validate retrieved value is valid base64

## Task
Verify that the retrieved LITESTREAM_ACCESS_KEY_ID value is properly base64-encoded and non-empty.

## Infrastructure Blocker
**Cannot access secret for validation due to RBAC restrictions.**

### Access Attempts

1. **Attempted kubeconfig path** (bf-4743d):
   - Path: `/home/coding/.kube/ord-devimprint.kubeconfig`
   - Result: File does not exist
   - ord-devimprint uses kubectl-proxy over Tailscale, not kubeconfig

2. **Attempted proxy access** (bf-2pn4n, bf-2y15n):
   - Command: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint`
   - Result: **Forbidden**
   - Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

### Root Cause
The `devpod-observer` ServiceAccount has read-only RBAC that explicitly denies access to secrets. This is a security restriction that prevents validation of the secret's base64 encoding.

### Validation Commands Blocked
The following validation commands cannot execute due to the RBAC blocker:

```bash
# Capture the value - BLOCKED by RBAC
VALUE=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}')

# Check non-empty - CANNOT TEST (value not retrieved)
# Validate base64 characters - CANNOT TEST (value not retrieved)
# Attempt decode - CANNOT TEST (value not retrieved)
```

### Access Pattern
According to CLAUDE.md:
- ord-devimprint uses kubectl-proxy over Tailscale
- Proxy runs in `devpod-observer` namespace with read-only RBAC
- Access is **read-only** and does NOT include secret access
- No direct kubeconfig exists for ord-devimprint (only iad-acb and iad-ci available)

### Resolution Path
To complete this validation, one of the following would be needed:
1. Direct kubeconfig with elevated permissions to ord-devimprint cluster
2. RBAC modification to grant devpod-observer SA secret read access in devimprint namespace
3. Alternative validation method that doesn't require direct secret access

### Status
- **Prerequisites**: All child beads (bf-4743d, bf-2pn4n, bf-2y15n) are closed
- **Blocker**: RBAC denies secret access
- **Validation**: Cannot proceed without secret access

### Related Documentation
- Git commit 301bfcc1: "docs(bf-4rqy0): re-verify infrastructure blocker persists - RBAC denies secret access"
- Git commit 78b9efe8: "docs(bf-4rqy0): document infrastructure blocker - cannot access secret for validation"
- Git commit 8c9de496: "docs(bf-2y15n): document infrastructure blocker - ord-devimprint proxy denies secret access"

## Timestamp
2026-07-11
