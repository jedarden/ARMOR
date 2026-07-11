# BF-VWTPR Status: RBAC Blocker Prevents Decode

## Date: 2026-07-11

## Issue
The child bead `bf-vwtpr` (Decode and validate LITESTREAM_ACCESS_KEY_ID) cannot be completed because its prerequisite was not met.

## Root Cause
The previous child bead (retrieve base64 value) failed due to RBAC restrictions:
- The `devpod-observer` ServiceAccount in the `devpod-observer` namespace does not have permissions to read secrets in the `devimprint` namespace
- The kubectl-proxy for `ord-devimprint` explicitly blocks secret access, even for `get` operations
- Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"`

## What Was Found
The file `/tmp/litestream_key_id.b64` exists but contains an error message instead of base64 content:
```
RBAC BLOCKER: Cannot retrieve secret value

Error from server (Forbidden): secrets "armor-writer" is forbidden
```

## Resolution Path
To complete this bead, one of the following approaches is needed:

1. **Use a cluster with proper permissions**: Access the secret via a cluster where the observer SA has secret read permissions (e.g., `ardenone-manager` with direct kubeconfig)

2. **Use port-forward**: If the pod can be accessed, use `kubectl port-forward` to access Litestream metrics/health endpoints directly

3. **Request secret value from user**: The user with cluster-admin access can manually retrieve the secret value

## Current State
The bead cannot be closed because the acceptance criteria cannot be met without a valid base64-encoded secret value to decode.

**Status**: BLOCKED on RBAC permissions
