# ADR-012: Authorization — Action Verbs, Identity Audit, and Enforced Consumer Separation

**Status:** Proposed
**Date:** 2026-08-07

## Context

### What exists (and is invisible)

ARMOR already implements multi-user authentication with per-credential prefix
ACLs, and has since the named-credential loader landed:

- `internal/config/config.go` — `Credentials map[string]*Credential`;
  `loadNamedCredentials` reads `ARMOR_AUTH_<NAME>_ACCESS_KEY`,
  `ARMOR_AUTH_<NAME>_SECRET_KEY`, `ARMOR_AUTH_<NAME>_ACL` for any number of
  named credentials alongside the default pair.
- `internal/server/auth.go` — `NewSigV4AuthWithCredentials` verifies SigV4
  per-credential (lookup by access key ID); `CheckACL(cred, bucket, key)`
  enforces `ACLEntry{Bucket, Prefix}` matches, with empty ACLs meaning full
  access.
- `internal/server/server.go` — router-level enforcement on every request,
  including a deliberate fallback for list operations (no key in the URL →
  check the `?prefix` query param), which correctly denies broad listings to
  prefix-scoped credentials.

None of this appears in the README, and the plan's configuration reference
documents only the default pair. The feature was nearly re-designed from
scratch during the 2026-08-07 CI-cache work because it could not be found by
reading the docs. Undocumented capability is capability that gets rebuilt.

### What is actually deployed

Every production consumer on the `iad-ci` deployment — Forgejo (attachments,
repo/postgres backups), CNPG/barman (`queue-db`, `forgejo-postgres` base
backups), the restore-verifier — authenticates with the **same single default
credential**, whose empty ACL grants full access to the whole bucket. The
prefixes visible in stored paths (`forgejo/postgres/`, …) are client-side
convention with zero server-side enforcement. `ARMOR_PREFIX` (ADR-001) does
not help here: it namespaces whole ARMOR *instances* within a shared bucket
and is set to `""` on iad-ci anyway; it was never a per-consumer control.

Consequences of the current state:

- Any compromised client can read, overwrite, or delete **every other
  client's data** — and its own backup history.
- A backup writer holding delete rights is precisely the property ransomware
  playbooks exploit. An append-only writer is unbuildable today.

### The structural gap: no action verbs

`CheckACL(cred, bucket, key)` has no verb parameter. `r.Method` is consulted
only inside SigV4 canonical-request construction, never in an authorization
decision. Authorization is therefore all-or-nothing per prefix: WHERE is
controllable, WHAT is not. Read access implies write and delete.

### Relationship to encryption keys (MEKs)

Named per-prefix MEKs (`NamedKeys`, "multi-key routing") are orthogonal:
clients never touch MEKs and encryption is transparent, so MEKs contribute
nothing to access separation. They govern cryptographic blast radius
(key-leak containment, crypto-shredding, rotation granularity). Aligning
per-prefix MEKs to the same consumer boundaries is a recommended *companion*
to this ADR, not a substitute for any part of it.

### Relationship to ADR-008

ADR-008 documents that `internal/server/handlers/handlers.go` contains zero
logging calls and that the per-request log line carries only the HTTP status,
never the S3 error code. The authorization layer inherits the same blindness:
ACL denials increment a metrics counter (`acl` 403) but log no identity, no
action, no path. This ADR's audit-logging decision deliberately complements
ADR-008 (identity + decision fields) rather than duplicating its error-code
observability scope.

## Decision

1. **Deploy what exists (P0, zero code).** Split every consumer of the
   `iad-ci` deployment onto its own named credential with a prefix ACL
   (`forgejo/`, CNPG's backup prefix, a `ci-cache/` credential for the CI
   cache, a restore-verifier credential). Rollout is entirely
   declarative-config env changes plus OpenBao entries. This converts "any
   client can destroy everything" into "any client can destroy only its own
   prefix" before any Go changes land.

