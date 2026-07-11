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

## Status

⚠️ **Awaiting kubeconfig from cluster administrator** - This requires access to Rackspace Spot console or coordination with the cluster admin who can provide credentials.
