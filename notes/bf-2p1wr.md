# bf-2p1wr: ord-devimprint Kubeconfig Acquisition

## Summary

This task requires obtaining a kubeconfig file with write access to the ord-devimprint cluster to retrieve the `armor-writer` secret in the `devimprint` namespace.

## Current Access Status

**Available Access:**
- Read-only kubectl proxy: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `system:serviceaccount:devpod-observer:devpod-observer`
- Limitation: **Cannot read secrets** - Forbidden error when attempting to access secrets

**Current Cluster Details:**
- **Provider:** Rackspace Spot
- **Cluster ID:** `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
- **Server URL:** `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Tailscale exposure:** Via Tailscale operator (hostname: `kubectl-proxy-ord-devimprint`)
- **Region:** Likely us-west-002 (based on B2 bucket references in deployment configs)

## Required Procedure

To obtain write access to ord-devimprint, follow this procedure:

### Step 1: Access Rackspace Spot UI
- Navigate to: https://spot.rackspace.com
- Login with Rackspace Spot credentials

### Step 2: Locate ord-devimprint Cloudspace
- Find the cloudspace/cluster with ID: `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
- Cluster name: `ord-devimprint`

### Step 3: Download Kubeconfig
- Use the Spot UI to download a kubeconfig with **cloudspace-admin OIDC token**
- This provides cluster-admin level access

### Step 4: Store Securely
- Save the kubeconfig to: `~/.kube/ord-devimprint.kubeconfig`
- Set appropriate permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Step 5: Verify Access
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
```

Expected result: List of secrets including `armor-writer`

## Pattern Reference

This follows the same pattern as **iad-options** (another Rackspace Spot cluster):
- Kubeconfig path: `~/.kube/iad-options.kubeconfig`
- Token type: cloudspace-admin OIDC token
- **Expiration:** ~3 days (requires periodic regeneration from Spot UI)

## Similar Clusters with Known Access

| Cluster | Kubeconfig Path | Access Method |
|---------|-----------------|----------------|
| iad-ci | ~/.kube/iad-ci.kubeconfig | ServiceAccount (argocd-manager) |
| iad-options | ~/.kube/iad-options.kubeconfig | Spot UI (cloudspace-admin OIDC) |
| rs-manager | ~/.kube/rs-manager.kubeconfig | Direct kubeconfig |
| **ord-devimprint** | **~/.kube/ord-devimprint.kubeconfig** | **Spot UI (cloudspace-admin OIDC)** |

## Why Spot UI Access is Required

1. **OIDC Token Authentication:** Rackspace Spot clusters use OIDC tokens that must be generated through the Spot UI
2. **No Static Credentials:** Unlike iad-ci (ServiceAccount), these clusters require time-bound tokens
3. **Cluster-Admin Access:** The cloudspace-admin token provides the necessary permissions to read secrets
4. **Security Model:** Spot clusters follow a zero-trust model with short-lived credentials

## Dependencies

This task is blocked by:
- **Spot UI Access:** Requires login credentials to https://spot.rackspace.com
- **Cloudspace Access:** Must have permission to access the ord-devimprint cloudspace

## Next Steps After Kubeconfig Acquisition

Once the kubeconfig is obtained and verified:

1. **Verify Secret Access:**
   ```bash
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
   ```

2. **Retrieve Secret for ARMOR Operations:**
   ```bash
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}' | base64 -d
   ```

3. **Proceed with Dependent Tasks:**
   - Tasks requiring armor-writer secret credentials
   - ARMOR deployment and configuration

## Notes

- **Token Refresh:** The kubeconfig will need to be regenerated every ~3 days as the OIDC token expires
- **GitOps Compliance:** All cluster changes should still go through declarative-config, not direct kubectl applies
- **ExternalSecrets:** The cluster uses ExternalSecrets referencing OpenBao paths under `rs-manager/ord-devimprint/*`
- **Declarative Config:** Cluster configuration is managed via `~/declarative-config/k8s/ord-devimprint/`

## Verification History

This task has been a persistent blocker across multiple sessions, with verification attempts showing:
- Read-only proxy explicitly denies secret access
- No existing kubeconfig file exists in ~/.kube/
- Spot UI access is required for credential acquisition
- Pattern matches iad-options cluster access method

## Coordination Required

This task requires coordination with:
- **Cluster Administrator:** To obtain or verify Spot UI access credentials
- **Rackspace Spot Access:** To login and download the kubeconfig
