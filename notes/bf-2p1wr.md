# Bead bf-2p1wr: ord-devimprint Kubeconfig Acquisition

## Current Status

**BLOCKED** - Requires manual intervention to obtain kubeconfig from Rackspace Spot console.

## What Was Attempted

1. **Created RBAC configuration** (previous session, commit `f8d6223`):
   - ServiceAccount: `secret-reader` in `devpod-observer` namespace
   - Role: `secret-reader-devimprint` with `get` and `list` permissions on secrets in `devimprint` namespace
   - Resources synced successfully (`secret-reader-token` secret exists as of 75 seconds ago)

2. **Verified extraction is blocked**:
   - Read-only proxy (`devpod-observer` SA) cannot read secret tokens
   - Cannot impersonate the `secret-reader` SA to test permissions
   - Even if we had the token, it would only give read access to `devimprint` secrets, not write access

3. **Identified the real solution**:
   - ord-devimprint is a **Rackspace Spot cluster**
   - Need admin kubeconfig from Spot UI (similar to iad-options pattern)
   - This provides full cluster-admin access via OIDC token

## Required Action

This kubeconfig **must be obtained manually** from the Rackspace Spot dashboard:

### Steps to Get the Kubeconfig

1. **Access Rackspace Spot console**
   - Navigate to: https://spot.rackspace.com (or the appropriate Spot dashboard URL)
   - Log in with your Rackspace credentials

2. **Locate the ord-devimprint cluster**
   - Cluster ID/Server: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
   - Look for cluster named "ord-devimprint" or matching that server URL

3. **Download admin kubeconfig**
   - Find the "Download Kubeconfig" or "Access" button
   - Select **cloudspace-admin** or **cluster-admin** credentials
   - This will use OIDC authentication (token expires ~3 days)

4. **Save to expected location**
   ```bash
   # Save the downloaded kubeconfig as:
   ~/.kube/ord-devimprint.kubeconfig
   chmod 600 ~/.kube/ord-devimprint.kubeconfig
   ```

5. **Verify access**
   ```bash
   # Test connectivity
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig version

   # Verify secret access
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
   ```

## Why This Approach

Based on the iad-options pattern documented in CLAUDE.md:
> "Read/write (cloudspace-admin OIDC token, expires every ~3 days — regenerate from Spot UI)"

Rackspace Spot clusters use OIDC tokens for admin access that must be regenerated from the Spot UI every ~3 days.

## Why the RBAC Approach Won't Work

The previous attempt to create a `secret-reader` service account has fundamental limitations:

1. **Read-only proxy can't extract tokens**: Even though `secret-reader-token` exists, the `devpod-observer` SA (running the proxy) doesn't have permission to read secret tokens
2. **Can't impersonate**: The proxy SA doesn't have impersonation rights to test or use the `secret-reader` SA
3. **Read-only anyway**: Even if we extracted the token, it would only grant **read access** to secrets in `devimprint` namespace, not write access
4. **Token rotation**: Service account tokens need to be managed and rotated, whereas the OIDC kubeconfig from Spot handles this automatically

## Why Spot Admin Kubeconfig Is Required

Looking at similar Rackspace Spot clusters:

**iad-options pattern** (from CLAUDE.md):
> "Read/write (cloudspace-admin OIDC token, expires every ~3 days — regenerate from Spot UI)"

Rackspace Spot clusters use OIDC tokens for admin access because:
- Provides full cluster-admin privileges (read/write all resources)
- Handles token refresh automatically (user re-authenticates via OIDC)
- Standard pattern for Spot cluster access
- No manual ServiceAccount token management required

## Cluster Information

- **Server**: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Current read-only proxy**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **Target secret**: `armor-writer` in `devimprint` namespace
- **Provider**: Rackspace Spot (similar to iad-ci, iad-options, iad-acb)

## Related Files

- RBAC config: `~/declarative-config/k8s/ord-devimprint/devpod-observer/secret-reader-sa.yml`
- ArgoCD app: `~/declarative-config/k8s/ord-devimprint/devpod-observer-application.yml`
- Similar pattern: `~/.kube/iad-options.kubeconfig` (cloudspace-admin OIDC token)
