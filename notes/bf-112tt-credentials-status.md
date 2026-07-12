# BF-112TT: LITESTREAM Credentials Retrieval Status

**Task**: Retrieve and decode LITESTREAM_SECRET_ACCESS_KEY from armor-writer secret and store both credentials securely

**Date**: 2026-07-12

## Current Status

### RBAC Blockade Confirmed
The ord-devimprint cluster's read-only kubectl-proxy explicitly denies access to secrets:

```
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" 
cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### Secret Key Naming Mismatch
The task requests `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`, but the actual Kubernetes secret uses different key names:

**ExternalSecret Configuration** (`k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`):
- `auth-access-key` → maps to env var `LITESTREAM_ACCESS_KEY_ID`
- `auth-secret-key` → maps to env var `LITESTREAM_SECRET_ACCESS_KEY`

**OpenBao Source**: `rs-manager/ord-devimprint/armor-writer`
- Property: `auth-access-key`
- Property: `auth-secret-key`

### Retrieved Credentials

**ACCESS_KEY_ID** (successfully retrieved and decoded):
- Base64 encoded: `lcs18qaArvWltpK/3oSfFrqiZ/oD7bcGMNYVkW2buD0=`
- Decoded value: Available in `/tmp/litestream_access_key_id_clean.txt`

**SECRET_ACCESS_KEY** (blocked by RBAC):
- Cannot be retrieved via kubectl-proxy (read-only RBAC restriction)
- No direct kubeconfig available for ord-devimprint cluster
- Cached file `/tmp/litestream_secret_key.txt` is empty (contains only RBAC error message)

## Requirements to Complete Task

To retrieve the SECRET_ACCESS_KEY, one of the following is needed:

1. **Direct kubeconfig** with secret read access to ord-devimprint cluster
2. **RBAC policy update** to allow devpod-observer SA to read secrets in devimprint namespace
3. **OpenBao access** to retrieve directly from `rs-manager/ord-devimprint/armor-writer`
4. **Alternative verification** via production logs (as used in bf-520v)

## Cluster Access Summary

| Cluster | Access Method | Secret Read |
|---------|--------------|-------------|
| ord-devimprint | kubectl-proxy (read-only) | ❌ BLOCKED |
| ord-devimprint | Direct kubeconfig | ❌ NOT AVAILABLE |
| rs-manager | Direct kubeconfig | ✅ AVAILABLE |

## Next Steps

The SECRET_ACCESS_KEY retrieval requires elevated credentials. Options:
1. Obtain `ord-devimprint.kubeconfig` with cluster-admin or secret-reader permissions
2. Access OpenBao directly to read `rs-manager/ord-devimprint/armor-writer`
3. Request RBAC update to allow devpod-observer SA secret read access
4. Verify credentials through production logs (alternative approach used in bf-520v)

## References

- Previous RBAC blockade documentation: bf-2xkyl (2026-07-11)
- ExternalSecret configuration: `k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`
- OpenBao path: `rs-manager/ord-devimprint/armor-writer`
