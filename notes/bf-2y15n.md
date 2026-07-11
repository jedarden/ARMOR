# Bead bf-2y15n: Retrieve base64-encoded value from secret

## Task Outcome: BLOCKED - Infrastructure Issue

Attempted to retrieve `LITESTREAM_ACCESS_KEY_ID` from the `armor-writer` secret in the `devimprint` namespace.

### Methods Attempted

1. **Direct kubeconfig** (from task description):
   - Path: `/home/coding/.kube/ord-devimprint.kubeconfig`
   - Result: File does not exist

2. **kubectl proxy** (per environment documentation):
   - Command: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'`
   - Result: `Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

### Root Cause

The `devpod-observer` ServiceAccount has RBAC rules that explicitly deny access to secrets. This is a documented infrastructure blocker on the `ord-devimprint` cluster.

### Recent Documentation

This blocker has been documented in recent commits:
- `3785fe4e` - docs(bf-2y15n): re-verify infrastructure blocker persists - proxy RBAC denies secret access
- `2fac5064` - docs(bf-2y15n): verify infrastructure blocker persists - RBAC denies secret access
- `329097c4` - docs(bf-2y15n): document infrastructure blocker - ord-devimprint proxy denies secret access

### Resolution Path

This task requires one of the following:
1. RBAC modification on the `devpod-observer` ServiceAccount to grant secret read access
2. Use of a different ServiceAccount with appropriate secret access permissions
3. Direct cluster access with credentials that can read secrets
4. Alternative approach that doesn't require kubectl secret access

### Notes

- The task prerequisites referenced child beads `bf-4743d` and `bf-2pn4n` - these may need to be completed to enable proper access
- This is a pure infrastructure issue, not a code or implementation issue
- The command syntax is correct; the blocker is purely permissions-based
