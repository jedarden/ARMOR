# BF-4RQY0 - RBAC Blocker Verification

**Date:** 2026-07-12
**Status:** ❌ CANNOT COMPLETE - Infrastructure Blocker

## Blocker Details

The `devpod-observer` ServiceAccount used by the kubectl-proxy on `ord-devimprint` has **read-only RBAC that explicitly denies secret access**:

```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
Error from server (Forbidden): User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## Acceptance Criteria Status

All criteria **❌ FAIL** - no value can be retrieved to validate:

| Criterion | Status |
|-----------|--------|
| Retrieved value is not empty | ❌ Cannot retrieve |
| Value contains valid base64 characters | ❌ No value to validate |
| Value length is reasonable | ❌ No value to measure |
| Can be decoded without errors | ❌ No value to decode |

## Available Access Methods

### ❌ kubectl-proxy (read-only)
- Server: `http://kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `devpod-observer` in `devpod-observer` namespace
- Permissions: **Cannot read secrets** (confirmed by Forbidden error)

### ❌ Admin kubeconfig
- File: `/home/coding/.kube/ord-devimprint.kubeconfig`
- Status: **Does not exist**

### ✅ Available kubeconfigs (different clusters)
- `iad-ci.kubeconfig` - different cluster
- `rs-manager.kubeconfig` - different cluster
- `ardenone-manager.kubeconfig` - different cluster
- `iad-options-observer.kubeconfig` - different cluster

## Resolution Path

To complete this task, one of the following is needed:

1. **Provision `ord-devimprint.kubeconfig`** with secret-level read access
2. **Modify RBAC** to grant `devpod-observer` SA secret read permissions in `devimprint` namespace
3. **Provide alternative validation method** that doesn't require direct secret access

## Previous Attempts

- 2026-07-11 23:57 - Documented RBAC blocker (commit 3c50a542)
- 2026-07-10 - Documented RBAC blocker preventing base64 validation (commit ca35b8d0)

## Conclusion

The bead **remains in_progress** awaiting infrastructure changes to enable secret access.
