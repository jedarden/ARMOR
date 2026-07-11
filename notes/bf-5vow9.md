# Bead bf-5vow9: Verify armor-writer secret exists

**Date:** 2026-07-11
**Cluster:** ord-devimprint
**Namespace:** devimprint

## Verification Summary

✓ **PASSED** - The `armor-writer` secret exists and is properly synced

## Evidence

### ExternalSecret Status
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get externalsecret armor-writer -n devimprint
```

**Result:**
- Name: `armor-writer`
- Status: `SecretSynced`
- Ready: `True`
- Last Sync: `2026-07-11T16:21:24Z` (31 minutes ago)
- Store: `ClusterSecretStore/openbao`
- Remote Key: `rs-manager/ord-devimprint/armor-writer`

### Secret Keys (from ExternalSecret spec)
The ExternalSecret defines these secret keys in the target Kubernetes Secret:
1. `auth-access-key` - from OpenBao property `auth-access-key`
2. `auth-secret-key` - from OpenBao property `auth-secret-key`

### Environment Variable Mapping (from job manifests)
The deployment manifests map these secret keys to environment variables:
- Secret key `auth-access-key` → `LITESTREAM_ACCESS_KEY_ID` env var
- Secret key `auth-secret-key` → `LITESTREAM_SECRET_ACCESS_KEY` env var

## Acceptance Criteria Clarification

**Original acceptance criteria requested:**
- Secret contains `LITESTREAM_ACCESS_KEY_ID` key
- Secret contains `LITESTREAM_SECRET_ACCESS_KEY` key

**Actual reality:**
The acceptance criteria conflates *environment variable names* with *secret key names*. The actual secret keys are `auth-access-key` and `auth-secret-key`, which are then mapped to the environment variables `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY` in pod specs.

## Access Limitation

**Note:** Direct verification of secret contents (`kubectl get secret armor-writer -o json`) is not possible via the read-only kubectl-proxy at `http://kubectl-proxy-ord-devimprint:8001`. The proxy's ServiceAccount (`devpod-observer`) explicitly denies access to secrets for security reasons.

**Verification method:** Relied on ExternalSecret status which shows successful sync and the secret keys defined in the ExternalSecret spec.

## Conclusion

The `armor-writer` secret exists in the `devimprint` namespace and is actively synced from OpenBao with the correct keys (`auth-access-key` and `auth-secret-key`) for litestream S3 write access.
