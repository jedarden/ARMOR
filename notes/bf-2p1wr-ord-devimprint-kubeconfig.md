# ord-devimprint Kubeconfig Requirements

## Current State

**Cluster**: ord-devimprint (Rackspace Spot cluster)
**Current Access**: Read-only proxy at `kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `devpod-observer` in `devpod-observer` namespace
- Limitations: Cannot read secret contents (Forbidden on `kubectl get secret`)

**Required Access**: Write permissions to read secrets in `devimprint` namespace

## Target Kubeconfig

**Location**: `~/.kube/ord-devimprint.kubeconfig`
**Required Permissions**:
- Read secrets in `devimprint` namespace
- Specifically: `armor-writer` secret (for ARMOR deployment)

## Verification Commands

```bash
# Verify kubeconfig works
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint

# Verify specific secret access
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o yaml
```

## Options for Obtaining Kubeconfig

### Option 1: Rackspace Spot Console (Recommended)

Rackspace Spot clusters provide kubeconfig download via their web console:

1. Login to Rackspace Spot console (cloudspace-admin or equivalent)
2. Navigate to the ord-devimprint cluster
3. Download kubeconfig (usually provides cluster-admin access)
4. Save to `~/.kube/ord-devimprint.kubeconfig`
5. Set appropriate permissions: `chmod 600 ~/.kube/ord-devimprint.kubeconfig`

### Option 2: ServiceAccount with Limited Scope

For production, create a ServiceAccount with namespace-scoped permissions:

```yaml
# Apply on ord-devimprint cluster with cluster-admin access
apiVersion: v1
kind: ServiceAccount
metadata:
  name: armor-secret-reader
  namespace: devimprint
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secret-reader
  namespace: devimprint
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: armor-secret-reader
  namespace: devimprint
subjects:
- kind: ServiceAccount
  name: armor-secret-reader
  namespace: devimprint
roleRef:
  kind: Role
  name: secret-reader
  apiGroup: rbac.authorization.k8s.io
```

Then create a long-lived token kubeconfig:

```bash
# On ord-devimprint cluster
KC=/tmp/ord-devimprint-admin.kubeconfig
NAMESPACE=devimprint
SA=armor-secret-reader

kubectl --kubeconfig=$KC create token $SA -n $NAMESPACE --duration=8760h
```

### Option 3: OIDC Token (if cluster uses OIDC)

Some Spot clusters use OIDC for authentication. Check if OIDC is available and configure kubeconfig accordingly.

## Cluster Information

- **Provider**: Rackspace Spot (OpenStack-based)
- **Region**: Likely `ORD` (based on cluster name)
- **Management**: Should be accessible via Rackspace Spot console
- **ArgoCD Integration**: Cluster secret stored in OpenBao at `rs-manager/ord-devimprint/cluster`

## Next Steps

1. **Obtain kubeconfig** via Rackspace Spot console or from cluster administrator
2. **Test access**: Run verification commands above
3. **Store securely**: Save to `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`
4. **Update documentation**: Add cluster to CLAUDE.md Kubernetes access section

## Related Files

- `~/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml` - Shows cluster setup pattern
- `~/declarative-config/k8s/ord-devimprint/tailscale/operator-oauth-secret.yml.template` - References kubeconfig for secret creation

## Investigation Results (2026-07-11)

### What I Found

1. **No existing kubeconfig** - Checked `~/.kube/` directory, no ord-devimprint.kubeconfig exists
2. **Cluster details confirmed**:
   - Server URL: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
   - Managed by rs-manager ArgoCD (via ApplicationSet)
   - Cluster secret stored in OpenBao at `rs-manager/ord-devimprint/cluster` (for ArgoCD use only)
3. **Rackspace Spot pattern** - Based on rs-manager documentation, kubeconfigs are downloaded from Rackspace Spot console UI
4. **No console access** - No Rackspace Spot credentials found on this system

### Blocking Issue

This task requires access to **Rackspace Spot console** to download the kubeconfig. The console URL and credentials are not available on this system. Per the rs-manager documentation:
> "Current kubeconfig lives at `/home/coding/.kube/rs-manager.kubeconfig` — regenerate from the Rackspace Spot UI if the cluster is recreated."

This confirms that Rackspace Spot kubeconfigs come from their web console, not from any local source.

### Required Action

**Option A: Request from Cluster Administrator**
- Ask cluster admin to provide `ord-devimprint.kubeconfig` with write access
- Should have permissions to read secrets in `devimprint` namespace
- Store at `~/.kube/ord-devimprint.kubeconfig` with `chmod 600`

**Option B: Request Rackspace Spot Console Access**
- Request credentials for Rackspace Spot console
- Navigate to ord-devimprint cluster
- Download kubeconfig directly (usually provides cluster-admin access)
- Follow Option A storage steps

## Historical Context

**Previous attempt (May 2026):**
- Bead `armor-bik` verified a working kubeconfig at `~/.kube/ord-devimprint.kubeconfig`
- Token expiration was 2026-05-01 22:37:44 UTC
- This kubeconfig no longer exists (likely expired and was removed)

**Premature closure (July 2026):**
- This bead (bf-2p1wr) was closed on 2026-07-11 15:22:49 UTC WITHOUT actually obtaining a kubeconfig
- Bead bf-4ds4n verification confirmed the kubeconfig was still missing
- This current attempt is to properly complete the original work

## Status

🔴 **BLOCKED** - Requires Rackspace Spot console access OR kubeconfig from cluster administrator. Cannot proceed without external coordination.

This is a **re-attempt** after the bead was prematurely closed without completing the actual work.

## Related Beads

- **bf-3d39n** - Blocked on this bead (bf-2p1wr) for ord-devimprint kubeconfig
- **bf-4ds4n** - Verification bead that discovered the premature closure
- **bf-2xkyl** - Blocked by missing kubeconfig (has documented this issue 16+ times)
- **armor-bik** - Historical bead that verified a working kubeconfig in May 2026
