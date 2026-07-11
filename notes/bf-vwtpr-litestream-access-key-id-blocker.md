# Bead bf-vwtpr: LITESTREAM_ACCESS_KEY_ID Decode - BLOCKED

## Task
Decode and validate LITESTREAM_ACCESS_KEY_ID from base64.

## Blocker
**RBAC prevents secret access; prerequisite bead did not complete base64 retrieval**

## Investigation Summary

### What the File Actually Contains
The `/tmp/litestream_key_id.b64` file does NOT contain base64-encoded data. Instead, it contains:
```
RBAC BLOCKER: Cannot retrieve secret value

Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secret"
```

### Prerequisites Were NOT Met
The bead description states:
> **Prerequisites**: Previous child bead complete (base64 value retrieved)

However, this prerequisite was **NOT actually completed**:
- The previous bead failed to retrieve the secret due to RBAC
- The file only contains an error message, not base64 data
- There is no base64 value to decode

### Attempted Decoding
```bash
$ base64 -d /tmp/litestream_key_id.b64
base64: invalid input
```

The decode fails because the file contains text error messages, not valid base64.

### Root Cause
This is the same RBAC blocker documented in `notes/bf-112tt-litestream-secret-blocker.md`:
- The read-only kubectl-proxy on `traefik-ardenone-manager:8001` explicitly denies secret access
- ServiceAccount `devpod-observer:devpod-observer` has read-only RBAC without secret permissions
- No direct kubeconfig with secret access exists for the ardenone-manager cluster

### Secret Status
The ExternalSecret `armor-writer` exists in the `armor` namespace and is synced:
- Status: Ready = True
- Reason: SecretSynced
- Last synced: 2026-07-11

However, **ExternalSecret status verification ≠ credential retrieval** - the actual secret values cannot be accessed via kubectl.

## Resolution Required
To complete this task, one of the following is needed:
1. Obtain `~/.kube/ardenone-manager.kubeconfig` with cluster-admin or secret access
2. Coordinate with cluster administrator to provide the LITESTREAM_ACCESS_KEY_ID value directly
3. Access OpenBao API directly with appropriate authentication
4. Have the prerequisite bead actually complete successfully

## Status
**BLOCKED** - Cannot decode and validate ACCESS_KEY_ID because:
- The prerequisite bead did NOT actually retrieve the base64 value
- RBAC blocks secret access via available kubectl proxies
- No kubeconfig with secret access permissions exists

## Related Beads
- bf-112tt: LITESTREAM_SECRET_ACCESS_KEY retrieval - BLOCKED (same RBAC issue)
- bf-2778z: ACCESS_KEY_ID retrieval - BLOCKED (prerequisite that failed)
