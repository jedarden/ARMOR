# Bead bf-4ds4n: Verify ord-devimprint write-access kubeconfig exists

## Task
Verify that we have a working kubeconfig with write access to the ord-devimprint cluster.

## Investigation Results
**Verification Date**: 2026-07-11 (re-verified - blocker persists)

### Expected Location
- `~/.kube/ord-devimprint.kubeconfig` (per prerequisite bead bf-2p1wr)

### Actual State
- **File does NOT exist** - verified with `ls -la /home/coding/.kube/ord-devimprint*`
- Only kubeconfigs present: `iad-acb.kubeconfig` and `iad-ci.kubeconfig`

### Read-Only Proxy Capabilities
The read-only proxy at `kubectl-proxy-ord-devimprint:8001`:
- **CAN read pods**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint` ✓
- **CAN list secrets**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint` ✓
- **CANNOT read secret data**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint` → "Forbidden: User \"system:serviceaccount:devpod-observer:devpod-observer\" cannot get resource \"secrets\"" ✗
- **CANNOT create**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i create pods -n devimprint` → "no" ✗
- **CANNOT delete**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i delete pods -n devimprint` → "no" ✗

### Prerequisite Bead Status
- **bf-2p1wr**: "Obtain ord-devimprint kubeconfig with write access"
- Status: **closed** (via CLI on 2026-07-11 15:22:49 UTC)
- **Evidence**: The kubeconfig file was never actually created
- **Conclusion**: Bead was closed prematurely without completing the work

### Acceptance Criteria Status
- [ ] Kubeconfig file exists at a known location - **FAILED**
- [ ] Can successfully authenticate to ord-devimprint cluster - **CANNOT TEST** (no kubeconfig)
- [ ] Has write access to the devimprint namespace (not read-only) - **CANNOT TEST** (no kubeconfig)

## Verification Commands Attempted
```bash
# Check for kubeconfig file
ls -la ~/.kube/ord-devimprint*
# Result: No such file or directory

# Test proxy secret listing
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# Result: SUCCESS (can list secrets)

# Test proxy secret data read
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
# Result: Forbidden - User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"

# Test proxy write permissions
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i create pods -n devimprint
# Result: no (permission denied)

kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i delete pods -n devimprint
# Result: no (permission denied)
```

## Conclusion
**BLOCKED**: The write-access kubeconfig does not exist. The prerequisite bead bf-2p1wr was closed without actually creating the kubeconfig file.

The read-only proxy is insufficient for operations requiring secret data access. While the proxy can **list** secrets (including `armor-writer`, `armor-credentials`, etc.), it **cannot read the actual secret values** due to RBAC restrictions (Forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets").

To complete this task, a kubeconfig with secret read access must be obtained from the Rackspace Spot dashboard for the ord-devimprint cluster and stored at `~/.kube/ord-devimprint.kubeconfig`.

## Related Beads
- **bf-2p1wr**: Prerequisite - "Obtain ord-devimprint kubeconfig with write access" (closed prematurely)
- **bf-2xkyl**: "Retrieve S3 credentials from armor-writer secret" - may be blocked by this
- **armor-bik**: Previously refreshed an expired token for this kubeconfig (file no longer exists)

## Next Steps Required
1. Re-open and complete bead bf-2p1wr to actually create the kubeconfig
2. Obtain cloudspace-admin OIDC token from Rackspace Spot UI for ord-devimprint cluster
3. Create kubeconfig at `~/.kube/ord-devimprint.kubeconfig`
4. Re-verify with this bead's acceptance criteria
