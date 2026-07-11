# Task bf-2y15n: Infrastructure Blocker - Cannot Retrieve Secret from ord-devimprint

## Attempt Summary

Task: Retrieve `LITESTREAM_ACCESS_KEY_ID` field from `armor-writer` secret in `devimprint` namespace on ord-devimprint cluster.

## Blocker Details

### 1. Kubeconfig Path Does Not Exist
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig
# Error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory
```

### 2. Read-Only Proxy Denies Secret Access
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Infrastructure Context

Per `/home/coding/CLAUDE.md`, the ord-devimprint cluster access pattern:

> ### ord-devimprint
> ```bash
> kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n <namespace>
> ```
> - Proxy runs in `devpod-observer` namespace with read-only RBAC
> - Access is **read-only** — cannot create, delete, or modify resources
> - Exposed via Tailscale operator (no Traefik on this cluster)

**Key difference:** Unlike `iad-options`, `ardenone-manager`, `rs-manager`, and `iad-ci`, ord-devimprint has **no read/write kubeconfig** available. Only the read-only proxy exists, and it explicitly denies secret access.

## Verification History

This blocker has been verified multiple times (see git log):
- `50ac2019` - re-verify infrastructure blocker persists - proxy RBAC denies secret access
- `1999b6a2` - document verification attempt - infrastructure blocker persists  
- `54ce66ec` - re-verify infrastructure blocker persists
- `56cb0f60` - re-verify infrastructure blocker persists - proxy RBAC denies secret access
- `a73d3595` - document RBAC blocker - cannot retrieve secret value
- `6265a38f` - document infrastructure blocker - kubeconfig missing, proxy denies secrets
- `97f8738e` - verify infrastructure blocker persists - kubeconfig missing, proxy denies secrets

## Resolution Required

To complete this task, one of the following is needed:

1. **Direct kubeconfig** for ord-devimprint with secret access (similar to `/home/coding/.kube/iad-options.kubeconfig`)
2. **RBAC modification** to allow the `devpod-observer` ServiceAccount to read secrets in the `devimprint` namespace
3. **Alternative access method** - retrieve the secret value from a cluster with appropriate access (e.g., from the ExternalSecret source in OpenBao)

## Acceptance Criteria Status

- [ ] Successfully executed kubectl command to retrieve the field - **BLOCKED by RBAC**
- [ ] Command returned a value (not empty string) - **Cannot test due to RBAC**
- [ ] Value was captured (no command errors) - **Cannot test due to RBAC**

## Recommendation

This task should remain blocked until infrastructure access is provisioned. The ord-devimprint cluster needs either:
- A read/write kubeconfig with secret access, OR
- Updated RBAC permissions for the proxy ServiceAccount

As documented in workspace learning (bead bf-520v), similar RBAC blockers have been worked around by using cached values or alternative access methods.
