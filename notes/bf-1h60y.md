# Bead bf-1h60y - Decode SECRET_ACCESS_KEY from Base64

## Issue Found: Infrastructure Access Block

The prerequisite bead bf-3llc7 cannot retrieve the secret due to fundamental infrastructure access limitations on the ord-devimprint cluster.

## Root Cause Analysis

### 1. Incorrect kubeconfig path in bf-3llc7
The command in bead bf-3llc7 specified:
```bash
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig
```
This kubeconfig file **does not exist** on this system.

### 2. No direct kubeconfig available for ord-devimprint
Unlike other clusters (ardenone-manager, rs-manager, iad-ci, iad-options), ord-devimprint has **no direct kubeconfig** file. Available kubeconfigs:
- `/home/coding/.kube/iad-acb.kubeconfig`
- `/home/coding/.kube/iad-ci.kubeconfig`

### 3. kubectl-proxy explicitly denies secrets access
When attempting to use the correct kubectl-proxy approach:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'
```

**Error:**
```
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

The devpod-observer service account has **explicitly denied access to secrets** - stricter than other clusters' observers.

## Verification

```bash
$ ls -la /tmp/litestream_secret_key_encoded.b64
-rw-r--r-- 1 coding users 0 Jul 12 10:25 /tmp/litestream_secret_key_encoded.b64

$ wc -c /tmp/litestream_secret_key_encoded.b64
0 /tmp/litestream_secret_key_encoded.b64
```

## Conclusion

**Cannot complete the decode operation** because:
1. No valid kubeconfig exists for ord-devimprint with secret access
2. The kubectl-proxy explicitly forbids secrets access (read-only RBAC)
3. Bead bf-3llc7 was incorrectly marked as closed despite failure

## Infrastructure Gap

The ord-devimprint cluster needs either:
- A direct kubeconfig file with secret access (like iad-ci, iad-options)
- Or a kubectl-proxy service account with elevated permissions to read secrets

## Action Taken

- Updated note documenting the infrastructure access block
- **NOT closing bead bf-1h60y** - task cannot be completed without infrastructure changes
