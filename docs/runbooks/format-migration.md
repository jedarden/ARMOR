# Format Migration Runbook (V1/V2 → V3)

Operator procedure for migrating one ARMOR deployment's stored objects from
the legacy envelope versions (v1, v2) to the current write format (v3). One
run of this runbook covers one deployment — one bucket, one `ARMOR_PREFIX`.
A fleet repeats the procedure per deployment, and each deployment accumulates
the per-bucket evidence record of §9.

Migration re-encrypts every candidate object: full decrypt (per-block HMAC
and plaintext SHA-256 verified), a fresh per-object DEK and IV, and a rewrite
in the target format. The walk runs server-side behind the admin API;
`armor migrate` is a thin client of it. Migration is **not** key rotation
(rotation only re-wraps DEKs and never rewrites object bodies) — see the
[Key Rotation Runbook](../key-rotation-runbook.md) for that procedure and for
how the two interact.

## 0. Hard gates — read before touching a live bucket

1. **Live migration destroys candidates larger than 5 MiB (known defect).**
   The multipart re-upload output path discards per-part HMAC tables and
   persists no sidecar, and the post-write read-back then fails — the object
   has already been overwritten with an unreadable body and is recorded as a
   failure. This is pinned by `TestGoldenMultipartMigratorDefect`
   (`internal/server/format_migration_golden_test.go`) and documented as
   Known Issue #4 in the
   [V3 Migration Reference](../research/migration/V3_Migration_Reference.md).
   **Gate:** from the dry-run classification (§3), if any candidate's
   plaintext is above the 5 MiB output threshold (`5 * 1024 * 1024` —
   everything in the 10 MB+ size classes, and the upper half of 1 MB–10 MB),
   **stop**: run no live migration on that bucket until the defect is fixed.
   The signal that it is fixed is that `TestGoldenMultipartMigratorDefect`
   starts failing; revisit this gate then. Dry runs stay safe (they write
   nothing).
2. **Objects are overwritten in place.** There is no staging copy, no swap,
   and no ARMOR-managed backup: a successful migration replaces the original
   ciphertext at the same key. Rollback means B2-side object versions or an
   external copy. **Enable bucket versioning (or take a verified
   backup/replica) before any live run.** What makes this safe at all is
   that writes fail closed: an object is replaced only after decrypt, HMAC
   and plaintext-SHA verification, and a successful read-back of the
   migrated object all succeeded. A failing object is left byte-identical.
