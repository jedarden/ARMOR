# Obtain ord-devimprint kubeconfig with write access

## Analysis

The ord-devimprint cluster is a **Rackspace Spot cluster**:
- Server: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- Managed via declarative-config: `k8s/ord-devimprint/`
- Uses Tailscale operator for kubectl-proxy exposure (read-only)

## Current Access

- **Read-only proxy**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount**: `devpod-observer` with read-only RBAC
- **Secrets access**: DENIED

## Required Action

Follow the pattern used for iad-options (another Rackspace Spot cluster):

> **cloudspace-admin OIDC token** — regenerate from Spot UI (expires every ~3 days)

### Steps to Obtain Kubeconfig

1. Log in to **Rackspace Spot console**
2. Navigate to the **ord-devimprint cloudspace**
3. Download or generate the **cloudspace-admin kubeconfig**
4. Save to: `~/.kube/ord-devimprint.kubeconfig`
5. Set proper permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Verification Commands

```bash
# Test basic connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Test secret access (acceptance criteria)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Test the specific secret we need
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Pattern Reference

Similar Rackspace Spot cluster setups:
- **iad-options**: Read/write via `~/.kube/iad-options.kubeconfig` (cloudspace-admin OIDC token from Spot UI)
- **iad-ci**: Full cluster-admin via `~/.kube/iad-ci.kubeconfig`
- **rs-manager**: Full cluster-admin via `~/.kube/rs-manager.kubeconfig`

## Next Steps

**Waiting for user to:**
1. Obtain kubeconfig from Rackspace Spot UI
2. Save to `~/.kube/ord-devimprint.kubeconfig`
3. Confirm access is working

Once kubeconfig is obtained and verified, update this bead and proceed to retrieve the `armor-writer` secret.
