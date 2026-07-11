# Task bf-37mxj: Obtain S3 credentials from ord-devimprint cluster

## Status: BLOCKED - Cannot complete without required access

## What I Found

### Target Secret
- **Name:** `armor-writer` 
- **Namespace:** `devimprint`
- **Keys:** `auth-access-key` and `auth-secret-key`
- **Synced from:** OpenBao path `rs-manager/ord-devimprint/armor-writer`
- **ExternalSecret:** Defined in `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`

### Access Blockers
1. **Read-only proxy:** The kubectl proxy at `http://kubectl-proxy-ord-devimprint:8001` uses serviceaccount `devpod-observer:devpod-observer` with read-only RBAC that explicitly denies secret access
2. **No kubeconfig:** No kubeconfig file exists for ord-devimprint cluster with write access
   - Checked: `/home/coding/.kube/*.kubeconfig` - only `iad-acb.kubeconfig` and `iad-ci.kubeconfig` exist
   - Ord-devimprint cluster access is only available via read-only proxy

### What Was Attempted
1. ✅ Verified secret exists: `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint` → Forbidden (expected)
2. ✅ Found ExternalSecret configuration showing the secret syncs from OpenBao
3. ✅ Located OpenBao service on rs-manager cluster (`openbao-rs-manager:8200`)
4. ❌ Cannot access OpenBao directly (no kubeconfig with secret access to rs-manager)
5. ❌ No local cache of credentials in `.env` files or documentation

## What Is Needed

To complete this task, one of the following is required:

1. **Ord-devimprint kubeconfig with write access** - A kubeconfig file that allows secret access in the devimprint namespace
2. **Alternative access path** - Direct OpenBao access with permissions to read `rs-manager/ord-devimprint/armor-writer`
3. **Cached credentials** - If credentials were previously retrieved and cached locally

## Correct Environment Variable Names

Based on the ExternalSecret configuration:
- Secret key: `auth-access-key` → should map to `LITESTREAM_ACCESS_KEY_ID`
- Secret key: `auth-secret-key` → should map to `LITESTREAM_SECRET_ACCESS_KEY`

The job manifest in `/home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml` shows the correct mapping.

## References

- ExternalSecret config: `/home/coding/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`
- Cluster secret setup: `/home/coding/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml`
- Usage example: `/home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml`
