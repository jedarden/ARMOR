# Task bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID

## Status: BLOCKED by RBAC

## Issue
The prerequisite task (retrieving the base64-encoded secret value) failed due to RBAC permissions on the `ord-devimprint` cluster.

### RBAC Blocker Details

The kubectl-proxy for `ord-devimprint` runs with read-only RBAC that **explicitly blocks secret access**:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

The ServiceAccount `devpod-observer` in the `devpod-observer` namespace does not have permissions to read secrets in `devimprint`.

### Command Attempted
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
```

### Result
Access forbidden - RBAC blocker on secret access.

## Why This Cannot Proceed
The file `/tmp/litestream_key_id.b64` contains error messages, not a base64-encoded value. Therefore:
- Cannot decode the value (base64 decode fails on error text)
- Cannot validate the AWS access key format
- Task prerequisites are not met

## Workaround Options
1. Use the direct kubeconfig for `ord-devimprint` if available with elevated permissions
2. Access the secret via OpenBao directly (if the ExternalSecret is synced)
3. Use cached/migrated secrets from another cluster
4. Coordinate with cluster administrator to grant necessary permissions

## Related Issues
This RBAC limitation is consistent with previous observations documented in workspace learnings (bead `bf-520v`):
- Read-only proxy access explicitly denies secret access
- ExternalSecrets sync issues remain unresolved but may not block operations

## Date
2026-07-11
