# ord-devimprint Kubeconfig Requirements

## Current Status

### Read-Only Access (Working)
- **Endpoint:** `kubectl-proxy-ord-devimprint:8001`
- **Access Level:** Read-only via ServiceAccount `devpod-observer`
- **Capabilities:**
  - Can list resources including secrets
  - Cannot read secret contents (Forbidden)
  - Cannot create/modify/delete resources

### Verification
```bash
# List secrets (works)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint

# Read secret content (forbidden)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Write Access Requirements

### Cluster Details
- **Name:** ord-devimprint
- **Provider:** Rackspace Spot
- **Region:** ord (Chicago)
- **Managed by:** rs-manager ArgoCD

### ArgoCD Registration
The cluster is registered in rs-manager ArgoCD as secret `cluster-ord-devimprint` in the `argocd` namespace, but this secret contains only connection credentials and is not directly usable as a kubeconfig.

### How to Obtain Kubeconfig

#### Option 1: Rackspace Spot Console (Recommended)
1. Log in to Rackspace Spot console at https://argocd-rs-manager.tail1b1987.ts.net:8080
2. Navigate to the ord-devimprint cluster
3. Download or generate the kubeconfig with appropriate permissions
4. Store securely at `~/.kube/ord-devimprint.kubeconfig`

#### Option 2: ServiceAccount Token
Create a ServiceAccount with cluster-admin or limited namespace access in the devimprint namespace, then generate a kubeconfig from its token.

## Required Permissions
The kubeconfig must have at minimum:
- **Namespace:** devimprint
- **Resource:** secrets
- **Verb:** get

## Acceptance Criteria
- [ ] Kubeconfig file obtained and stored at `~/.kube/ord-devimprint.kubeconfig`
- [ ] Can successfully read secrets: `kubectl get secret armor-writer -n devimprint`
- [ ] Verify before proceeding to next child bead

## Notes
- This is a **blocker** for retrieving the armor-writer secret
- The cluster administrator (user with Rackspace Spot console access) needs to perform this step
- Similar to how `iad-options.kubeconfig` is obtained (cloudspace-admin OIDC token from Spot UI)

## References
- CLAUDE.md documents ord-devimprint as having only read-only proxy access
- Other Rackspace clusters (iad-ci, iad-options, iad-kalshi) have write-capable kubeconfigs
