# Bead bf-2p1wr: ord-devimprint Kubeconfig Acquisition

## Current Status

**BLOCKED** - Requires manual coordination to complete.

## What Was Done

1. **Created RBAC configuration** in `~/declarative-config/k8s/ord-devimprint/devpod-observer/secret-reader-sa.yml`:
   - ServiceAccount: `secret-reader` in `devpod-observer` namespace
   - Role: `secret-reader-devimprint` with `get` and `list` permissions on secrets in `devimprint` namespace
   - RoleBinding linking the service account to the role
   - Secret token for the service account

2. **Pushed to declarative-config**:
   - Committed: `feat(ord-devimprint): add secret-reader service account for devimprint namespace`
   - Pushed to `main` branch
   - Commit: `f8d6223`

## Current Blocker

The resources have not been synced to ord-devimprint cluster yet. Possible reasons:
- ArgoCD application `devpod-observer-ord-devimprint` may not be syncing automatically
- The ord-devimprint cluster may not be registered with ArgoCD on ardenone-manager
- Manual intervention may be required to bootstrap access

## Next Steps (Requires Manual Coordination)

### Option 1: Via ArgoCD Sync
If the cluster is registered with ArgoCD:
1. Verify the application syncs the new resources
2. Wait for `secret-reader-token` secret to be created in `devpod-observer` namespace
3. Extract the token: `kubectl get secret secret-reader-token -n devpod-observer -o jsonpath='{.data.token}' | base64 -d`
4. Create kubeconfig at `~/.kube/ord-devimprint.kubeconfig`

### Option 2: Direct Kubeconfig from Rackspace Spot
Since ord-devimprint is a Rackspace Spot cluster:
1. Access Rackspace Spot console for the `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330` cluster
2. Download kubeconfig with cloudspace-admin credentials
3. Store at `~/.kube/ord-devimprint.kubeconfig`
4. Verify access: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`

## Verification Command

Once the kubeconfig is obtained:
```bash
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
```

## Related Files

- RBAC config: `~/declarative-config/k8s/ord-devimprint/devpod-observer/secret-reader-sa.yml`
- ArgoCD app: `~/declarative-config/k8s/ord-devimprint/devpod-observer-application.yml`

## Notes

- Server URL: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- Current read-only proxy: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001`
- Target secret: `armor-writer` in `devimprint` namespace
