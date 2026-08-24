# Named Credential Provisioning Specification — iad-ci ARMOR

**Purpose:** ADR-012 Decision 1 implementation — provision four named credentials for iad-ci ARMOR consumers

**Status:** Ready for operator execution (2026-08-24)

## Context

Every production consumer on the `iad-ci` ARMOR deployment currently authenticates with the same single default credential with full bucket access. This document specifies the credential provisioning that enforces per-consumer prefix ACLs before any code changes land.

## Credential Specification

### 1. Forgejo Credential

**Consumer:** Forgejo (attachments, repo/postgres backups)
**Path convention:** `forgejo/`
**OpenBao path:** `secret/rs-manager/iad-ci/armor/forgejo`
**ACL:** `b2-armor-iad-ci:forgejo/` (all actions until ADR-012 Decision 2 adds verbs)
**Environment mapping:** 
- `ARMOR_AUTH_FORGEJO_ACCESS_KEY_ID` → access key
- `ARMOR_AUTH_FORGEJO_SECRET_ACCESS_KEY` → secret key
- `ARMOR_AUTH_FORGEJO_ACL` → `b2-armor-iad-ci:forgejo/`

### 2. CNPG Backups Credential

**Consumer:** CNPG/barman (queue-db, forgejo-postgres base backups)
**Path convention:** `cnpg-backups/`
**OpenBao path:** `secret/rs-manager/iad-ci/armor/cnpg-backups`
**ACL:** `b2-armor-iad-ci:cnpg-backups/`
**Environment mapping:**
- `ARMOR_AUTH_CNPG_BACKUPS_ACCESS_KEY_ID` → access key
- `ARMOR_AUTH_CNPG_BACKUPS_SECRET_ACCESS_KEY` → secret key
- `ARMOR_AUTH_CNPG_BACKUPS_ACL` → `b2-armor-iad-ci:cnpg-backups/`

### 3. Restore-Verifier Credential

**Consumer:** Restore-verifier (continuous verification of all backups)
**Path convention:** `restore-verifier/` (for its own state, needs read access to all prefixes)
**OpenBao path:** `secret/rs-manager/iad-ci/armor/restore-verifier`
**ACL:** Multi-prefix: `b2-armor-iad-ci:forgejo/` + `b2-armor-iad-ci:cnpg-backups/` + `b2-armor-iad-ci:ci-cache/` + `b2-armor-iad-ci:.armor/`
**Environment mapping:**
- `ARMOR_AUTH_RESTORE_VERIFIER_ACCESS_KEY_ID` → access key
- `ARMOR_AUTH_RESTORE_VERIFIER_SECRET_ACCESS_KEY` → secret key
- `ARMOR_AUTH_RESTORE_VERIFIER_ACL` → `b2-armor-iad-ci:forgejo/,cnpg-backups/,ci-cache/,.armor/` (comma-separated for multi-prefix)

**Note:** The restore-verifier needs read-only access to all consumer prefixes plus `.armor/` for metadata. Once ADR-012 Decision 2 adds action verbs, this becomes a GET-only credential.

### 4. CI Cache Credential

**Consumer:** CI cache (build cache artifacts)
**Path convention:** `ci-cache/`
**OpenBao path:** `secret/rs-manager/iad-ci/armor/ci-cache`
**ACL:** `b2-armor-iad-ci:ci-cache/`
**Environment mapping:**
- `ARMOR_AUTH_CI_CACHE_ACCESS_KEY_ID` → access key
- `ARMOR_AUTH_CI_CACHE_SECRET_ACCESS_KEY` → secret key
- `ARMOR_AUTH_CI_CACHE_ACL` → `b2-armor-iad-ci:ci-cache/`

## Credential Generation Method

Each credential requires:
- **Access Key ID:** 20-character random alphanumeric string
- **Secret Access Key:** 40-character random base64 string

Generation command (per credential):
```bash
ACCESS_KEY=$(openssl rand -base64 15 | tr -d '/+=' | cut -c1-20)
SECRET_KEY=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-40)
```

## OpenBao Provisioning Procedure

For each of the four credentials above:

```bash
# Generate the keypair
ACCESS_KEY=$(openssl rand -base64 15 | tr -d '/+=' | cut -c1-20)
SECRET_KEY=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-40)

# Write to OpenBao (rs-manager owns the secret/rs-manager/* prefix)
bao --address=http://traefik-rs-manager:8200 kv put \
  secret/rs-manager/iad-ci/armor/<consumer-name> \
  access_key_id="$ACCESS_KEY" \
  secret_access_key="$SECRET_KEY" \
  acl="<bucket>:<prefix>"

# Example for forgejo:
# bao --address=http://traefik-rs-manager:8200 kv put \
#   secret/rs-manager/iad-ci/armor/forgejo \
#   access_key_id="$ACCESS_KEY" \
#   secret_access_key="$SECRET_KEY" \
#   acl="b2-armor-iad-ci:forgejo/"
```

**Critical constraint:** Never write credential values to any file, commit, bead, doc, or log. The deliverable is the OpenBao path; values are retrieved only by the ExternalSecret operator at deployment time.

## Verification Procedure

After provisioning, verify each credential exists and has the expected structure:

```bash
# Verify each path exists
bao --address=http://traefik-rs-manager:8200 kv get secret/rs-manager/iad-ci/armor/forgejo
bao --address=http://traefik-rs-manager:8200 kv get secret/rs-manager/iad-ci/armor/cnpg-backups
bao --address=http://traefik-rs-manager:8200 kv get secret/rs-manager/iad-ci/armor/restore-verifier
bao --address=http://traefing-rs-manager:8200 kv get secret/rs-manager/iad-ci/armor/ci-cache

# Verify ACL field is present (check metadata, not the value itself)
bao --address=http://traefik-rs-manager:8200 kv metadata get secret/rs-manager/iad-ci/armor/forgejo
```

## Deliverables

The deliverable for this task is the set of OpenBao paths where credentials are stored:

1. `secret/rs-manager/iad-ci/armor/forgejo`
2. `secret/rs-manager/iad-ci/armor/cnpg-backups`
3. `secret/rs-manager/iad-ci/armor/restore-verifier`
4. `secret/rs-manager/iad-ci/armor/ci-cache`

These paths are what will be referenced in ExternalSecret resources for each consumer deployment. No credential values are ever documented or delivered — they exist only in OpenBao and are retrieved at deployment time by the ExternalSecret operator.

## Next Steps (Out of Scope for This Task)

This specification covers credential provisioning ONLY. Subsequent work (separate beads) will:

1. Create ExternalSecret resources referencing these OpenBao paths
2. Update each consumer's Deployment to use the named credential environment variables
3. Verify ACL enforcement in a test instance before production cutover
4. Implement ADR-012 Decision 2 (action verbs) to create append-only backup roles

## References

- ADR-012: Authorization — Action Verbs, Identity Audit, and Enforced Consumer Separation
- ARMOR plan.md — Configuration reference (named credentials)
- CLAUDE.md — OpenBau access guidelines (rs-manager owns `secret/rs-manager/*`)
