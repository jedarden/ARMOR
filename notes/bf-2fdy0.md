# Bead bf-2fdy0: Retrieve base64-encoded SECRET_ACCESS_KEY from Kubernetes secret - BLOCKED

## Task
Extract the LITESTREAM_SECRET_ACCESS_KEY value from the armor-writer secret in the devimprint namespace.

This step only retrieves the base64-encoded value without decoding it.

## Status: BLOCKED - RBAC Infrastructure Limitation

### Root Cause
The SECRET_ACCESS_KEY retrieval is blocked by RBAC:

1. **ord-devimprint cluster access is read-only only**
   - Only available access: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
   - ServiceAccount: `system:serviceaccount:devpod-observer:devpod-observer`
   - No direct kubeconfig exists (no `~/.kube/ord-devimprint.kubeconfig`)

2. **Read-only proxy lacks secret read permissions**
   ```
   Error from server (Forbidden): secrets "armor-writer" is forbidden: 
   User "system:serviceaccount:devpod-observer:devpod-observer" cannot get 
   resource "secrets" in API group "" in the namespace "devimprint"
   ```

3. **ExternalSecret exists and syncs successfully**
   - ExternalSecret `armor-writer` shows `SecretSynced` status
   - Last sync: 4m5s ago
   - But the underlying Secret cannot be accessed via read-only proxy

### Verification Attempts
```bash
# Attempt 1: Using specified kubeconfig (does not exist)
$ kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig \
    get secret armor-writer -n devimprint \
    -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'
error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory

# Attempt 2: Using read-only proxy
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
    get secret armor-writer -n devimprint \
    -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'
Error from server (Forbidden): secrets "armor-writer" is forbidden

# Attempt 3: Check permissions
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
    auth can-i get secrets -n devimprint
no
```

### Comparison with Other Clusters
Other clusters have both read-only proxy AND read/write kubeconfig:
- **ardenone-manager**: `~/.kube/ardenone-manager.kubeconfig` (cluster-admin)
- **rs-manager**: `~/.kube/rs-manager.kubeconfig` (cluster-admin)
- **ord-devimprint**: Read-only proxy only, no direct kubeconfig

### Related Beads
- **bf-41jxs**: Encountered same RBAC blocker for SECRET_ACCESS_KEY retrieval
- **bf-520v**: Used cached secrets to avoid OpenBao dependency when RBAC blocked access

## Resolution Options
To complete this task, one of:

1. **Create ord-devimprint.kubeconfig** with cluster-admin access
2. **Grant secret read permissions** to devpod-observer ServiceAccount
3. **Use cached/alternative credentials** (as done in bf-520v)
4. **Access via different cluster** that has visibility into devimprint secrets

## Acceptance Criteria Status
- ❌ Successfully executed kubectl command to retrieve the secret (BLOCKED)
- ❌ Captured the base64-encoded SECRET_ACCESS_KEY value (BLOCKED)
- ❌ Value is stored in a temporary file for the next step (BLOCKED)

## Files Checked
- No existing `ord-devimprint.kubeconfig` in `~/.kube/`
- ExternalSecret `armor-writer` exists and syncs successfully
- No cached SECRET_ACCESS_KEY found in local files