2. **Add action verbs to ACL entries.** Extend `ACLEntry` with an `Actions`
   set drawn from `{Get, Put, Delete, List}`. Every S3 operation maps to
   exactly one verb (GetObject/HeadObject → Get; PutObject, multipart
   create/upload-part/complete, CopyObject destination → Put;
   DeleteObject(s), AbortMultipartUpload → Delete; ListObjectsV2,
   ListMultipartUploads → List). `CheckACL` gains the action argument at both
   call sites. **Backward compatible:** an ACL entry with no verbs specified
   grants all verbs, so existing `_ACL` strings keep their meaning; the parse
   format extends (e.g. `bucket:prefix:get+list`) rather than changes.

3. **Append-only writer becomes the standard backup role.** Backup-writing
   credentials (CNPG, Forgejo backup paths) get `Put+List` only. A
   compromised backup client can then neither destroy nor exfiltrate its
   history. Overwrite-as-destruction is accepted residual risk in v1 (S3
   PutObject overwrites; without bucket versioning a poisoned re-upload is
   possible) — documented, revisited if B2 versioning is enabled (see
   "Operations Not Implemented").

4. **Per-request identity audit logging.** The per-request log line gains
   `access_key_id` (the *ID*, never the secret), the resolved verb, the
   object key, and the authorization outcome (`allow` / `deny-acl` /
   `deny-auth`). Denials log at Warn. Fleet log shipping (VictoriaLogs)
   turns this into a queryable audit trail with no new infrastructure.

5. **Enforcement coverage tests.** Explicit tests pinning the paths where
   ACL enforcement is subtle or currently unproven:
   - `CopyObject`: the `x-amz-copy-source` **source** key must be checked
     for Get on the source, not just Put on the destination.
   - `DeleteObjects` (batch POST): every key in the body must be checked,
     not just the URL (which carries none).
   - Multipart lifecycle: create → upload-part → complete → abort, all
     verb-checked as Put/Delete respectively.
   - ListObjects with no `?prefix` from a scoped credential: stays denied
     (pin the existing correct behavior with a test).

6. **A separate test instance, stood up before production rollout.** The
   RBAC matrix (each credential × each verb × allow/deny) must be validated
   against a live instance that is not the production backup path. Cheapest
   viable shape: a second ARMOR Deployment (`armor-test`) using the
   **existing** B2 bucket and credentials with `ARMOR_PREFIX=armor-test/`
   (ADR-001 gives instance isolation for free), a throwaway MEK, and the
   named-credential matrix under test. No new B2 bucket, no new B2 keys,
   torn down or left at near-zero cost (single small pod) after validation.

## Consequences

- The `_ACL` format grows a third segment; absent segment = all verbs, so no
  existing deployment breaks. Documentation (README auth section, plan.md
  configuration table) ships in the same phase — the current situation, where
  the feature exists but is undiscoverable, is itself a defect this ADR
  fixes.
- CheckACL stays O(entries) string prefix matching; verb sets add a bitmask.
  No measurable request-path cost.
- Coverage tests may surface real enforcement gaps (CopyObject source is
  unverified today). Any gap found is fixed in the same change as its test.
- The audit log line grows ~40 bytes per request. Identity is the access-key
  ID only — never secrets — safe for shipped logs.
- Production rollout ordering is P0 (split credentials) → verbs → roles
  tightened to append-only. Each step is a separate revertible
  declarative-config or code change; nothing requires a flag day.

## Alternatives considered

- **External authorization proxy in front of ARMOR** (OPA sidecar, nginx
  auth_request): adds a hop and a second policy language to a single-binary
  design whose value is being self-contained; rejected.
- **Migrating to MinIO for its IAM**: discards ARMOR's reason to exist
  (transparent envelope encryption over B2 with Cloudflare zero-egress
  reads); rejected.
- **Verbs-first, split-later**: inverted ordering leaves the shared-master-
  credential exposure standing while code is written. The P0 split needs no
  code and removes the worst risk immediately; rejected.
