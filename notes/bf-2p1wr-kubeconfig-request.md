# ord-devimprint Kubeconfig Request

## Status: BLOCKED - Requires Rackspace Spot Console Access

## Current Situation

### Working Access
- **Read-only proxy:** `kubectl-proxy-ord-devimprint:8001` 
- **Capabilities:** List resources (including secret names)
- **Limitation:** Cannot read secret contents (Forbidden by RBAC)

### Required Access
- **Target secret:** `armor-writer` in namespace `devimprint`
- **Needed permission:** `get secrets` in `devimprint` namespace
- **Current blocker:** ServiceAccount `devpod-observer:devpod-observer` lacks secret read permission

## How to Obtain Kubeconfig

The ord-devimprint cluster is managed by **Rackspace Spot** and requires console access to generate a kubeconfig with write permissions.

### Step 1: Access Rackspace Spot Console
```
URL: https://argocd-rs-manager.tail1b1987.ts.net:8080
Access: Tailscale VPN only
Auth: Google SSO (or configured authentication)
```

### Step 2: Locate ord-devimprint Cluster
In the ArgoCD UI:
1. Navigate to **Clusters** (or **Settings** → **Clusters**)
2. Find cluster named `ord-devimprint` or with server matching the pattern
3. Click to view cluster details

### Step 3: Generate/Download Kubeconfig
Options depending on Spot console features:
- Look for "Download kubeconfig" or "Generate kubeconfig" button
- Or create a ServiceAccount with appropriate permissions and extract token
- Similar pattern to `iad-options.kubeconfig` (cloudspace-admin OIDC token from Spot UI)

### Step 4: Store Securely
```bash
# Save to standard location
mv ~/Downloads/kubeconfig-ord-devimprint.yaml ~/.kube/ord-devimprint.kubeconfig
chmod 600 ~/.kube/ord-devimprint.kubeconfig
```

### Step 5: Verify Access
```bash
# Should list secret contents (not just names)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint -o jsonpath='{.data}'

# Quick sanity check
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig \
  get secrets -n devimprint
```

## Alternative: rs-manager Access

If the rs-manager kubeconfig exists (not currently present at `~/.kube/rs-manager.kubeconfig`):
```bash
# rs-manager has ArgoCD secrets that may contain cluster credentials
kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig \
  get secret -n argocd cluster-ord-devimprint -o json
```

Note: ArgoCD cluster secrets contain connection info but may not be directly usable as kubeconfigs.

## Pattern Reference

This follows the same pattern as other Rackspace Spot clusters:
- **iad-ci:** Has `~/.kube/iad-ci.kubeconfig` with `argocd-manager` ServiceAccount token
- **iad-options:** Uses OIDC cloudspace-admin token from Spot UI (expires every ~3 days)
- **ord-devimprint:** Should follow similar pattern

## Next Steps

1. **Person with Rackspace Spot console access:**
   - Log in to https://argocd-rs-manager.tail1b1987.ts.net:8080
   - Generate kubeconfig for ord-devimprint with secret read permissions
   - Save to `~/.kube/ord-devimprint.kubeconfig`
   - Run verification command above

2. **Once kubeconfig is obtained:**
   - Run verification: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint`
   - Proceed to next child bead to retrieve armor-writer secret data

## Verification Commands

```bash
# Test read access to armor-writer secret
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint -o json

# Expected output: JSON with .data containing base64-encoded secret values
# Error "Forbidden" means permissions are insufficient
```

## Documentation References
- CLAUDE.md: Documents ord-devimprint as read-only proxy only
- Existing notes/bf-2p1wr.md: Contains detailed documentation
- Pattern matches iad-options OIDC token approach

---

**Generated:** 2026-07-11
**Bead:** bf-2p1wr
**Status:** AWAITING CONSOLE ACCESS
