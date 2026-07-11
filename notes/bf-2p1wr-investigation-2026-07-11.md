# ord-devimprint Kubeconfig Investigation - 2026-07-11

## Current Status: BLOCKED - Requires Human Action

### Verified Limitations

1. **No Write-Access Kubeconfig Exists**
   - Checked `~/.kube/ord-devimprint.kubeconfig` - does not exist
   - Only 2 kubeconfigs present in `~/.kube/`:
     - `iad-acb.kubeconfig`
     - `iad-ci.kubeconfig`

2. **Read-Only Proxy Confirmed**
   - Proxy available: `kubectl-proxy-ord-devimprint:8001`
   - ServiceAccount: `system:serviceaccount:devpod-observer:devpod-observer`
   - **Can LIST secrets**: `kubectl get secrets -n devimprint` works
   - **Cannot GET secret data**: Forbidden error on `kubectl get secret armor-writer -n devimprint -o jsonpath='{.data}'`

3. **No Programmatic Access Available**
   - No Rackspace Spot CLI tools found (spotctl, rackspot, etc.)
   - No Rackspace Spot API credentials in environment
   - No automation patterns in terraform/rackspace-spot for kubeconfig generation

### Why This Requires Human Intervention

Rackspace Spot clusters use OIDC authentication with web UI-based kubeconfig generation. Based on the pattern for `iad-options` cluster:
- Kubeconfigs must be downloaded from the Rackspace Spot console
- Tokens expire every ~3 days
- No API/programmatic method available without existing credentials

### Action Required

To complete this bead, a human with Rackspace Spot console access must:

1. Log into Rackspace Spot console (https://spot.rackspace.com)
2. Navigate to the ord-devimprint cluster (ORD region)
3. Download/generate a kubeconfig with secret-read permissions
4. Save to: `~/.kube/ord-devimprint.kubeconfig`
5. Verify with: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint`

### Next Steps After Kubeconfig is Available

Once the kubeconfig is obtained, the next task is to retrieve the `armor-writer` secret:
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

### Related Documentation

- See `notes/bf-2p1wr.md` for detailed investigation log
- 4 previous verification attempts (2026-07-11, 2026-06-10) all reached same conclusion
- Persistent blocker documented across multiple commits

## Investigation Date

2026-07-11 (5th verification)
