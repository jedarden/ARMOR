# 6th Verification: ord-devimprint Kubeconfig Blocker Persists

**Date:** 2026-07-11
**Bead:** bf-2p1wr
**Verification Count:** 6th overall

## Finding

The persistent blocker requiring human access to the Rackspace Spot console remains in effect.

### Verification Steps

1. **Checked for existing kubeconfig files:**
   ```bash
   ls -la ~/.kube/*.kubeconfig | grep -i ord
   # Result: No output - no ord-devimprint kubeconfig exists
   ```

2. **Verified read-only proxy still blocks secret access:**
   ```bash
   # Can LIST secrets (allowed by RBAC)
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
   # Result: Shows secret names (admin-oauth, armor-credentials, armor-readonly, armor-writer)

   # Cannot GET/READ secret values (blocked by RBAC)
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data}'
   # Result: Error from server (Forbidden): secrets "armor-writer" is forbidden:
   #         User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

3. **Confirmed RBAC restriction:**
   The `devpod-observer` ServiceAccount has only `list` verb on secrets, not `get` or `watch`.
   This means secret names are visible but secret data values cannot be retrieved.

## Conclusion

**BLOCKER REMAINS:** Cannot be completed programmatically. Requires human action:

1. Log into Rackspace Spot console
2. Navigate to ord-devimprint cluster (ORD region)
3. Generate/download kubeconfig with secret read permissions
4. Save to `~/.kube/ord-devimprint.kubeconfig`
5. Verify with: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`

## Pattern Confirmation

This matches the pattern used for other Rackspace Spot clusters:
- `iad-options`: Requires Spot UI to regenerate OIDC token every ~3 days
- `iad-ci`: Full cluster-admin access via direct kubeconfig
- `rs-manager`: Full cluster-admin access via direct kubeconfig

The read-only proxy pattern is consistent across all Spot clusters - it provides safe, read-only access for debugging but deliberately blocks sensitive operations like reading secrets.

## Next Steps

**Bead should remain open** pending human action to obtain the kubeconfig from Rackspace Spot console.
