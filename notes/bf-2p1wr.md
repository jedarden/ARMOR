# Bead bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Date: 2026-07-11

## Task
Acquire a kubeconfig file with write access to the ord-devimprint cluster.

## Current State Investigation

### Existing Access Methods

1. **Read-Only Proxy (Currently Working)**
   - Endpoint: `http://kubectl-proxy-ord-devimprint:8001`
   - RBAC: Read-only (devpod-observer ServiceAccount)
   - Can list pods and secrets
   - ❌ Cannot read secret contents or perform write operations

2. **Direct Kubeconfig (Previously Existed)**
   - Expected location: `~/.kube/ord-devimprint.kubeconfig`
   - Current status: ❌ File does not exist
   - Previous auth method: OIDC (kubectl-oidc-login plugin)
   - Last known working: 2026-05-01 (per armor-bik.md)

### Cluster Information

- **Provider:** OpenStack (providerID: `openstack:///62a06c73-3882-4765-ad54-35437e1143da`)
- **Ingress:** Tailscale operator (hostname: `kubectl-proxy-ord-devimprint`)
- **Namespaces of interest:** `devimprint`

### Verification Tests

```bash
# Current read-only proxy - can list but not read secrets
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
# Returns list of secrets including: armor-writer, armor-readonly, admin-oauth

# But cannot read secret data
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Acceptance Criteria Status

- [ ] Kubeconfig file for ord-devimprint cluster is obtained
- [ ] Kubeconfig has permissions to read secrets in the devimprint namespace
- [ ] Can successfully run: `kubectl get secrets -n devimprint`
- [ ] Can successfully run: `kubectl get secret armor-writer -n devimprint -o json`

## Requirements to Complete

This task **requires cluster administrator coordination**. The kubeconfig file must be created by someone with cluster-admin access to the ord-devimprint OpenStack cluster.

### Required Actions (by Cluster Administrator)

1. **Create ServiceAccount with appropriate RBAC** in the `devimprint` namespace:
   - Read access to secrets
   - Read/write access to pods (for debugging)

2. **Generate kubeconfig** for the ServiceAccount or user account

3. **Deliver kubeconfig securely** to this server at: `~/.kube/ord-devimprint.kubeconfig`

4. **Set appropriate permissions:** `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Alternative: OIDC Authentication (if previously used)

If the cluster uses OIDC authentication (as suggested by previous notes), the cluster administrator needs to:

1. Create/renew OIDC token for the user
2. Configure kubectl-oidc-login plugin
3. Ensure kubeconfig references the correct OIDC issuer and client ID

## Next Steps

1. **Contact cluster administrator** to request write-access kubeconfig
2. **Specify required permissions:**
   - Read secrets in `devimprint` namespace
   - Read/write pods in `devimprint` namespace (for debugging)
3. **Store kubeconfig** at `~/.kube/ord-devimprint.kubeconfig` when received
4. **Verify access** by reading the `armor-writer` secret

## Dependencies

This bead blocks bead `bf-4ds4n` which needs to verify the kubeconfig works.

## References

- Previous kubeconfig notes:
  - `notes/armor-bik.md` - Last known working kubeconfig (2026-05-01)
  - `notes/armor-s8k.3.2.2-final-summary-2026-05-02.md` - OIDC authentication issues
  - `notes/bf-4ds4n.md` - Verification that kubeconfig is missing