3. **One migration at a time, strictly serialized.** There is no lock. A
   second POST while a run is in flight interleaves with it (a second live
   POST "resumes" from the last-saved cursor and double-processes; a
   concurrent dry-run POST overwrites the live run's state file). Never
   dry-run while a live run is in progress, and never run two walks against
   one deployment concurrently.
4. **`include` defaults to v2 only.** For a v3 target an omitted `include`
   means `v2` — v1 objects are **not** migrated unless you pass
   `include=v1,v2` explicitly. Since v1 is the version with the broken
   zero-knowledge property ([ADR-005](../adr/005-ctr-counter-stride-fix.md)),
   an accidental v2-only run is quiet and useless: pass the include list
   explicitly on every run.
5. **The MEK must cover every candidate.** Migration unwraps DEKs with the
   server's default key (plus ring fallback during unwrap). Objects wrapped
   under a fingerprint absent from the active key and ring fail at unwrap —
   permanently, they are never migrated. Run `armor check` first; its
   fingerprint probe fails when any object names an unknown fingerprint.
6. **The walk is sequential.** `-concurrency` is accepted, validated
   (1–50) and recorded, but the walk processes one object at a time. Size
   the window from the dry-run counts, not from the concurrency flag.
7. **The admin surface is the only driver and must stay private.** The
   endpoint is disabled fail-closed without `ARMOR_ADMIN_TOKEN`, and the
   admin listener (`ARMOR_ADMIN_LISTEN`, default `127.0.0.1:9001`) must not
   be exposed publicly. Point clients at the admin listener (direct, in-pod,
   or via `kubectl proxy`), never at the public S3 listener.

## 1. Preconditions (check per bucket)

- [ ] The deployment writes v3 (`ARMOR_FORMAT_VERSION=3`, the default) and
      runs a current release. The migration target must equal the
      configured write version (`-target v3` is validated against it; a
      mismatch is a 400), and a deployment configured below v2 refuses
      migration outright.
- [ ] `ARMOR_ADMIN_TOKEN` is provisioned on the deployment (without it the
      endpoint answers `403 admin API disabled`), and you can reach the
      admin listener.
- [ ] `armor check` passes: backend reachable, canary MEK unwrap OK,
      fingerprint probe OK (see the
      [CLI Command Reference](../cli-reference.md), `armor check`).
- [ ] Bucket versioning is on (or a fresh backup/verified replica exists) —
      gate 2 above.
- [ ] Multipart candidates all have their HMAC sidecar at
      `<prefix>.armor/hmac/<sha256hex(key)>` (a missing sidecar is a
      guaranteed `missing_sidecar` failure — find them in the inventory, not
      in the failure list).
- [ ] If any bucket content was written by a version emitting ADR-011
      placeholder HMACs: sweep for all-zero 32-byte sidecar entries first.
      Those objects read fine but fail migration's strict verification
      (V3 Migration Reference §8.7).

## 2. Inventory: the dry run

A dry run (`dry_run=true`) walks the whole bucket, decrypts every candidate,
verifies integrity, and writes nothing. It reports the same failure set the
live run would.

```bash
# Preferred: in-pod, no network exposure (the CLI's own POST times out at
# 120 s — see §6 before relying on it for a long walk)
kubectl exec deploy/armor -n <namespace> -- env \
  ARMOR_ADMIN_TOKEN="$(kubectl get secret -n <namespace> armor-secrets \
    -o jsonpath='{.data.admin-token}' | base64 -d)" \
  armor migrate -admin-url http://localhost:9001 \
    -dry-run -target v3 -include v1,v2 -json > dry-run.json
```

```bash
# Equivalent raw-API form (kubectl proxy); keep the bearer token in a shell
# variable, never on the command line
curl -s -X POST \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  "http://127.0.0.1:8001/api/v1/namespaces/<namespace>/services/armor:9001/proxy/admin/format/migrate?dry_run=true&include=v1,v2" \
  | jq . > dry-run.json
```

Notes:

- The run opens with a **full inventory pass** (list the bucket, then one
  HEAD per object). On a large bucket this phase is long and silent:
  `total_objects` stays 0 and GET progress shows nothing until the walk
  proper starts. Do not mistake silence for a hang.
- `total_objects` counts **candidates only** (source version in the include
  list, below target). Non-ARMOR, already-v3, malformed and contradictory
  objects are classified but never counted as candidates.
- A dry run never resumes anything and is never resumed: it always scans
  from the beginning, and its state is discarded by the next live run. It
  does, however, **overwrite `<prefix>.armor/migration-state.json`** —
  archive any prior run's state before dry-running over it.
- `GET /admin/format/migrate` returns `{"status":"no_migration"}` when no
  state exists; it also answers `no_migration` (200, not an error) when the
  state file is corrupt — do not parse that as "nothing happened".

## 3. Read the dry-run result

Judge the run by `failed_objects` and the classification, not by `status`:
the POST returns `status: "completed"` even when objects failed.

```jsonc
// classification summary (also rendered human-readable on stderr)
{
  "status": "completed",
  "total_objects": 123,          // candidates
  "processed_objects": 123,      // attempted (dry run included)
  "skipped_objects": 456,        // out of scope: non-ARMOR, v3, malformed...
  "failed_objects": 2,
  "failures": [ { "key": "...", "reason": "...", "time": "..." } ],
  "classification": { ... }
}
```

The classification is a per-dimension census of the whole bucket (see
[Migration Inventory Classification](../notes/migration-classification.md)
for the full decision rules):

| Dimension | Buckets | Operator reading |
|---|---|---|
| source/layout | `v1_single_put`, `v1_multipart`, `v2_single_put`, `v2_multipart`, `v3`, `non_armor`, `malformed`, `contradictory` | The four `v1_*`/`v2_*` counts are the candidates. `contradictory` objects are reported, never migrated — triage them by hand. `malformed` (unparseable version) fails the walk outright |
| size | `size_lt_1mb` … `size_gt_10gb` | **Hard-gate check (§0.1):** any candidate count above the 5 MiB threshold blocks the live run |
| key fingerprint | `by_key_fingerprint`, plus `legacy` for v1-style wrapping | Every fingerprint present must be covered by the active MEK or ring; `legacy` counts unwrap by trial |
| outcome | `processed`, `skipped`, `failed`, `integrity-failed` | `integrity-failed` (HMAC or plaintext-SHA mismatch) is corrupted-at-rest: restore, do not migrate |

Dry-run failures are the live run's failures. Fix causes (or restore
corrupted objects) **before** going live, and re-run the dry run until
`failed_objects` is zero or every remaining failure has a documented
disposition (§9).

