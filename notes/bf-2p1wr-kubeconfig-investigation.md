# ord-devimprint Kubeconfig Investigation (Bead bf-2p1wr)

## Current State

### Access Method
- **Read-only proxy**: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount**: `system:serviceaccount:devpod-observer:devpod-observer`
- **Location**: Proxy runs in `devpod-observer` namespace

### Permissions Test Results
```bash
# Listing secrets - WORKS
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE                             DATA   AGE
admin-oauth             Opaque                           3      63d
armor-credentials       Opaque                           7      80d
armor-readonly          Opaque                           2      80d
armor-writer            Opaque                           2      80d    # <-- Target secret
devimprint-b2-workers   Opaque                           5      66d
devimprint-cloudflare   Opaque                           8      80d
...

# Reading secret contents - FORBIDDEN
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

### Existing Kubeconfig
- **Status**: No direct kubeconfig file exists in `~/.kube/`
- **Expected location**: `/home/coding/.kube/ord-devimprint.kubeconfig`

## What's Needed

### Target Secret
- **Name**: `armor-writer`
- **Namespace**: `devimprint`
- **Purpose**: Contains credentials needed for ARMOR deployment (likely database connection details or API credentials)

### Required Permissions
The kubeconfig needs at minimum:
- Read access to secrets in `devimprint` namespace
- Full access (cluster-admin or namespace admin) preferred for flexibility

### Coordination Required
This requires action from the **ord-devimprint cluster administrator**:
1. Generate a kubeconfig with appropriate permissions
2. Deliver it securely to this server
3. Store in `/home/coding/.kube/ord-devimprint.kubeconfig`

## Acceptance Criteria (from bead)
- [ ] Kubeconfig file for ord-devimprint cluster is obtained
- [ ] Kubeconfig has permissions to read secrets in the devimprint namespace
- [ ] Can successfully run: `kubectl get secrets -n devimprint`

## Verification Steps (once kubeconfig is obtained)
```bash
# Test basic connectivity
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get nodes

# Test secret access (the key requirement)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint

# Verify we can decode the secret
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

## Blocker Status
**BLOCKED**: Awaiting cluster administrator to provide kubeconfig with secret access permissions.

## Next Steps (for admin)
1. Create ServiceAccount with secret read permissions in `devimprint` namespace
2. Generate kubeconfig for that ServiceAccount
3. Securely transfer to `~/.kube/ord-devimprint.kubeconfig` on this server

## Environment Context
This server (Hetzner EX44 via Tailscale) uses direct kubeconfigs for other clusters when write access is needed:
- `~/.kube/ardenone-manager.kubeconfig` (cluster-admin)
- `~/.kube/rs-manager.kubeconfig` (cluster-admin)
- `~/.kube/iad-ci.kubeconfig` (cluster-admin via argocd-manager SA)

The ord-devimprint cluster should follow this same pattern.

---
*Created: 2026-07-12*
*Bead: bf-2p1wr*
