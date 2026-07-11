# Bead bf-2y15n: Retrieve base64-encoded value from secret

## Task Blocked - RBAC Restriction

Attempted to retrieve the LITESTREAM_ACCESS_KEY_ID field from the armor-writer secret in the ord-devimprint cluster.

### Command Attempted

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

### Error Encountered

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### Root Cause

The ord-devimprint cluster is accessed via a read-only kubectl-proxy (running in the `devpod-observer` namespace with a restricted ServiceAccount). This proxy explicitly denies access to secrets, which is consistent with the cluster's security model.

### Available Access Options

Per CLAUDE.md documentation for ord-devimprint:
- Access is **read-only** via proxy
- Cannot create, delete, or modify resources
- The observer ServiceAccount has no secret access

### Resolution Required

To complete this bead, one of the following would be needed:
1. A direct kubeconfig with elevated privileges (similar to iad-options or iad-ci clusters)
2. An alternative method to retrieve the secret value
3. Secret access granted to the observer ServiceAccount (unlikely given security posture)

### Status

**BLOCKED** - Cannot complete task with current access level. Bead should remain open for retry with appropriate credentials.
