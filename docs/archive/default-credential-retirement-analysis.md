# Default ARMOR Credential Retirement Analysis

**Date:** 2026-08-27
**Task:** armor-dde1a3c2 (ADR-012 Phase 7 - Default credential retirement)
**Scope:** Infrastructure consumers only (forgejo-backup, cnpg-backups, restore-verifier, ci-cache)

## Summary

The default ARMOR credential (stored at OpenBao path `rs-manager/iad-ci/armor` with keys `AUTH_ACCESS_KEY` and `AUTH_SECRET_KEY`) cannot be fully retired because the `forgejo-gitea` application still depends on it for LFS and object storage.

## Consumers Status

### ✅ Infrastructure Consumers (Migrated to Named Credentials)

| Consumer | Named Credential | OpenBao Path | ACL Scope | Status |
|----------|------------------|-------------|-----------|--------|
| forgejo-backup | forgejo-armor-named | rs-manager/iad-ci/armor/forgejo | iad-ci:forgejo/ | ✅ Migrated |
| cnpg-backups | armor-cnpg-backups | rs-manager/iad-ci/armor/cnpg-backups | iad-ci:queue-db/cnpg/, iad-ci:forgejo/cnpg/ | ✅ Migrated |
| ci-cache | armor-ci-cache | rs-manager/iad-ci/armor/ci-cache | iad-ci:ci-cache/ | ✅ Migrated |
| restore-verifier | restore-verifier-auth | rs-manager/iad-ci/armor/restore-verifier | iad-ci:forgejo/, iad-ci:queue-db/cnpg/, iad-ci:ci-cache/ | ✅ Migrated |

### ⚠️ Application Workloads (Still Using Default Credential)

| Application | Secret | OpenBao Path | Usage | Status |
|------------|--------|-------------|-------|--------|
| forgejo-gitea | forgejo-backup-armor | rs-manager/iad-ci/armor (AUTH_ACCESS_KEY/AUTH_SECRET_KEY) | LFS storage, General storage (MINIO backend) | ⚠️ Not Migrated |

## Technical Details

### How forgejo-gitea Uses the Default Credential

The `forgejo-gitea` deployment references the `forgejo-backup-armor` Secret for these environment variables:

```yaml
- GITEA__lfs__MINIO_ACCESS_KEY_ID (from forgejo-backup-armor: auth-access-key)
- GITEA__lfs__MINIO_SECRET_ACCESS_KEY (from forgejo-backup-armor: auth-secret-key)
- GITEA__storage__MINIO_ACCESS_KEY_ID (from forgejo-backup-armor: auth-access-key)
- GITEA__storage__MINIO_SECRET_ACCESS_KEY (from forgejo-backup-armor: auth-secret-key)
```

The `forgejo-backup-armor` ExternalSecret pulls these from:
- OpenBao: `rs-manager/iad-ci/armor`
- Properties: `AUTH_ACCESS_KEY`, `AUTH_SECRET_KEY` (the default credential)

### Why This Matters

The `forgejo-backup-armor` Secret name is misleading - it suggests it's only used by the backup deployment, but it's actually used by both:
1. ~~forgejo-backup~~ (now uses `forgejo-armor-named` instead)
2. **forgejo-gitea** (still uses `forgejo-backup-armor`)

## Constraint Analysis

### Why We Cannot Fully Remove the Default Credential

1. **forgejo-gitea Dependency**: The Forgejo application requires LFS and object storage. Removing the default credential would break:
   - Git LFS push/pull operations
   - Package storage
   - Attachment storage
   - Other object storage features

2. **Secret Naming Confusion**: The `forgejo-backup-armor` Secret suggests it's backup-only, but it's actually used by the main Forgejo application. Renaming it would require updating multiple deployment references.

### Why forgejo-gitea Was Not Included

Per ADR-012, the consumer separation effort targeted **infrastructure consumers** (backup systems, verification, CI cache), not application workloads. The Forgejo application itself (forgejo-gitea) is an application workload and was not in scope for the credential migration effort.

## Recommendations

### Immediate Actions

1. **Document the Exception**: Add clear documentation that `forgejo-gitea` still uses the default credential and is exempt from the infrastructure consumer migration.

2. **Create Follow-up Task**: Create a separate bead/task for migrating `forgejo-gitea` to a named credential with scoped ACL (iad-ci:forgejo/ prefix).

3. **Update Secret Naming**: Consider renaming `forgejo-backup-armor` to `forgejo-application-armor` or `forgejo-gitea-armor` to reflect its actual usage.

### Follow-up Migration Plan

To migrate `forgejo-gitea` to a named credential:

1. Create new ExternalSecret: `forgejo-gitea-armor` (or rename existing)
2. Update OpenBao path: Keep using `rs-manager/iad-ci/armor/forgejo` (already exists for forgejo-backup)
3. Update `forgejo-application.yml` deployment to reference the new Secret
4. Verify LFS and storage operations work correctly
5. Remove old `forgejo-backup-armor` ExternalSecret

## Conclusion

The 4 infrastructure consumers specified in the task have been successfully migrated to named credentials with scoped ACLs. However, the default credential cannot be fully retired because the `forgejo-gitea` application workload still depends on it.

This is an **application workload**, not an infrastructure consumer, and should be addressed in a separate follow-up task. The infrastructure consumer migration (this task's scope) is complete.

## References

- ADR-012: Authorization — Action Verbs, Identity Audit, and Enforced Consumer Separation
- Task: armor-dde1a3c2 (Retire full-access use of the default armor credential)
- Parent: bf-12icmx (Phase 7 named-credentials work)
