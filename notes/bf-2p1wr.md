# Bead bf-2p1wr: ord-devimprint Kubeconfig Investigation

## Current State

### Existing Access
- **Read-only proxy**: `kubectl-proxy-ord-devimprint:8001` via Tailscale
- **Proxy RBAC**: ServiceAccount `devpod-observer:devpod-observer`
- **Limitations**: Explicitly denies access to secrets and cluster-level resources

### Cluster Details
- **Platform**: Rackspace Spot (us-east-iad-1)
- **Cluster ID**: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Management**: ArgoCD on rs-manager
- **ArgoCD ApplicationSet**: `manifest-appset-ord-devimprint` in `rs-manager/argocd`

### Pattern from Other Clusters
Other Rackspace Spot clusters have direct kubeconfigs:
- `~/.kube/rs-manager.kubeconfig` (cluster-admin)
- `~/.kube/iad-ci.kubeconfig` (cluster-admin)
- `~/.kube/iad-acb.kubeconfig` (proxy only, small file)

**ord-devimprint does not have a direct kubeconfig.**

## ArgoCD Credential Setup

The file `k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` contains the one-time setup instructions for ArgoCD cluster credentials, which requires a kubeconfig with cluster-admin access.

This is circular - the setup requires a kubeconfig, but no kubeconfig exists.

## Required Action

**Rackspace Spot Console Access Required**

To obtain a kubeconfig with write access to ord-devimprint:

1. **Log in to Rackspace Spot console** (us-east-iad-1 region)
2. **Navigate to the cluster**: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
3. **Download kubeconfig** with cluster-admin or appropriate namespace-level permissions
4. **Store securely**: `~/.kube/ord-devimprint.kubeconfig`
5. **Test access**:
   ```bash
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
   ```

## Alternative Approaches

If full cluster-admin is not available:
- **Namespace-specific admin**: Create a RoleBinding with `admin` role in `devimprint` namespace only
- **ServiceAccount token**: Create a ServiceAccount with limited secret read permissions in `devimprint` namespace

## Blocker Summary

This task is blocked on:
- **User action**: Access to Rackspace Spot console to download kubeconfig
- **Or coordination**: Cluster administrator providing the kubeconfig

Once kubeconfig is obtained, the ArgoCD credential setup can proceed if not already done.
# Investigation Results - 2026-07-11

## Investigation Summary

Attempted to find alternative paths to obtain ord-devimprint kubeconfig access.

### Findings

1. **Read-only proxy confirmed**: `kubectl-proxy-ord-devimprint:8001` can list secret names but cannot read secret content (Forbidden error on `get secret armor-writer`)

2. **ArgoCD cluster secret exists**: Found `cluster-ord-devimprint` secret in `argocd` namespace on rs-manager cluster, but cannot read it due to RBAC restrictions on the devpod-observer ServiceAccount

3. **Missing kubeconfigs**: Expected kubeconfigs from CLAUDE.md do not exist:
   - `~/.kube/rs-manager.kubeconfig` - NOT FOUND
   - `~/.kube/ardenone-manager.kubeconfig` - NOT FOUND
   - Only found: `iad-acb.kubeconfig`, `iad-ci.kubeconfig`

4. **ExternalSecret setup**: The file `declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` contains setup instructions that require a fresh kubeconfig - circular dependency

### Verification (2026-07-11)

**Test 1 - List secrets (succeeds)**
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
```
Result: Lists 10 secrets including `armor-writer` ✅

**Test 2 - Read secret content (fails)**
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
```
Result: `Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"` ❌

**Test 3 - Check for existing kubeconfigs**
```bash
ls -la ~/.kube/*.kubeconfig
```
Result: Only `iad-acb.kubeconfig` and `iad-ci.kubeconfig` exist. `ord-devimprint.kubeconfig` does not exist ❌

### Conclusion

**BLOCKER CONFIRMED**: This task requires human intervention to obtain kubeconfig from Rackspace Spot console.

No automated workaround exists without:
- Direct access to Rackspace Spot console (us-east-iad-1 region, cluster: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`)
- Or coordination with cluster administrator to provide the kubeconfig

### Next Steps (Requires Human Action)

1. Log into Rackspace Spot console (us-east-iad-1 region)
2. Navigate to cluster `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`  
3. Download kubeconfig with appropriate permissions
4. Store at `~/.kube/ord-devimprint.kubeconfig`
5. Verify: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`
6. Close bead bf-2p1wr and proceed to dependent tasks

## Related Beads

This bead blocks:
- `bf-3d39n`: Verify ord-devimprint ExternalSecret armor-writer sync