## 4. The live run

Same command without `-dry-run`. Two rules from §0 decide the mechanics:

- **Explicitly pass the include list**: `-include v1,v2`.
- **Respect the 120 s client timeout.** The POST is synchronous — the
  response carries the final report — and `armor migrate` uses a 120 s HTTP
  client. A walk longer than that is cut by the *client*, the server marks
  the run `interrupted`, and the CLI exits non-zero. For anything but a
  small bucket, drive the POST with a client that has no short timeout
  (`curl`), and monitor separately (§5):

```bash
curl -s -X POST \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  "http://127.0.0.1:8001/api/v1/namespaces/<namespace>/services/armor:9001/proxy/admin/format/migrate?include=v1,v2" \
  | jq . > live-run.json
```

Expected outcomes:

- **`200` + `status: "completed"`** — the walk finished. Still check
  `failed_objects`: anything failed, the CLI's watch mode says
  `Migration had N failures.` and exits 1, and the JSON carries the full
  `failures` array.
- **`500` + `{"status":"failed", "error": ...}`** — the run aborted: a
  whole-bucket listing failed, state could not be persisted, or the request
  context was cancelled. The partial result rides along in `result`.
- **Client timeout / dropped connection** — the server marks the run
  `interrupted` and stops between pages. This is recoverable (§6).
- **`400`** — request rejected before anything ran: `target` mismatch,
  `include` version not older than the target, `concurrency` outside 1–50.
- **`403`/`401`** — admin token unset on the server / wrong token.

## 5. Monitoring

Poll the state endpoint; it answers with the full persisted state:

```bash
watch -n 30 'curl -s -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  "http://127.0.0.1:8001/api/v1/namespaces/<namespace>/services/armor:9001/proxy/admin/format/migrate" | jq "{status, processed_objects, total_objects, failed_objects, last_key}"'
```

- Progress is saved at run start, **every 100 objects**, and at completion —
  `processed_objects` moves in steps of up to 100. `last_key` is the
  lexicographic cursor; keys at or before it are already done.
- `status` reads `in_progress` → `completed`, or `interrupted` /
  `failed` on the abort paths. There is no cancel endpoint: to stop a run,
  disconnect the client (the walk stops at the next page boundary).
- `armor migrate -watch` does the same polling at a 2 s cadence after its
  POST — but see §4 on the 120 s POST timeout before choosing it.

## 6. Failure recovery

**Interruption (client disconnect, timeout, SIGTERM, pod crash).**

- A graceful cancellation saves `status: "interrupted"`; a crash leaves the
  last periodic save, `status: "in_progress"` with `last_key`.
- **A re-POST resumes only an `in_progress` state** — from `last_key`, with
  counters and failure records carried forward (cumulative across runs).
- An `interrupted` state is *not* resumed: the next POST starts a fresh run
  (new migration id, cursor at the beginning). Already-migrated objects are
  re-classified as at-target and skipped, so no work is undone — but the
  earlier run's failure records do not carry over, and the fresh walk
  re-derives everything. Judge the bucket from the newest completed run.
- Failure records reading `context canceled` (e.g.
  `failed to get metadata: context canceled`) in an interrupted run's
  tail are cancellation artifacts, not object defects. The next run gives
  the authoritative verdict for those keys.
- Already-migrated objects are never re-encrypted: on any subsequent run
  they classify as `v3` / at-target and are skipped. Re-running to
  convergence is safe and idempotent.

**Per-object failures never block the run** — the walk is best-effort: a
failing object is recorded (state `failures` array, `failed_objects` counter,
server log warning) and left byte-identical, and the walk continues. There is
no failure-count threshold that aborts a run. Failed objects are **never
retried automatically** — the cursor advances past them — and they fail again
on re-runs unless the underlying object changed.

Triage by `failures[].reason`:

