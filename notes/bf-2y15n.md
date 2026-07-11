# Task bf-2y15n: Infrastructure Blocker

## Finding
The task to retrieve `LITESTREAM_ACCESS_KEY_ID` from the `armor-writer` secret is blocked by infrastructure limitations.

## Verified Constraints

### 1. Kubectl Proxy Access Denied
Attempted via kubectl-proxy on ord-devimprint:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

**Result:** Forbidden - User `system:serviceaccount:devpod-observer:devpod-observer` cannot get resource `secrets`

### 2. No Direct Kubeconfig
Expected kubeconfig path does not exist:
```bash
ls -la /home/coding/.kube/ord-devimprint.kubeconfig
# No such file or directory
```

### 3. Prerequisite Bead Status
- bf-4743d: Verify kubeconfig exists - marked closed, but kubeconfig doesn't exist
- bf-2pn4n: Test kubectl access - marked closed, but references non-existent kubeconfig

This inconsistency suggests the prerequisite beads may have been closed prematurely or infrastructure has changed.

## Infrastructure Context
Per `/home/coding/CLAUDE.md`, ord-devimprint cluster access is documented as:
- Read-only proxy via kubectl-proxy
- No direct kubeconfig mentioned for this cluster
- RBAC appears to explicitly deny secret access (similar to iad-options observer)

## Next Steps
This task cannot be completed without either:
1. A kubeconfig with elevated permissions for ord-devimprint
2. RBAC changes to allow secret access via the proxy
3. An alternative method to obtain the secret value

This is a documented infrastructure blocker (see commits d55fc3ea, 25c263f1, 329097c4).
