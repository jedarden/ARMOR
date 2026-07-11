# bf-6bs48: RBAC Blocker - Secret Access Forbidden on ord-devimprint

## Task Attempted
Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID from armor-writer secret in devimprint namespace.

## Result
**RBAC BLOCKER - Access Forbidden**

### Error Details
```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

### Root Cause
The kubectl-proxy for ord-devimprint runs with read-only RBAC that **explicitly blocks secret access**. The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in the `devimprint` namespace.

### Command Attempted
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

### Access Pattern
This matches the pattern seen on other clusters:
- `iad-options` observer explicitly denies secret access
- `ord-devimprint` observer also denies secret access (confirmed)
- Read-only proxies cannot access secrets, even with get operations

### Next Steps Required
To retrieve secret values from ord-devimprint, need one of:
1. Direct kubeconfig with elevated privileges (like iad-ci, iad-options read/write kubeconfig)
2. Updated RBAC rules to grant secret read access to devpod-observer SA
3. Alternative secret retrieval method (ExternalSecrets, direct OpenBao access, etc.)

## Documentation
This finding documents the RBAC blocker that prevents secret access on ord-devimprint via the standard kubectl-proxy pattern used across clusters.