| Reason (shape) | Meaning | Recovery |
|---|---|---|
| `failed to get metadata: …` | transient backend error | re-run; persistent → check backend health |
| `unparseable armor version "…": source format cannot be established…` | ARMOR-tagged object with a garbage version label | manual repair (see [Manifest Repair Quarantine](../notes/manifest-repair-quarantine.md)) or restore |
| `object … has invalid base64 in wrapped DEK/IV: …`, `invalid v2 wrapped DEK format: …` | metadata corruption, or a foreign-format object | restore from backup; never hand-edit |
| `failed to unwrap DEK: wrapped DEK must be 40 bytes` / `key unwrap failed: invalid AIV` | wrong/missing MEK for that fingerprint, or corrupt wrap | bring the fingerprint into the ring, or restore |
| `header version disagrees with metadata version: header=H metadata=M` | lying metadata; fail-closed gate added to protect the §8.1 corruption class | treat as suspect: verify with `armor verify` / restore |
| `declared part structure contradicts the derived object structure: …` | multipart metadata vs. actual object geometry | restore or repair the manifest |
| `failed to decode envelope header: invalid ARMOR magic` | corrupt/truncated single-PUT object | restore from backup (magic bytes are repairable only from a verified copy) |
| `ciphertext too short to contain HMAC table: got N, need M` | truncated object | restore from backup |
| `… HMAC verification failed` (per-block, or `HMAC table too short`) | ciphertext or sidecar corruption — data damage, not migration trouble | restore from backup; the object cannot self-repair |
| `missing_sidecar` | multipart object whose `.armor/hmac/…` sidecar is absent | restore the sidecar from a replica, or the object |
| `integrity verification failed: SHA-256 mismatch after migration: expected X, got Y` | read-back check failed — **do not trust that object** | restore the key from a pre-migration version/backup |
| `plaintext integrity check failed: …` | declared digest disagrees with decryptable content | corrupted at rest; restore |
| `version not updated: expected 3, got N` | read-back saw stale metadata | re-run; persistent → investigate backend consistency |

Deep failure-plumbing references:
[Format Migration Failure Recording Flow](../research/format-migration-failure-recording-flow.md),
[Migration Error Handling Flow](../research/migration/migration-error-handling-flow.md),
and the taxonomy in V3 Migration Reference §6.

## 7. Zero-legacy verification

No dedicated "what's left" endpoint exists — prove completion with a fresh
read-only inventory (a dry run; a non-dry POST is a real walk):

```bash
# POST dry_run=true again (fresh run, whole bucket) and check:
#   classification source:  v1-single=0 v1-multipart=0 v2-single=0 v2-multipart=0
#   total_objects: 0         (no candidates left)
#   everything that remains lands in skipped
```

Then close the loop with independent evidence:

1. **Key-ring census** — `GET /admin/key/ring`: the `legacy` bucket
   (v1-style, unfingerprinted wraps) must be 0. The default census is
   in-memory and fast; `?census=head` reads live metadata and takes hours on
   a large bucket (see the Key Rotation Runbook for the full semantics).
