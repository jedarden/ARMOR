# Bead bf-vwtpr: Decode and validate LITESTREAM_ACCESS_KEY_ID

## Status: FAILED - Prerequisite not met

## Finding

The file `/tmp/litestream_key_id.b64` does not contain base64-encoded data. Instead, it contains an RBAC blocker message:

```
RBAC BLOCKER: Cannot retrieve secret value

Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

## Root Cause

The previous child bead (base64 value retrieval) failed due to insufficient RBAC permissions:
- The kubectl-proxy for ord-devimprint runs with read-only RBAC
- The ServiceAccount `devpod-observer` explicitly blocks secret access
- Even get operations on secrets are forbidden

## Impact

Cannot proceed with decoding and validation because there is no base64 data to decode.

## Resolution Path

The bead workflow requires resolving the RBAC blocker before the secret can be retrieved and decoded. Possible approaches:
1. Use a kubeconfig with higher privileges (not the read-only proxy)
2. Have someone with appropriate access retrieve the secret value
3. Use ExternalSecrets to sync the secret to a location accessible by current credentials

## Commands Attempted

```bash
# Attempted decode
base64 -d /tmp/litestream_key_id.b64 > /tmp/litestream_key_id.txt
# Result: base64: invalid input
```

## Verification

Confirmed the file contains only error text, not base64 data.
