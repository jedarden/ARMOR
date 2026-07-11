# Obtaining ord-devimprint Kubeconfig with Write Access

## Current Situation

The `ord-devimprint` cluster currently has a read-only kubectl proxy at `kubectl-proxy-ord-devimprint:8001`. This proxy uses a ServiceAccount (`devpod-observer`) with RBAC that explicitly denies secret access:

```yaml
# From declarative-config/k8s/ord-devimprint/devpod-observer/rbac.yml
- apiGroups: [""]
  resources:
    - secrets
  verbs: ["list"]  # Only list, NOT get/watch - cannot read secret values
```

This is why `kubectl get secret armor-writer -n devimprint` returns:
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## What's Needed

A kubeconfig file with sufficient permissions to read secrets in the `devimprint` namespace. Based on the pattern for other Rackspace Spot clusters (like `iad-options`), this requires:

1. Access to the **Rackspace Spot console** (web UI)
2. Generating a kubeconfig/OIDC token for the `ord-devimprint` cluster
3. Saving it to `~/.kube/ord-devimprint.kubeconfig`

## Why This Requires Manual Intervention

Rackspace Spot clusters use OIDC authentication with short-lived tokens (expires every ~3 days for `iad-options`). The kubeconfig must be generated through the Spot console's interface - there is no API or programmatic way to obtain it without existing credentials.

This is a **persistent blocker** that has been re-verified multiple times:
- 2026-07-11: Re-verified - read-only proxy still blocks secret access, requires Rackspace Spot console
- 2026-06-10: Re-verified - requires Rackspace Spot console access (commit `330b6d2a`)
- Earlier: Multiple re-verification attempts (commits `44053922`, `0fb2d44d`)

## Steps to Complete (Requires Human Action)

1. **Log into Rackspace Spot console**
   - Navigate to the ord-devimprint cluster (ORD region)
   - This requires account access with permissions for this cluster

2. **Generate kubeconfig**
   - Use the Spot UI to download or generate a kubeconfig
   - This typically creates an OIDC-based auth token
   - Ensure it has permissions to read secrets in the `devimprint` namespace

3. **Save the kubeconfig**
   ```bash
   # Save to standard location
   cp ~/Downloads/kubeconfig-ord-devimprint ~/.kube/ord-devimprint.kubeconfig
   
   # Set proper permissions
   chmod 600 ~/.kube/ord-devimprint.kubeconfig
   ```

4. **Verify access**
   ```bash
   # Test secret access
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint
   
   # Specifically verify armor-writer is accessible
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
   ```

## Acceptance Criteria

- [ ] Kubeconfig file exists at `~/.kube/ord-devimprint.kubeconfig`
- [ ] Kubeconfig has permissions to read secrets in the `devimprint` namespace
- [ ] Successfully run: `kubectl get secrets -n devimprint` with the kubeconfig
- [ ] Can read the `armor-writer` secret specifically

## Reference Pattern

For comparison, the `iad-options` cluster uses a similar pattern:
- **Read-only proxy**: `kubectl --server=http://traefik-iad-options:8001` (denies secret access)
- **Read/write kubeconfig**: `kubectl --kubeconfig=/home/coding/.kube/iad-options.kubeconfig`
- **Token expiry**: ~3 days (requires regeneration from Spot UI)

## After Completion

Once the kubeconfig is obtained and verified, this bead can be closed and dependent work can proceed to retrieve the `armor-writer` secret.

## Investigation Log

| Date | Action | Finding |
|------|--------|---------|
| 2026-07-11 | Re-verified access (3rd attempt) | Verified read-only proxy (kubectl-proxy-ord-devimprint:8001) still blocks secret access with Forbidden error. Confirmed no kubeconfig exists at ~/.kube/ord-devimprint.kubeconfig. Persistent blocker remains - requires Rackspace Spot console access. |
| 2026-07-11 | Re-verified access (2nd attempt) | Read-only proxy (kubectl-proxy-ord-devimprint:8001) still explicitly denies secret access. RBAC confirms secrets only have `list` verb, not `get`. No programmatic path to obtain write-access kubeconfig. |
| 2026-07-11 | Re-verified access (1st attempt) | Initial investigation documented persistent blocker requiring Rackspace Spot console access |
| 2026-06-10 | Previous verification | Documented persistent blocker requiring Rackspace Spot console access |

Last verified: 2026-07-11
Bead: bf-2p1wr
Status: BLOCKED - Requires human access to Rackspace Spot console
