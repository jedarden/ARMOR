# BLOCKER: bf-2xkyl - Cannot Retrieve S3 Credentials

## Status: BLOCKED - Missing Kubeconfig Access

**Date**: 2026-07-11  
**Bead**: bf-2xkyl  
**Blocker**: Missing ord-devimprint kubeconfig with secret read permissions

## Problem

The prerequisite bead `bf-2p1wr` (obtain ord-devimprint kubeconfig with write access) was marked **closed**, but the required kubeconfig file does **not exist**.

## What Was Verified

### Kubeconfig Files (Expected vs Actual)
| Expected File | Status |
|---------------|--------|
| `~/.kube/ord-devimprint.kubeconfig` | ❌ DOES NOT EXIST |
| `~/.kube/rs-manager.kubeconfig` | ❌ DOES NOT EXIST |
| `~/.kube/ardenone-manager.kubeconfig` | ❌ DOES NOT EXIST |

### Available Access
| Access Method | Permissions | Secret Access |
|---------------|-------------|---------------|
| `kubectl-proxy-ord-devimprint:8001` | Read-only (devpod-observer SA) | ❌ **Forbidden** - cannot read secrets |
| No other kubeconfigs available | N/A | N/A |

## Why Read-Only Proxy Cannot Access Secrets

The devpod-observer ServiceAccount explicitly denies secret access:

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error: Forbidden - User "system:serviceaccount:devpod-observer:devpod-observer" cannot get secrets
```

## Acceptance Criteria Status

- ❌ Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- ❌ Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)
- ❌ Credentials stored temporarily in a secure location

## Required Resolution

### Option 1: Obtain ord-devimprint Kubeconfig (Recommended)

Follow the steps documented in `notes/bf-2p1wr-ord-devimprint-kubeconfig.md`:

1. Login to Rackspace Spot console (cloudspace-admin or equivalent)
2. Navigate to the ord-devimprint cluster
3. Download kubeconfig (usually provides cluster-admin access)
4. Save to `~/.kube/ord-devimprint.kubeconfig`
5. Set permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Option 2: Create ServiceAccount with Limited Scope

Apply the YAML from `notes/bf-2p1wr-ord-devimprint-kubeconfig.md` to create an `armor-secret-reader` ServiceAccount with namespace-scoped secret read permissions.

### Option 3: Access via rs-manager (if rs-manager.kubeconfig exists)

If rs-manager kubeconfig becomes available, ord-devimprint secrets might be accessible through the management cluster.

## Current State

- **bf-2xkyl**: IN_PROGRESS but blocked
- **bf-2p1wr**: Incorrectly marked CLOSED (kubeconfig never obtained)
- **Action Required**: Reopen bf-2p1wr or manually obtain kubeconfig

## Next Steps

1. **Obtain kubeconfig** via Rackspace Spot console or cluster administrator
2. **Verify access**:
   ```bash
   kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
   ```
3. **Retry bf-2xkyl** with valid kubeconfig
4. **Update bf-2p1wr** status to reflect actual completion

## Related Documentation

- `notes/bf-2p1wr-ord-devimprint-kubeconfig.md` - Detailed requirements and options
- `declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` - Shows OpenBao secret path
- CLAUDE.md - Kubernetes access section (needs ord-devimprint entry once kubeconfig is obtained)
