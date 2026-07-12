# ord-devimprint Kubeconfig Access Investigation

## Summary
Task `bf-2p1wr` requires obtaining a kubeconfig with write access to the `ord-devimprint` cluster to retrieve the `armor-writer` secret.

## Findings

### Cluster Identity
- **Provider:** Rackspace Spot (OpenStack-based managed Kubernetes)
- **Cluster Server:** `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Region:** ORD (Chicago)
- **Age:** ~81 days (based on node ages)

### Current Access Status
- **Read-only proxy:** Available at `kubectl-proxy-ord-devimprint:8001` via Tailscale
- **Write access:** NOT available - requires kubeconfig from Spot UI
- **Existing kubeconfig:** None found at `~/.kube/ord-devimprint.kubeconfig`

### Related Cluster Access
The cluster is managed by `rs-manager` (also a Rackspace Spot cluster in IAD). However, the rs-manager kubeconfig (`~/.kube/rs-manager.kubeconfig`) is also missing and would need to be regenerated from Spot UI.

### ArgoCD Integration
The cluster is registered with ArgoCD via ExternalSecret `cluster-ord-devimprint` on rs-manager, which stores cluster credentials in OpenBao at path `secret/rs-manager/ord-devimprint/cluster`. However, this ExternalSecret requires an initial kubeconfig to set up the serviceaccount and token (see `/home/coding/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` lines 4-16).

## Required Action

To obtain write access to ord-devimprint, you need to:

1. **Access Rackspace Spot UI**
   - Navigate to the Spot console (URL needed - likely spot.rackspace.com or similar)
   - Login with appropriate Rackspace credentials

2. **Download kubeconfig for ord-devimprint cluster**
   - Find the cluster `ord-devimprint` (server: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`)
   - Download the kubeconfig file

3. **Store and verify the kubeconfig**
   ```bash
   # Store at standard location
   mv ~/Downloads/kubeconfig-ord-devimprint ~/.kube/ord-devimprint.kubeconfig
   chmod 600 ~/.kube/ord-devimprint.kubeconfig

   # Verify access
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
   ```

4. **(Optional but recommended) Create long-lived serviceaccount for automation**
   Once you have write access, consider setting up a dedicated serviceaccount with limited scope for automation purposes, similar to what's documented in the ExternalSecret.

## Why This Requires Manual Action
- Rackspace Spot kubeconfigs require authentication through their web UI
- The UI likely uses OIDC or session-based authentication that cannot be automated
- This is analogous to the `iad-options` cluster, which requires regenerating from Spot UI every ~3 days due to expiring OIDC tokens

## Next Steps
Once the kubeconfig is obtained and verified, this bead can be closed and the next child bead (retrieving armor-writer secret) can proceed.

## Blocker Status
This is currently blocking downstream work because we cannot retrieve secrets from the devimprint namespace without write access.
