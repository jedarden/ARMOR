# Bead bf-41jxs: Litestream Credentials Storage - PARTIAL COMPLETE / BLOCKER

## Task
Store both LITESTREAM_ACCESS_KEY_ID and LITESTREAM_SECRET_ACCESS_KEY in /tmp/ with secure permissions.

## What Was Completed

### ACCESS_KEY_ID ✓
- File: `/tmp/litestream_access_key_id.txt`
- Permissions: `-rw-------` (600 - owner read/write only)
- Size: 32 bytes
- Contains: Binary credential data
- Secure: Not group/world readable ✓

### SECRET_ACCESS_KEY ✗ BLOCKED
- File: `/tmp/litestream_secret_key_decoded.txt`
- Permissions: `-rw-------` (600 - owner read/write only) ✓
- Size: 0 bytes (EMPTY) ✗
- Contains: Nothing - file is empty
- Reason: RBAC prevents secret retrieval

## Root Cause

The SECRET_ACCESS_KEY value is **not available**. The `/tmp/litestream_secret_key_encoded.b64` file contains:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

This is the same RBAC blocker documented in:
- `notes/bf-112tt-litestream-secret-blocker.md`
- `notes/bf-vwtpr-litestream-access-key-id-blocker.md`

## Prerequisite Status

The bead description states: **"Prerequisites: Child bf-2xqfw complete (confirmed ACCESS_KEY_ID is available)"**

This is **PARTIALLY CORRECT**:
- ✓ ACCESS_KEY_ID is available (stored as binary in `/tmp/litestream_access_key_id.txt`)
- ✗ SECRET_ACCESS_KEY is NOT available (retrieval blocked by RBAC)

The read-only kubectl-proxy explicitly denies secret access. No kubeconfig with secret access permissions exists for accessing the `armor-writer` secret in the `devimprint` namespace.

## Verification of Permissions

```bash
$ ls -la /tmp/litestream_access_key_id.txt /tmp/litestream_secret_key_decoded.txt
-rw------- 1 coding users 32 Jul 12 10:48 /tmp/litestream_access_key_id.txt
-rw------- 1 coding users  0 Jul 12 10:50 /tmp/litestream_secret_key_decoded.txt
```

**Permission check**: ✓ Both files have `-rw-------` (600 permissions)
**Content check**: ✗ SECRET_ACCESS_KEY file is empty

## Resolution Required

To fully complete this task, SECRET_ACCESS_KEY must be retrieved through one of:
1. Obtain `~/.kube/rs-manager.kubeconfig` or `~/.kube/ord-devimprint.kubeconfig` with secret access
2. Access OpenBao API directly with appropriate authentication
3. Coordinate with cluster administrator to provide the credential value directly

## Status

**PARTIALLY COMPLETE** - Access Key ID stored securely, Secret Key blocked by RBAC
