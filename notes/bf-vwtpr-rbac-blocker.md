# Bead bf-vwtpr: RBAC Blocker Prevents Completion

**Date:** 2026-07-11
**Bead ID:** bf-vwtpr
**Task:** Decode and validate LITESTREAM_ACCESS_KEY_ID
**Status:** FAILED - RBAC blocker

## Problem Statement

The bead `bf-vwtpr` requires decoding a base64-encoded value from `/tmp/litestream_key_id.b64`. However, the file contains an RBAC error message instead of the actual base64 data, indicating the prerequisite retrieval step failed.

## Root Cause

The prerequisite bead (retrieve base64 value) attempted to access the `armor-writer` secret in the `devimprint` namespace on the `ord-devimprint` cluster using:

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

This failed with:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

## RBAC Configuration Issue

The `kubectl-proxy` for `ord-devimprint` runs with **read-only RBAC that explicitly blocks secret access**. The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in `devimprint`.

## Cluster Access Context

From CLAUDE.md:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n <namespace>
```

- Proxy runs in `devpod-observer` namespace with **read-only RBAC**
- Access is **read-only** — cannot create, delete, or modify resources
- **Secret access is explicitly denied** (stricter than other clusters' observers)
- Exposed via Tailscale operator (no Traefik on this cluster)

## Impact

The ARMOR litestream restore workflow is blocked because:
1. Cannot retrieve S3 credentials from `armor-writer` secret via kubectl-proxy
2. Cannot decode and validate credentials
3. Cannot proceed with restore operations

## Alternatives Considered

1. **Direct kubeconfig:** No read-write kubeconfig is available for `ord-devimprint` in the environment
2. **Alternative cluster:** S3 credentials would need to come from a cluster where read-write access or secret-reader permissions are available
3. **Cached credentials:** Previous beads (e.g., `bf-520v`) used cached secrets to avoid OpenBao dependency; similar approach may be needed

## Resolution Path

To proceed with ARMOR litestream restore:
1. Obtain proper credentials for `ord-devimprint` cluster with secret read access
2. OR use cached credentials from a previous successful retrieval
3. OR adjust the RBAC permissions on `devpod-observer` ServiceAccount to allow secret read (requires cluster admin access)
4. OR use an alternative cluster with looser RBAC constraints

## Acceptance Criteria NOT Met

❌ Successfully decoded the base64 value to plain text - FAILED (no valid base64 data)
❌ Decoded value is not empty - FAILED (file contains error message)
❌ Value appears valid - FAILED (cannot validate what cannot be retrieved)
❌ Value is human-readable - FAILED (file contains error message, not secret)

## Files

- `/tmp/litestream_key_id.b64` - Contains RBAC error message (723 bytes)
- `/tmp/litestream_key_id.txt` - Not created (decode failed)

## Next Steps

The bead cannot be completed until the RBAC blocker is resolved. The workflow needs to either:
- Adjust to use available access methods
- Obtain proper credentials for secret access
- Use an alternative data source for S3 credentials
