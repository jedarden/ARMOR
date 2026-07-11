# Bead bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID - FAILED

## Status: PREREQUISITE NOT MET

**Attempt 2 (2026-07-11):** Confirmed the same issue - the prerequisite child bead did not actually succeed.

This bead cannot be completed because the prerequisite child bead (retrieve base64 value) did not actually succeed.

## Root Cause

The file `/tmp/litestream_key_id.b64` does not contain a base64-encoded AWS access key. Instead, it contains error output from a failed kubectl attempt:

```
RBAC BLOCKER: Cannot retrieve secret value
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

## Issue

The kubectl-proxy for `ord-devimprint` runs with read-only RBAC that **explicitly blocks secret access**, even for get operations. The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in `devimprint`.

## Command Attempted (from previous bead)

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

## Resolution Path

This bead must be re-attempted after resolving the secret access issue:

1. **Option A:** Use direct kubeconfig access to ord-devimprint with appropriate secret read permissions
2. **Option B:** Access the secret through a different cluster that has proper permissions
3. **Option C:** Use a different method to retrieve the Litestream credentials

## Conclusion

This is a dependency chain blocker - the prerequisite bead appeared to complete (it wrote to the file), but the actual secret retrieval failed due to RBAC restrictions.

**Confirmed on second attempt:** The base64 file still contains only the RBAC error message, not the actual secret value.

The bead should be released for retry after resolving the secret access issue. The root blocker must be addressed first before this decoding/validation task can proceed.
