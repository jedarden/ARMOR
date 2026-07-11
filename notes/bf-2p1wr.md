# Task: Obtain ord-devimprint kubeconfig with write access

## Current Situation

The ord-devimprint cluster is currently accessible only via a read-only kubectl proxy:

- **Proxy endpoint**: `kubectl-proxy-ord-devimprint:8001`
- **Access level**: Read-only (via devpod-observer service account)
- **Secrets access**: DENIED (`kubectl auth can-i get secrets -n devimprint` returns `no`)

## What's Needed

To retrieve the `armor-writer` secret, we need a kubeconfig file with:
- Permissions to read secrets in the `devimprint` namespace
- Stored securely at `~/.kube/ord-devimprint.kubeconfig`

## Existing Kubeconfig Pattern

Other clusters have direct kubeconfigs:
- `~/.kube/iad-ci.kubeconfig` - Full cluster-admin access
- `~/.kube/iad-acb.kubeconfig` - Another cluster config

## Next Steps

**This requires coordination with the cluster administrator.**

The kubeconfig must be created by whoever administers the ord-devimprint cluster, as I cannot:
1. Create credentials with elevated permissions
2. Modify RBAC on the cluster (no write access)
3. Generate new tokens or certificates

## Verification Steps (once kubeconfig is obtained)

```bash
# Test basic connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Test secret access (acceptance criteria)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Test the specific secret we need
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Cluster Notes

From CLAUDE.md:
- ord-devimprint uses Tailscale operator (not Traefik like other clusters)
- Proxy hostname: `kubectl-proxy-ord-devimprint`
- No existing write-access kubeconfig on file

## Status

**BLOCKED** - Waiting for cluster administrator to provide kubeconfig with write access.

This task cannot be completed without external coordination. The cluster administrator needs to:
1. Create a ServiceAccount with appropriate RBAC to read secrets in the `devimprint` namespace
2. Generate a kubeconfig or token for that ServiceAccount
3. Provide it securely for storage at `~/.kube/ord-devimprint.kubeconfig`
