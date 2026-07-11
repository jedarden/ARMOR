# Bead bf-2xkyl: Retrieve S3 credentials from armor-writer secret

## Status: BLOCKED - Missing kubeconfig access

## Investigation Summary

### Current Access State
- **Read-only proxy**: `kubectl-proxy-ord-devimprint:8001` can LIST secrets but cannot READ secret contents (RBAC forbidden)
- **Kubeconfig files available**:
  - `~/.kube/iad-ci.kubeconfig` (Rackspace Spot cluster)
  - `~/.kube/iad-acb.kubeconfig` (unknown cluster)
  - **No ord-devimprint kubeconfig exists**

### Prerequisites Not Met
Child bead `bf-2p1wr` (Obtain ord-devimprint kubeconfig with write access) is marked as **closed**, but:
- No kubeconfig file exists at `~/.kube/ord-devimprint.kubeconfig` or any other expected location
- Cannot access secret contents via the read-only proxy
- No alternative access path identified

### Attempts Made
1. **Direct proxy access** - RBAC forbidden:
   ```bash
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
   # Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

2. **Check for kubeconfig** - File not found:
   ```bash
   ls ~/.kube/ord-devimprint*  # No results
   ```

3. **Check ArgoCD cluster registry** - ord-devimprint not registered:
   ```bash
   kubectl --server=http://traefik-ardenone-manager:8001 get clusters -n argocd
   # No devimprint cluster found
   ```

### Blocker Details
The task cannot proceed because:
- Secret read access requires elevated permissions beyond the read-only proxy SA
- No kubeconfig with secret read/write access exists
- Prerequisite bead bf-2p1wr was closed without actually obtaining the required kubeconfig

## Required Actions
1. **Reopen bead bf-2p1wr** - The prerequisite bead needs to be actually completed
2. **Obtain kubeconfig** - Get a kubeconfig with secret read access to ord-devimprint cluster
3. **Verify access** - Confirm kubectl can read secret contents before marking complete
4. **Return to bf-2xkyl** - Once kubeconfig is available, complete credential retrieval

## Commands to Run When Unblocked
Once kubeconfig is obtained at `~/.kube/ord-devimprint.kubeconfig`:
```bash
# Retrieve ACCESS_KEY_ID
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

# Retrieve SECRET_ACCESS_KEY
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d
```

## Notes
- Credentials must be stored temporarily (not committed to git)
- Consider using mktemp or a secure temp location for credential storage
- Coordinates: 2026-07-11 investigation session

## Re-verification - 2026-07-11 15:51 UTC
Re-checked available kubeconfigs:
- `~/.kube/iad-ci.kubeconfig` (confirmed exists)
- `~/.kube/iad-acb.kubeconfig` (confirmed exists)
- No ord-devimprint kubeconfig found

Re-tested read-only proxy access:
```
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Still returns: Forbidden - devpod-observer SA cannot read secrets
```

**Conclusion**: Blocker remains active. Task cannot proceed without ord-devimprint kubeconfig with secret access.
