# Task bf-3llc7: Retrieve base64-encoded SECRET_ACCESS_KEY from armor-writer secret

## Finding

Cannot retrieve the SECRET_ACCESS_KEY due to access limitations on the ord-devimprint cluster.

## Access Methods Attempted

1. **Direct kubeconfig** (as specified in task command):
   - File `/home/coding/.kube/ord-devimprint.kubeconfig` does not exist
   - No ord-devimprint kubeconfig is available on this system

2. **kubectl-proxy over Tailscale**:
   - Command: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint`
   - Result: Forbidden by RBAC
   - Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

## Root Cause

The ord-devimprint cluster is accessed via a read-only proxy (`kubectl-proxy-ord-devimprint:8001`) running in the `devpod-observer` namespace. The ServiceAccount has read-only RBAC that **explicitly denies access to secrets**, similar to the iad-options cluster configuration.

Unlike other clusters (ardenone-manager, rs-manager) which have both a read-only proxy AND a direct kubeconfig with cluster-admin access, ord-devimprint only has the read-only proxy option.

## Available Kubeconfigs

Only the following kubeconfigs exist on this system:
- `/home/coding/.kube/iad-ci.kubeconfig` (cluster-admin for iad-ci cluster)
- `/home/coding/.kube/iad-acb.kubeconfig`

## Resolution

To retrieve the secret, one of the following would be needed:
1. A direct kubeconfig with secret access permissions for ord-devimprint cluster
2. RBAC modification to allow devpod-observer SA to read secrets (not recommended for security)
3. Direct cluster access to retrieve the secret through alternative means

## Related Documentation

This limitation is already documented in:
- CLAUDE.md under "ord-devimprint" cluster access section
- Workspace learnings bead bf-3llc7: "ord-devimprint secret access limitation"
