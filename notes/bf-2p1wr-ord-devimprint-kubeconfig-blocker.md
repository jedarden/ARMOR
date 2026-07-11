# ord-devimprint Kubeconfig Blocker Analysis

## Task Objective
Obtain a kubeconfig file with write access to the ord-devimprint cluster to retrieve the `armor-writer` secret (containing Litestream S3 credentials).

## Current State

### Read-only Access (Available)
- **Proxy URL**: `kubectl-proxy-ord-devimprint:8001`
- **Access Level**: Read-only via `devpod-observer` ServiceAccount
- **Capabilities**:
  - Can list pods: ✅ Working
  - Can list secret names: ✅ Working
  - Can get secret data: ❌ **Forbidden**

```bash
# Can list secret names
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# Returns: armor-credentials, armor-readonly, armor-writer, etc.

# Cannot get secret data
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error: Forbidden - User "system:serviceaccount:devpod-observer:devpod-observer" 
# cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### Required Access (Missing)
- **Kubeconfig**: `~/.kube/ord-devimprint.kubeconfig`
- **Access Level**: Write/Secret read access
- **Purpose**: Retrieve `armor-writer` secret containing:
  - `LITESTREAM_ACCESS_KEY_ID`
  - `LITESTREAM_SECRET_ACCESS_KEY`

## Cluster Information
- **Cluster**: ord-devimprint
- **Provider**: Rackspace Spot
- **Region**: ord (Chicago)
- **Nodes**: 6 worker nodes (v1.33.0)
- **Node Naming Pattern**: `prod-instance-{id}` (consistent with Rackspace Spot)

## Blocker Root Cause

### Access Requirements
The ord-devimprint cluster requires a kubeconfig generated through the **Rackspace Spot console UI**. This is documented for similar clusters:

**Example: iad-options cluster**
```bash
# Read/write (cloudspace-admin OIDC token, expires every ~3 days — regenerate from Spot UI)
kubectl --kubeconfig=/home/coding/.kube/iad-options.kubeconfig get pods -n <namespace>
```

### Why This Is a Blocker
1. **No Interactive Browser Access**: This environment has no direct browser access to Rackspace Spot console
2. **OIDC Token Required**: Rackspace Spot clusters use OIDC authentication that requires:
   - Interactive login to Spot UI
   - Token generation through the console
   - Token expires every ~3 days (similar to iad-options)
3. **No Alternative Access Paths**: Unlike other clusters with multiple access methods, ord-devimprint only has the read-only proxy

## Resolution Path

### Required Actions (Human Intervention)
1. **Access Rackspace Spot Console**
   - Navigate to: https://console.rackspace.com/
   - Authenticate with Rackspace credentials
   - Locate the ord-devimprint cloudspace/cluster

2. **Generate Kubeconfig**
   - Find the "Download Kubeconfig" or "Generate Kubeconfig" option
   - Select "cloudspace-admin" or equivalent role with secret read access
   - Download the kubeconfig file

3. **Store Kubeconfig Securely**
   ```bash
   # Save to standard location
   cp ~/Downloads/ord-devimprint.kubeconfig ~/.kube/ord-devimprint.kubeconfig
   chmod 600 ~/.kube/ord-devimprint.kubeconfig
   ```

4. **Verify Access**
   ```bash
   # Test secret access
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
   # Should return secret data (not Forbidden)
   ```

### Alternative Path: Cluster Administrator
If Rackspace Spot console access is not available:
- Contact the cluster administrator
- Request a kubeconfig with secret read access for the `devimprint` namespace
- Specify ServiceAccount with `role` allowing `secrets/*` reads

## Related Clusters
Other clusters with similar access patterns:
- **iad-options**: Rackspace Spot, requires Spot UI for kubeconfig
- **rs-manager**: Rackspace Spot manager cluster
- **iad-ci**: Rackspace Spot CI cluster (has kubeconfig at `~/.kube/iad-ci.kubeconfig`)

## Impact
This blocker prevents:
1. Retrieving Litestream S3 credentials from `armor-writer` secret
2. Restoring queue-api database from S3 backup
3. Completing dependent beads in the ARMOR recovery workflow

## Verification Steps (Once Kubeconfig Is Obtained)
```bash
# 1. Verify kubeconfig exists
ls -la ~/.kube/ord-devimprint.kubeconfig

# 2. Test basic connectivity
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# 3. Verify secret access (critical requirement)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

# 4. Verify cluster access
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get pods -n devimprint
```

## Documentation References
- Git history shows this blocker has been verified and re-verified (0756ed4c, ff60f0ee, ef105dc4, etc.)
- All previous investigations conclude: "requires Rackspace Spot console access"
- This is a **persistent blocker** that requires external action to resolve

## Conclusion
This task **cannot be completed programmatically** from this environment. It requires:
- Browser access to Rackspace Spot console, OR
- Cluster administrator to provide kubeconfig

The bead should remain **open** until a human with appropriate access can provide the kubeconfig file.
