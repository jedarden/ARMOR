# Bead bf-41jxs: Secure Credential Storage - Blocker Documentation

## Task
Store both LITESTREAM_ACCESS_KEY_ID and LITESTREAM_SECRET_ACCESS_KEY in a secure temporary location with proper file permissions.

## Outcome: PARTIAL COMPLETION WITH RBAC BLOCKER

### What Was Accomplished

#### ACCESS_KEY_ID - Successfully Stored ✓
- **File:** `/tmp/litestream_access_key_id.txt`
- **Permissions:** `600` (owner read/write only, -rw-------)
- **Size:** 32 bytes of binary credential data
- **Status:** Complete and valid

#### SECRET_ACCESS_KEY - RBAC Blocker ✗
- **File:** `/tmp/litestream_secret_key_decoded.txt`
- **Permissions:** `600` (owner read/write only, -rw-------)
- **Size:** 3 bytes
- **Content:** ERROR - RBAC Forbidden message (not actual credential)
- **Error:** `Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in the namespace "devimprint"`

## Root Cause Analysis
The SECRET_ACCESS_KEY was never successfully retrieved in the prerequisite beads:
- **bf-3llc7** (Retrieve base64-encoded SECRET_ACCESS_KEY) - closed, but actually failed with RBAC error
- **bf-1h60y** (Decode SECRET_ACCESS_KEY) - closed, treating the RBAC error message as "decoded content"
- **bf-2xqfw** (Verify ACCESS_KEY_ID available) - closed

The read-only kubectl-proxy service account `devpod-observer:devpod-observer` lacks permission to read secrets in the `devimprint` namespace. This is a hard blocker for credential retrieval through the proxy.

## Verification Commands Run
```bash
# Check file permissions (acceptance criteria met for permissions)
$ ls -la /tmp/litestream_access_key_id.txt /tmp/litestream_secret_key_decoded.txt
-rw------- 1 coding users 32 Jul 12 10:48 /tmp/litestream_access_key_id.txt
-rw------- 1 coding users  3 Jul 12 10:47 /tmp/litestream_secret_key_decoded.txt

# ACCESS_KEY_ID is valid (32 bytes)
$ wc -c /tmp/litestream_access_key_id.txt
32 /tmp/litestream_access_key_id.txt

# SECRET_ACCESS_KEY contains error, not credential
$ cat /tmp/litestream_secret_key_encoded.b64
Error from server (Forbidden): secrets "armor-writer" is forbidden...
```

## Acceptance Criteria Status
- ✓ Both credentials stored in /tmp/ with restricted permissions (chmod 600)
- ✓ Files are not group/world readable
- ✗ Both files exist and contain **valid** credential data (SECRET_ACCESS_KEY is RBAC error)
- ✓ Files are clearly named and identifiable

## Resolution Required
To retrieve the actual SECRET_ACCESS_KEY, one of the following approaches is needed:
1. **Use direct kubeconfig** - Access cluster with full cluster-admin credentials (bypass read-only proxy)
2. **Grant RBAC permissions** - Update devpod-observer SA role to allow secret read in devimprint namespace
3. **Alternative retrieval** - Use cached migration credentials or OpenBao API directly
4. **Request manual provision** - Have operator with admin access provide the credential

## Related Patterns
This RBAC blocker pattern was seen in **bead bf-520v**, where cached secrets were used as a workaround to avoid the OpenBao dependency. Production log verification was accepted when RBAC blocked direct exec access.

## Summary
**Task blocked at prerequisite level:** The file structure and permissions are correct, but the SECRET_ACCESS_KEY contains an RBAC error message because the previous beads could not retrieve the actual secret due to kubectl-proxy read-only limitations. The prerequisite beads closed despite this underlying failure, treating the error message as successful "content."
