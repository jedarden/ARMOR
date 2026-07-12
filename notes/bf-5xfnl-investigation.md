# Investigation Summary: bf-5xfnl - Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID

## Date: 2026-07-11

## Finding
The task is **blocked by RBAC** on the ord-devimprint cluster. The read-only kubectl-proxy explicitly denies access to secrets.

## What I Discovered

### 1. Correct Secret Field Mapping
The task description refers to `LITESTREAM_ACCESS_KEY_ID`, but the actual field in the `armor-writer` secret is `auth-access-key`. The deployment files map them like this:

```yaml
- name: LITESTREAM_ACCESS_KEY_ID
  valueFrom:
    secretKeyRef:
      name: armor-writer
      key: auth-access-key
```

### 2. Secret Source
The `armor-writer` secret is an ExternalSecret synced from OpenBao on rs-manager:
- OpenBao path: `rs-manager/ord-devimprint/armor-writer`
- Fields: `auth-access-key`, `auth-secret-key`
- Refresh interval: 1 hour

### 3. Access Constraint
ord-devimprint cluster access:
- **Only available:** Read-only kubectl-proxy via Tailscale
- **RBAC restriction:** Observer serviceaccount cannot read secrets
- **No read/write kubeconfig:** Does not exist on this system

### 4. Commands Attempted
```bash
# Both attempts failed with Forbidden error
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}'
```

Error:
```
Error from server (Forbidden): secrets "armor-writer" is forbidden
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Resolution Path
To complete this task, one of the following is needed:
1. A read/write kubeconfig for ord-devimprint cluster
2. RBAC modification to allow observer to read secrets (security risk)
3. Direct access to the OpenBao secret on rs-manager cluster

## Prerequisite Chain Issue
The prerequisite beads (bf-58r06, bf-2c1jp, bf-2txcw) were marked complete despite no functional secret access being available. This suggests:
- The kubeconfig used then has expired and wasn't regenerated
- Or the beads were marked complete without actual verification

## Files Updated
- `notes/bf-5xfnl-blocked.md` - Updated with field mapping discovery and investigation details
