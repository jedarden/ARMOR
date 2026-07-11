# bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Investigation Summary

### Finding
There was previously a kubeconfig file at `~/.kube/ord-devimprint.kubeconfig` that provided write access to the ord-devimprint cluster. However, this file no longer exists (likely removed after token expiry).

### Evidence
- **Bead armor-bik** (closed): Documented expired JWT token (expired 2026-04-26 19:10 EDT)
- **Resolution**: Token was successfully refreshed via Rackspace Spot dashboard
- **Current status**: File no longer exists at expected location

### Current Access Limitations
- **Read-only proxy**: Available at `kubectl-proxy-ord-devimprint:8001`
- **Permission gap**: Can list secrets but cannot get secret details
- **Required**: Need kubeconfig with secret read permissions to retrieve `armor-writer` secret

## Required Action

This task requires **human intervention** to access the Rackspace Spot dashboard:

### Step-by-Step Instructions

1. **Access Rackspace Spot Dashboard**
   ```
   URL: https://spot.rackspace.com (or organization-specific dashboard)
   Region: ORD (Chicago)
   Cluster: ord-devimprint (hcp-5f30c973-cde7-42d9-8c7b-5d0573821330)
   ```

2. **Download Kubeconfig**
   - Navigate to cluster details
   - Find "Download Kubeconfig" or similar option
   - Ensure the config includes permissions for:
     - Reading secrets in `devimprint` namespace
     - Basic cluster operations (get, list, watch)

3. **Store Kubeconfig**
   ```bash
   # Save the downloaded file
   mv ~/Downloads/kubeconfig-ord-devimprint.yaml ~/.kube/ord-devimprint.kubeconfig
   
   # Set secure permissions
   chmod 600 ~/.kube/ord-devimprint.kubeconfig
   ```

4. **Verify Access**
   ```bash
   # Test basic connectivity
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes
   
   # Test secret access (critical requirement)
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
   
   # Verify we can retrieve secret data
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o json
   ```

5. **Document Token Expiry**
   - Note when the token expires (Rackspace Spot JWT tokens typically have limited lifespan)
   - Set reminder to refresh before expiry

## Why This Requires Human Action

1. **Authentication**: Rackspace Spot dashboard requires authenticated access with organization credentials
2. **Security**: Kubeconfig files contain sensitive authentication tokens that shouldn't be automated
3. **Dashboard UI**: The process involves navigating a web interface and downloading files
4. **Token management**: JWT tokens need to be periodically refreshed (every ~30-90 days)

## What Happens Next

Once the kubeconfig is obtained and verified:
1. Store securely at `~/.kube/ord-devimprint.kubeconfig`
2. Verify access meets acceptance criteria (can get secrets)
3. Update bead `bf-2p1wr` with success confirmation
4. Proceed with dependent beads that require ord-devimprint access

## Blocked Work

Without this kubeconfig, the following work is blocked:
- Retrieving `armor-writer` secret credentials from devimprint namespace
- Any ARMOR operations that require write access to ord-devimprint
- Debugging or configuration changes requiring secret access

## Success Criteria

- ✅ Kubeconfig exists at `~/.kube/ord-devimprint.kubeconfig`
- ✅ Can successfully run: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`
- ✅ Can retrieve `armor-writer` secret: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o json`
