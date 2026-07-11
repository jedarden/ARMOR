# ord-devimprint Kubeconfig Access Requirements

## Current Status

**Cluster**: ord-devimprint (Rackspace Spot cluster - ORD region)
**Server**: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`

### Previous Access
- ✅ **Previously had kubeconfig**: `~/.kube/ord-devimprint.kubeconfig`
- ❌ **Current status**: File no longer exists (likely removed after token expiry)
- 📋 **Previous issue**: Bead `armor-bik` documented expired JWT token (expired 2026-04-26)
- 🔄 **Resolution method**: Token was refreshed via Rackspace Spot dashboard

### Current Access
- **Read-only proxy**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount**: `system:serviceaccount:devpod-observer:devpod-observer`
- **Permissions**: 
  - ✅ Can list resources (pods, services, secrets, etc.)
  - ❌ Cannot get secret details (User cannot get resource "secrets")
  - ❌ Cannot create/modify/delete resources

### What We Need
To retrieve the `armor-writer` secret from the `devimprint` namespace, we need:
- **Kubeconfig path**: `~/.kube/ord-devimprint.kubeconfig`
- **Required permissions**:
  - `get` on `secrets` in namespace `devimprint`
  - (Optional) `create`/`update` permissions for managing ARMOR credentials

### Available Secrets in devimprint Namespace
```
NAME                    TYPE                             DATA   AGE
admin-oauth             Opaque                           3      62d
armor-credentials       Opaque                           7      80d
armor-readonly          Opaque                           2      80d
armor-writer            Opaque                           2      80d  ← TARGET
devimprint-b2-workers   Opaque                           5      66d
devimprint-cloudflare   Opaque                           8      80d
docker-hub-registry     kubernetes.io/dockerconfigjson   1      80d
github-oauth            Opaque                           2      31d
github-pat              Opaque                           1      80d
queue-api-auth          Opaque                           2      2d19h
```

## Solution: Refresh via Rackspace Spot Dashboard

Based on bead `armor-bik`, the ord-devimprint kubeconfig uses JWT tokens that expire and need to be refreshed via the Rackspace Spot dashboard.

### Steps to Obtain/Refresh Kubeconfig

1. **Log into Rackspace Spot dashboard**
   - Navigate to the Rackspace Spot console
   - Access the ORD region clusters

2. **Locate ord-devimprint cluster**
   - Find cluster with server: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
   - This is the ORD region devimprint cluster

3. **Generate/download kubeconfig**
   - Use Spot dashboard's kubeconfig download feature
   - Ensure the downloaded config has appropriate permissions (secret read access)

4. **Store the kubeconfig**
   - Save as `~/.kube/ord-devimprint.kubeconfig`
   - Set appropriate permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

5. **Verify access**
   ```bash
   # Test basic connectivity
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes
   
   # Test secret access (the primary goal)
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
   
   # Verify we can get secret details
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
   ```

## Alternative: Create ServiceAccount (If Spot Dashboard Unavailable)

If you have cluster-admin access to ord-devimprint, you can create a ServiceAccount:

1. Create ServiceAccount with secret read permissions
2. Extract the token from the associated Secret
3. Create kubeconfig file with the token
4. Test and verify access

## Related Beads
- **armor-bik** (closed): "Refresh ord-devimprint kubeconfig token" - Previously resolved via Spot dashboard
- **armor-l64** (closed): ARMOR crash investigation that required cluster access
- **bf-4qq1** (open): "Bump ord-devimprint ARMOR to a fixed version" - May need this access

## Related Clusters Access Pattern
Based on CLAUDE.md and previous beads, Rackspace Spot clusters use these patterns:
- **rs-manager**: Direct kubeconfig with cluster-admin
- **iad-options**: OIDC token via Spot UI (expires every ~3 days)
- **iad-ci**: ServiceAccount with cluster-admin
- **ord-devimprint**: JWT token via Spot dashboard (expires periodically)

## Current Blocker
The ord-devimprint kubeconfig file no longer exists at `~/.kube/ord-devimprint.kubeconfig`. Based on previous experience with bead `armor-bik`, this needs to be refreshed via the Rackspace Spot dashboard.

## Next Steps
1. **Immediate**: Access Rackspace Spot dashboard and download new kubeconfig for ord-devimprint
2. **Store**: Save as `~/.kube/ord-devimprint.kubeconfig`
3. **Verify**: Test access to secrets in devimprint namespace
4. **Document**: Note token expiry timeline for future refresh

## References
- CLAUDE.md: Kubernetes Access section for ord-devimprint
- declarative-config: `k8s/ord-devimprint/devpod-observer/rbac.yml`
- Bead `armor-bik`: Previous ord-devimprint kubeconfig refresh
