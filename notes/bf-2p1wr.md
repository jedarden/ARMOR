# ord-devimprint Kubeconfig Acquisition

## Task
Obtain kubeconfig with write access to ord-devimprint cluster to read the `armor-writer` secret.

## Current State

### Available Access
- **Read-only proxy:** `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount:** `devpod-observer` in `devpod-observer` namespace
- **Limitations:** Cannot read secrets (Forbidden on `get secret`)

### Cluster Details
- **Type:** Rackspace Spot cluster
- **Server:** `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Managed via:** rs-manager ArgoCD ApplicationSet
- **Tailscale exposure:** Via operator (hostname: `kubectl-proxy-ord-devimprint`)

### Existing Secrets in devimprint namespace
```
NAME                    TYPE                             AGE
admin-oauth             Opaque                           63d
armor-credentials       Opaque                           81d
armor-readonly          Opaque                           81d
armor-writer            Opaque                           81d    ← Target secret
devimprint-b2-workers   Opaque                           66d
devimprint-cloudflare   Opaque                           81d
docker-hub-registry     kubernetes.io/dockerconfigjson   81d
github-oauth            Opaque                           32d
github-pat              Opaque                           81d
queue-api-auth          Opaque                           2d
```

## Attempted Solution

A `secret-reader-sa.yml` was created in `declarative-config/k8s/ord-devimprint/devpod-observer/` to add a ServiceAccount with secret-read permissions. However:
- This has not synced to the cluster via ArgoCD (or failed)
- Even if synced, the token cannot be retrieved through the read-only proxy
- This approach requires write access to bootstrap

## Required Action

To obtain a kubeconfig with write access to ord-devimprint:

### Option 1: Rackspace Spot Dashboard (Recommended)
1. Log in to Rackspace Spot dashboard at `https://spot.rackspace.com`
2. Navigate to the `ord-devimprint` cluster (hcp-5f30c973-cde7-42d9-8c7b-5d0573821330)
3. Download kubeconfig with cluster-admin or namespace-admin privileges
4. Store at: `~/.kube/ord-devimprint.kubeconfig`

### Option 2: Cluster Administrator Coordination
Request kubeconfig from the cluster administrator with:
- Access level: At minimum, `secret` reader in `devimprint` namespace
- Preferred: Full `cluster-admin` (like rs-manager.kubeconfig)
- Format: Standard kubeconfig YAML

## Verification Steps (Once Kubeconfig is Obtained)

```bash
# Test basic access
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get nodes

# Test secret access (goal)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint

# Verify we can decode the secret
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

## Related Files

- ArgoCD ApplicationSet: `declarative-config/k8s/rs-manager/argocd/ord-devimprint-applicationset.yml`
- Observer RBAC: `declarative-config/k8s/ord-devimprint/devpod-observer/rbac.yml`
- Secret-reader SA (pending sync): `declarative-config/k8s/ord-devimprint/devpod-observer/secret-reader-sa.yml`

## Next Steps

1. **External action required:** Obtain kubeconfig from Rackspace Spot dashboard or cluster administrator
2. Save kubeconfig to `~/.kube/ord-devimprint.kubeconfig`
3. Verify access with test commands above
4. Proceed to retrieve `armor-writer` secret for parent task

## Status

- **Current:** Awaiting external kubeconfig provision from Rackspace Spot
- **Blocker:** Cannot read secrets through read-only proxy
- **Estimated effort:** Low (once dashboard access is available)
