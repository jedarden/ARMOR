# ARMOR Format Migration 2026

## Overview

This document tracks the ARMOR Version 1 → Version 2 format migration across all production buckets, as mandated by [ADR-005](../adr/005-cipher-counter-reuse.md) and Plan §8.1.

## Migration Status

### iad-ci (BLOCKED - Admin Token Access)

**Status:** ⛔ BLOCKED - Admin API not accessible
**ARMOR Version:** 0.1.1933 (format-migration-capable)
**Bucket:** iad-ci
**Timestamp:** 2026-08-29T23:50:00Z

**Blocker:**
Unable to access the admin token from OpenBao. The ExternalSecret `armor-secrets` is in `SecretSyncedError` state because the secret path `rs-manager/iad-ci/armor/admin` cannot be accessed. The `ex44` AppRole policy has insufficient permissions to read or write to the required paths.

**Current State:**
- ARMOR 0.1.1933 is deployed (contains the migration endpoint)
- ExternalSecret sync failing: "could not get secret data from provider"
- Permission denied when accessing OpenBao via `bao-as rs-manager`
- Permission denied when accessing via `bao-as openbao-v2`
- Secret path `secret/rs-manager/iad-ci/armor/admin` does not exist or is inaccessible

**Investigation:**
- ClusterSecretStore uses iad-ci OpenBao instance with Kubernetes auth
- ExternalSecret remoteRef: `rs-manager/iad-ci/armor/admin`
- Multiple 403 errors when attempting to access any rs-manager or iad-ci secrets
- ex44 token shows policies: [default ex44] but appears to lack access to required paths

**Required Action:**
⚠️ **OPERATOR INTERVENTION REQUIRED**

The admin token must be provisioned in OpenBao before migration can proceed. Based on the ExternalSecret configuration, the secret should exist at path `secret/rs-manager/iad-ci/armor/admin` (or the equivalent in the iad-ci OpenBao instance accessible via `http://openbao.external-secrets.svc.cluster.local:8200`).

Options:
1. Provision the secret directly in the iad-ci OpenBao instance with the correct path structure
2. Update the ex44 policy to allow provisioning secrets at the required path
3. Use a break-glass authentication method with broader OpenBao access

**Root Cause:**
The ex44 AppRole policy appears to have very limited permissions (403 on all attempted secret paths). The ExternalSecret operator uses Kubernetes auth (role: eso) which may have different access than the ex44 AppRole.

**Migration Steps (Blocked):**
1. ✅ Verify ARMOR 0.1.1933 is deployed
2. ❌ Access admin token from OpenBao
3. ❌ Provision admin token if missing
4. ❌ POST `…/admin/format/migrate?dry_run=true` - record counts
5. ❌ POST `…/admin/format/migrate` - start migration
6. ❌ Poll GET `…/admin/format/migrate` until completion
7. ❌ Spot-check x-amz-meta-armor-version: 2 on three objects
8. ❌ Final dry-run should return candidates: 0

---

### rs-manager (BLOCKED - Admin Token Not Provisioned)

**Status:** ⛔ BLOCKED - Admin API not accessible
**ARMOR Version:** 0.1.1931 (format-migration-capable)
**Bucket:** rs-manager
**Timestamp:** 2026-08-29T22:00:00Z

**Blocker:**
The `/admin/format/migrate` endpoint is fail-closed because `ARMOR_ADMIN_TOKEN` is not set. The deployment expects this token from the `armor-secrets` Kubernetes secret (key: `admin-token`), but this key does not exist. The ExternalSecret `armor-secrets` is in `SecretSyncedError` state because the corresponding OpenBao secret does not exist.

**Current State:**
- ARMOR 0.1.1931 is deployed (contains the migration endpoint)
- Admin API returns: `admin API disabled: ARMOR_ADMIN_TOKEN not set`
- OpenBao path `secret/rs-manager/rs-manager/armor/admin` does not exist
- ExternalSecret sync failing: "could not get secret data from provider"

**Required Action:**
Provision the admin token in OpenBao at `secret/rs-manager/rs-manager/armor/admin` with key `admin_token`. This must be done before the migration can proceed. See Plan §8.6 for the complete rollout procedure.

**Migration Steps (Blocked):**
1. ✅ Verify ARMOR 0.1.1931 is deployed
2. ❌ Provision admin token in OpenBao
3. ❌ POST `…/admin/format/migrate?dry_run=true` - record counts
4. ❌ POST `…/admin/format/migrate` - start migration
5. ❌ Poll GET `…/admin/format/migrate` until completion
6. ❌ Spot-check x-amz-meta-armor-version: 2 on three objects
7. ❌ Final dry-run should return candidates: 0

## Technical Details

### Admin API Access

The ARMOR runtime image is `FROM scratch` (no shell, no curl), so admin API access requires:

```bash
kubectl proxy --port=8001 &
export ARMOR_ADMIN_TOKEN=$(bao kv get -field=admin_token secret/rs-manager/rs-manager/armor/admin)
curl -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/format/migrate?dry_run=true
```

**Note:** The admin token must never appear in command lines, logs, or transcripts. It flows only through pipes and environment variables.

### Version Checking

ARMOR's HeadObject never forwards `x-amz-meta-armor-*` headers to clients. To verify object versions:

```bash
export B2_KEY_ID=$(bao kv get -field=b2-access-key-id secret/rs-manager/backblaze/armor)
export B2_SECRET=$(bao kv get -field=b2-secret-access-key secret/rs-manager/backblaze/armor)
export B2_BUCKET=$(bao kv get -field=bucket secret/rs-manager/backblaze/armor)
export B2_REGION=$(bao kv get -field=b2-region secret/rs-manager/backblaze/armor)

aws s3api head-object \
  --endpoint-url https://s3.${B2_REGION}.backblazeb2.com \
  --bucket ${B2_BUCKET} \
  --key <object-key>
```

The response includes `x-amz-meta-armor-version`. After migration, this should be `2`.

### Post-Migration Steps

After successful migration, MEK rotation is required as an operator step (Plan §8.1). This is a separate procedure not covered by this bead.

## References

- [ADR-005: Cipher Counter Reuse](../adr/005-cipher-counter-reuse.md)
- [Plan §8.1: Cipher Correctness - Version 2 by Default](../plan/plan.md#81-cipher-correctness--version-2-by-default-p0)
- [Plan §8.6: Credentials as a Hot-Reloaded File](../plan/plan.md#86-credentials-as-a-hot-reloaded-file-p2)
