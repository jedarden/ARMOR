# Bead bf-112tt: Retrieve and decode LITESTREAM_SECRET_ACCESS_KEY - BLOCKED

## Status: ❌ BLOCKED - Cannot Complete

## Task Summary

Retrieve the LITESTREAM_SECRET_ACCESS_KEY value from the armor-writer secret, base64-decode it, and store both credentials securely.

## Infrastructure Context

**Cluster:** ord-devimprint (Rackspace Spot)
**Secret:** armor-writer in devimprint namespace
**Secret property mapping:**
- `auth-access-key` → LITESTREAM_ACCESS_KEY_ID
- `auth-secret-key` → LITESTREAM_SECRET_ACCESS_KEY

**ExternalSecret configuration:**
```yaml
apiVersion: external-secrets.io/v1alpha1
kind: ExternalSecret
metadata:
  name: armor-writer
  namespace: devimprint
spec:
  data:
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-access-key
    secretKey: auth-access-key
  - remoteRef:
      key: rs-manager/ord-devimprint/armor-writer
      property: auth-secret-key
    secretKey: auth-secret-key
  refreshInterval: 1h
  secretStoreRef:
    kind: ClusterSecretStore
    name: openbao
```

## Blocker Details

### 1. RBAC Blocker - Cannot Access Secrets

**Read-only kubectl-proxy denies secret access:**

```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}'

# Error: (Forbidden) User "system:serviceaccount:devpod-observer:devpod-observer" 
# cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

**ServiceAccount limitations:**
- Proxy runs as `system:serviceaccount:devpod-observer:devpod-observer`
- RBAC explicitly denies secret access in devimprint namespace
- This is a permanent limitation of the read-only observer

### 2. No Cached SECRET_ACCESS_KEY Value

Unlike LITESTREAM_ACCESS_KEY_ID (which was successfully retrieved in previous beads and cached), there is **no cached SECRET_ACCESS_KEY value** available.

**Evidence:**
- No `/tmp/litestream_secret_access_key*` files exist
- No notes files contain SECRET_ACCESS_KEY values
- No bead chain successfully retrieved SECRET_ACCESS_KEY (unlike ACCESS_KEY_ID chain: bf-58r06 → bf-48qtv → bf-5xfnl → bf-1v7cv)

**Existing ACCESS_KEY_ID cache:**
- `/tmp/litestream_access_key_id.decoded` (32 bytes, binary data)
- `/tmp/litestream_access_key_id.b64` (base64-encoded)
- `/tmp/litestream_key_id.txt` (779 bytes)

### 3. No Alternative Access Methods

**Checked alternatives:**
- ❌ No kubeconfig with secret read permissions for ord-devimprint
- ❌ rs-manager cluster cannot access devimprint namespace secrets
- ❌ No OpenBao CLI available to access secret directly
- ❌ No cached values from previous successful retrievals

## Acceptance Criteria Status

All criteria **CANNOT be met**:

- ❌ **Successfully retrieved the base64-encoded SECRET_ACCESS_KEY**
  - Cannot access secret due to RBAC limitations
  - No cached value available

- ❌ **Successfully decoded it to plain text**
  - No value retrieved to decode

- ❌ **Both credentials stored in secure temporary location**
  - Only ACCESS_KEY_ID is available (already cached)
  - SECRET_ACCESS_KEY cannot be retrieved

- ❌ **Credentials NOT committed to git history**
  - Cannot retrieve SECRET_ACCESS_KEY to store

## Comparison with ACCESS_KEY_ID Success

The ACCESS_KEY_ID retrieval succeeded because:
1. Previous beads (bf-58r06 → bf-48qtv → bf-5xfnl → bf-1v7cv) successfully retrieved it
2. The value was cached and available for reuse
3. Bead bf-2778z used the cached value to complete

SECRET_ACCESS_KEY has no such successful retrieval history:
1. Multiple beads (bf-2xkyl chain) blocked on the same issue
2. No cached value exists
3. First-time retrieval requires secret access (blocked by RBAC)

## Related Beads

**Blocked by same issue:**
- bf-2xkyl: Retrieve S3 credentials from armor-writer secret - **OPEN** (blocked)
- bf-24hrg: Obtain S3 credentials for litestream restore - **OPEN** (blocked)
- bf-2c1jp: Verify armor-writer secret exists - **CLOSED** (verification only)
- bf-30sof: Set S3 credentials as environment variables - **OPEN** (blocked)

**Successful ACCESS_KEY_ID chain:**
- bf-58r06: Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID - **CLOSED**
- bf-48qtv: Validate retrieved value - **CLOSED**
- bf-5xfnl: Retrieve base64-encoded LITESTREAM_ACCESS_KEY_ID - **CLOSED**
- bf-1v7cv: Decode to plain text - **CLOSED**
- bf-2778z: Retrieve and decode LITESTREAM_ACCESS_KEY_ID - **CLOSED**

## Resolution Requirements

This bead requires one of the following to proceed:

1. **RBAC Change:** Modify `devpod-observer` role to allow secret reads in devimprint namespace
2. **Kubeconfig Provision:** Create read/write kubeconfig for ord-devimprint with secret access
3. **Cached Value:** Provide SECRET_ACCESS_KEY value from external source (OpenBao direct access)
4. **Alternative Access:** Deploy helper pod with secret read permissions to extract credentials

## Notes

- The ACCESS_KEY_ID is 32 bytes of binary data (not standard AWS format), suggesting MinIO or other S3-compatible internal format
- The SECRET_ACCESS_KEY likely has similar binary format
- Litestream works with various S3-compatible systems beyond AWS
- Both credentials are required for S3 authentication

## Status

**BEAD REMAINS OPEN** - Cannot complete without infrastructure changes or alternative access method.

Date: 2026-07-11
Bead ID: bf-112tt
Blocker: RBAC denies secret access via read-only kubectl-proxy
