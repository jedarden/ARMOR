# Bead bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID

## Issue Found

The prerequisite for this bead (base64 value retrieval) was not successfully completed. The file `/tmp/litestream_key_id.b64` contains an RBAC error message instead of actual base64-encoded data.

## Root Cause

The previous attempt to retrieve the LITESTREAM_ACCESS_KEY_ID used the read-only kubectl-proxy for `ord-devimprint`:

```
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

This proxy runs with the `devpod-observer` ServiceAccount which has **read-only RBAC that explicitly blocks secret access**.

## Cluster Access Constraints

According to the environment documentation:
- `ord-devimprint` cluster only has read-only proxy access
- No read-write kubeconfig is available for this cluster
- The observer SA explicitly denies access to secrets (stricter than other clusters)

## Evidence

```bash
$ cat /tmp/litestream_key_id.b64
RBAC BLOCKER: Cannot retrieve secret value

Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

## Resolution Required

This task cannot be completed because:
1. The base64 file contains an error message, not valid data
2. There is no available kubeconfig with secret access for ord-devimprint
3. The prerequisite (successful base64 value retrieval) was not met

The parent bead (bf-2778z) and this child bead (bf-vwtpr) are blocked by RBAC constraints.
