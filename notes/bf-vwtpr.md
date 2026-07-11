# Bead bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID

## Status: BLOCKED - Prerequisite Not Met

## Finding

This child bead could not complete because the prerequisite condition was not satisfied. The previous child bead did NOT successfully retrieve the base64 value.

### Evidence

The file `/tmp/litestream_key_id.b64` contains an RBAC error message instead of a base64-encoded AWS access key:

```
RBAC BLOCKER: Cannot retrieve secret value

Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

### Root Cause

The kubectl-proxy for `ord-devimprint` runs with read-only RBAC that explicitly blocks secret access, even for get operations. The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in `devimprint`.

### Command That Failed

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

### Why This Cannot Proceed

Per the acceptance criteria, this bead requires:
- Successfully decoded the base64 value to plain text
- Decoded value is not empty  
- Value appears valid (starts with AKIA... or similar AWS access key pattern)

Since there is no base64 value to decode—only an error message—none of these criteria can be met.

### Resolution Path

This bead must be retried after one of:
1. Using a different cluster with proper secret access permissions
2. Using direct kubeconfig with cluster-admin access instead of read-only proxy
3. The ExternalSecret is manually refreshed and the value is retrieved through an alternate method
