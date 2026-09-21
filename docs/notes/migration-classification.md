# Migration Inventory Classification

The format migration reports a per-dimension count of everything it finds in
the bucket, not just the candidate totals. Every listed, non-internal object
lands in exactly one bucket per dimension: a source category, a size class,
optionally a MEK fingerprint, and — once the walk reaches it — an outcome.
The counters live in `server.ObjectClassification`
(`internal/server/format_migration.go`) and are populated by the pure
per-object classifier `ClassifyMigrationObject`
(`internal/server/format_migration_classify.go`).

## Where the report surfaces

| Surface | Shape |
|---|---|
| Persisted state `<keyPrefix>.armor/migration-state.json` | the `classification` member of the migration state JSON |
| `POST /admin/format/migrate` (synchronous response) | the `classification` member of the result JSON |
| `GET /admin/format/migrate` (progress poll) | the `classification` member of the state JSON |
| Server log at completion | human-readable summary via `ObjectClassification.Summary()` |
| `armor migrate` completion report | human summary on stderr in every mode; with `-json` the full machine-readable report (classification included) on stdout |

`armor migrate -json` keeps stdout to the one JSON document — the server's
completion document verbatim, the same shape in synchronous and watch mode —
while in watch mode the progress lines move to stderr so the final state can
be piped or diffed.

## Source categories (the version/layout dimension)

Decided by `ClassifyMigrationObject` from raw metadata alone, in decision
order — the first matching category wins:

| JSON tag | Category | Meaning | Migrate candidate? |
|---|---|---|---|
| `non_armor` | NonARMOR | no `x-amz-meta-armor-version` header at all | no |
| `malformed` | Malformed | version header present but unparseable, or an unknown major strictly below target (neither v1 nor v2) | no |
| `v3` | AlreadyAtTarget | version at or beyond the configured target — never re-encrypted, so the layout dimension collapses (hence the `v3` tag) | no |
| `v1_single_put` | V1SinglePut | version 1, no multipart flag | yes |
| `v1_multipart` | V1Multipart | version 1, `x-amz-meta-armor-multipart == "true"` | yes |
| `v2_single_put` | V2SinglePut | version 2, no multipart flag | yes |
| `v2_multipart` | V2Multipart | version 2, `x-amz-meta-armor-multipart == "true"` | yes |
| `contradictory` | Contradictory | a v1/v2 candidate whose metadata contradicts itself (rules below) — detected and reported, never migrated | no |

The target is the run's configured migration target — the server's format
write version — never a hard-coded format number, so a future v4 target
classifies v3 objects into `v3` the same way v3 targets classify v2.

## The other dimensions

Size classes (plaintext size from the metadata when it parses and carries
one, otherwise the listing size): `size_lt_1mb`, `size_1mb_to_10mb`,
`size_10mb_to_100mb`, `size_100mb_to_1gb`, `size_1gb_to_10gb`,
`size_gt_10gb`.

Key fingerprints (`by_key_fingerprint`, omitted when empty): the 16-hex MEK
fingerprint carried by a v2-style wrapped DEK, or the label `legacy` for
v1-style wrapping (which records no MEK identity). Objects without usable
key material appear in no fingerprint bucket, so this dimension may total
less than the others.

Outcomes (owned by the walk, not the inventory pass): `outcome_processed`
(migration candidate attempted, dry runs included), `outcome_skipped`,
`outcome_failed`, `outcome_integrity_failed` (HMAC or plaintext SHA-256
mismatch).

Invariants: the source, size and outcome buckets each sum to
`ObjectClassification.Total()` (see `Balanced`); the fingerprint dimension
is the one allowed to fall short.

## Deterministic reason strings

`ClassifyMigrationObject` returns a reason alongside every category. The
reasons are byte-stable for identical input — repeated inventories render
identically — because they are composed only from a fixed, explicitly named
set of metadata keys rendered by `classifyReasonFields` in sorted key order,
never from a range over the raw metadata map (whose iteration order Go
randomizes). Each cited field renders as `key="value"` (quoted), or
`key=<unset>` when absent, so a reason also records which deciding fields
were missing. The decision prefixes are:

- `no armor version header: …`
- `unparseable armor version: …`
- `version <n> at or beyond target v<t>: …`
- `unknown armor version major <n> below target v<t>: …`
- `armor v1 single-PUT: …` / `armor v1 multipart: …` /
  `armor v2 single-PUT: …` / `armor v2 multipart: …`
- `contradictory metadata: <slug>: <fields>[; <slug>: <fields>]…`

A contradictory object reports every rule it trips, slugs sorted, so an
object matching two rules says both. The slugs:

| Slug | Contradiction |
|---|---|
| `multipart-claims-missing-sizes` | multipart flag is `true` but part-size or plaintext-size is absent, zero or unparseable |
| `multipart-fields-without-flag` | part-size present without the multipart flag |
| `v2-prefixed-dek-on-v1` | v1 object whose wrapped DEK carries the `v2:<fingerprint>:` prefix |
| `v2-dek-lacks-fingerprint-prefix` | v2+ object whose wrapped DEK lacks the `v2:<fingerprint>:` prefix |
| `nonpositive-block-size` | block-size absent, zero, negative or unparseable |
| `negative-plaintext-size` | plaintext-size below zero |
| `invalid-base64-wrapped-dek` | wrapped DEK fails the same base64 pre-validation `migrateObject` applies (shared `wrappedDEKBase64Error`, so the two cannot drift) |
| `invalid-base64-iv` | IV present but not valid base64 |
| `no-wrapped-dek-key-material` | an ARMOR version claimed with no wrapped DEK at all |

## Ownership, resumes and dry runs

The inventory pass (`countObjects`) is the single writer of the source, size
and fingerprint counters and re-derives them from the full listing on every
run — a resumed run replaces the loaded static counts instead of
accumulating on top of them. The outcome counters belong to the walk and
accumulate across resumes. A dry run classifies the whole inventory and
persists its own state; the next live run re-derives the static dimensions
and starts fresh outcomes, so dry-run counts never leak into the live report
(pinned by `TestMigrateClassificationDryRunDoesNotLeak` and
`TestMigrateClassificationDryRunPersistsStateJSON`).