2. **Spot-check object metadata** — `x-amz-meta-armor-version: 3` on a
   sample of migrated objects, including at least one former multipart
   object (which, if ≤ 5 MiB, came out single-PUT with 4096-byte blocks —
   expected, see Known Issue #10 in the V3 reference).
3. **Offline audit** — `armor verify` (full mode) over the migrated range:
   every object must report `OK`. Prefer scoping with `-prefix` /
   `-keys-file` over whole-bucket sweeps, and note that a keying gap shows
   as `ERROR`, not `CORRUPTED`.
4. **Continuous verification** — the restore verifier keeps re-checking the
   bucket on a schedule ([deployment guide](../restore-verifier-deployment-guide.md),
   [ADR-004](../adr/004-continuous-restore-verification.md)).

## 8. Post-migration cleanup

- **Orphaned HMAC sidecars.** Migration never deletes the source object's
  old `.armor/hmac/<sha256hex(key)>` sidecar. After verification passes,
  sweep them to reclaim space (each migrated multipart source leaves one).
- **Stale `part-size` metadata.** Small multipart sources come out
  single-PUT with the old `part-size` still set. Harmless (nothing consumes
  it on a single-PUT object) — do not "fix" it by hand.
- **State file.** `<prefix>.armor/migration-state.json` holds the last run's
  state and is overwritten by every later run or dry run. Archive evidence
  out-of-band (§9); do not treat the in-bucket file as the record.

## 9. Per-bucket evidence record

A bucket's migration is "done" when this record exists and is consistent.
Keep it with the bucket's deployment-change bead, and file follow-up beads
for every unresolved failure disposition.

**Before (from the dry run):**

- [ ] Dry-run completion JSON (`dry-run.json`): counts, full classification,
      empty `failures` (or documented dispositions)
- [ ] Explicit statement of the 5 MiB gate decision: candidate size
      histogram, and that no candidate exceeds the threshold
- [ ] Bucket versioning / backup confirmation (what a rollback would
      restore, and up to when)
- [ ] `armor check` output (fingerprint probe OK; every `by_key_fingerprint`
      value covered by the ring)

**During:**

- [ ] Migration id, start/end timestamps, include list, target
- [ ] Monitor snapshots (§5) or the watch transcript — at minimum one
      mid-run poll proving the cursor advanced

**After:**

- [ ] Final completion JSON (`live-run.json`): `status: "completed"`,
      `failed_objects: 0` **or** the failure list with a disposition per
      failure (restored / repaired / accepted-loss, each with a follow-up
      bead reference)
- [ ] Fresh dry-run JSON showing zero candidates (§7) — the zero-legacy proof
- [ ] Key-ring census showing `legacy: 0`
- [ ] Spot-check listing (keys + `x-amz-meta-armor-version: 3`)
- [ ] `armor verify` report (exit 0) over the migrated range
- [ ] Sidecar-cleanup confirmation (or a deliberate keep, with the storage
      cost noted)

## 10. Test matrix — where migration behavior is pinned

| Area | Where | Command |
|---|---|---|
| Golden fixture migrations (V1/V2, single + multipart) | `tests/fixtures/migration/`, `internal/server/format_migration_golden_test.go` | `go test ./internal/server/ -run TestGolden -short` |
| Multipart output-path defect gate (§0.1) | `TestGoldenMultipartMigratorDefect` | same command; a failure here means the gate can be lifted |
| Dry-run isolation, resume, failure bookkeeping | `internal/server/format_migration_test.go` | `go test ./internal/server/ -run TestFormatMigration -short` |
| Classification + contradiction slugs | `internal/server/format_migration_classify_test.go`, [Migration Inventory Classification](../notes/migration-classification.md) | `go test ./internal/server/ -run TestClassif -short` |
| HTTP contract (auth, params, status codes) | `internal/server/format_migration_http_test.go`, `format_migration_validation_test.go` | `go test ./internal/server/ -run TestMigrate -short` |
| CLI flags, exit codes, JSON output | `cmd/armor/cmd_migrate_test.go`, [CLI Command Reference](../cli-reference.md) | `go test ./cmd/armor/ -run TestMigrate -short` |
| Whole-suite gate | [tests/README.md](../../tests/README.md) | `scripts/definition-of-done.sh` |
| Client compatibility after migration | [AWS CLI Compatibility](../../tests/aws-cli-compatibility/README.md), [Multipart Client-Concurrency Matrix](../multipart-client-compatibility.md) | `make compat` (CI runs it per build) |
| Large-object boundaries | [Multipart 5 GiB Boundary Tests](../testing/multipart-5gb-boundary-tests.md), integration suite | `go test -tags=integration ./tests/integration/...` (credential-gated) |

## 11. Further reading

- [V3 Migration Reference](../research/migration/V3_Migration_Reference.md) —
  the deep technical reference: format comparison, decision tree, per-fixture
  outcomes, known issues, FAQ
- [Envelope V3 Format](../format/envelope-v3.md) — the target format spec
- [Format Migration 2026](../notes/format-migration-2026.md) — per-bucket
  fleet migration status history
- [Key Rotation Runbook](../key-rotation-runbook.md) — re-wrap vs.
  re-encrypt, and rotation state the migration depends on
- [ADR-005](../adr/005-ctr-counter-stride-fix.md) (why v1 must go),
  [ADR-003](../adr/003-multipart-object-layout-and-read-path.md) /
  [ADR-015](../adr/015-out-of-order-multipart-uniform-part-size.md) /
  [ADR-016](../adr/016-multipart-metadata-finalization.md) (multipart layout
  the migration must respect)
- [Disaster Recovery](../disaster-recovery.md) — MEK escrow and
  restore-from-backup procedures referenced by the triage table
