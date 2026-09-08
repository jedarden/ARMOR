# ARMOR ready-frontier starvation resolution — 2026-08-28 alert, verified 2026-09-08

**Verdict: the starvation condition is resolved.** Every open bead is accounted for by the
pluck exclusions, and genuine candidates remain (`final_candidate_count` = 18 then 17 on
back-to-back reads). No visibility bug was found in the frontier itself: zero excluded
beads are held by a closed blocker, zero are stuck on a stale assignee, zero on resource
conflicts. Follow-up recommendations for the stale *manual* parks are at the end — those
are quarantine decisions, not frontier defects.

## Method

```bash
bead list --ready            # rewrites .beads/diagnostics/pluck-diagnostics.json
bead list --status open --json --limit 999999   # JSONL; small default windows miss recent beads
bead show <blocker-id>       # live status for every blocker not in the open set
```

Three reads were taken on 2026-09-08; the file is rewritten by every pluck, including
parallel workers', and two more beads closed during the verification. The classification
below is pinned to the snapshot that was fully cross-referenced (07:56Z: 62 open, all 45
exclusions classified, every blocker resolved live):

| metric | 07:40:33Z | 07:56Z (classified) | 08:05:55Z |
|---|---|---|---|
| total_open_beads | 63 | **62** | 60 |
| final_candidate_count | 18 | **17** | 15 |
| excluded | 45 | **45** | 45 |
| has_assignee | 0 | **0** | 0 |
| manually_blocked | 7 | **7** | 7 |
| has_dependencies | 38 | **38** | 38 |
| resource_conflicts | 0 | **0** | 0 |

Every read sums exactly (candidates + exclusions = total) and `final_candidate_count > 0`
in all three. The only movement across reads is beads *closing* — the frontier shrank
while staying non-empty. The alert's "60 open beads" figure dates from 2026-08-28; the
population has moved since.

## 1. has_assignee — 0 beads

No open bead carries an assignee. The historical failure mode (assigned-but-open beads
invisible to the ready frontier, 583 fleet-wide on 2026-08-16) is absent.

## 2. resource_conflicts — 0 beads

No exclusions from resource conflicts.

## 3. manually_blocked — 7 beads, gate checks

All seven carry the `cycling` + `quarantine: false-close-detected-after-N-tries` +
`verification-failed` label family: they are parked by the false-close quarantine
mechanism, not by a live external gate. Gate-by-gate:

| bead | P | title gate | live check 2026-09-08 | verdict |
|---|---|---|---|---|
| armor-a3b49aaf | P0 | armor-build publishes nothing; no image ≥0.1.1959 | registry: 0.1.1960/1962/1963/1964 all HTTP 200; only 0.1.1959 is a 404 ghost | **STALE — premise expired** |
| armor-c00b9a10 | P1 | Phase 8.11 target validation + `include` param | `-target`/`-include` flags live in `cmd/armor/cmd_migrate.go:37-38` with tests | **largely landed** |
| armor-3e6da43d | P1 | gitleaks allowlist needs a forgejo-gitea pod replacement | checkpoint-bearing commits land on origin; push backlog cleared 2026-09-05 | **STALE — premise expired** |
| armor-a041e0f1 | P1 | operator must admit 398 MB push backlog | HEAD == origin/main == 2f2292e33, 0 commits ahead | **STALE — premise expired** |
| armor-b9013d16 | P2 | run full suite "after the 5 format migration tests pass" | all 5 `TestFormatMigration*` tests PASS on a clean export of HEAD (`internal/server`) | **precondition met; the run itself still owed** |
| armor-55a1fb4c | P2 | document V1 multipart fixture V3 conversions | `docs/research/migration/v1-multipart-fixtures.md` exists (37,498 bytes, Sep 3) | **STALE — deliverable exists** |
| armor-a1be3e50 | P3 | remove `jobs:1`/`--min-chunk-size` "after v3" | removal already landed in declarative-config (`data: {}`, no matches); V3 migration chain still open fleet-wide | **edit landed; "after v3" ordering + backup/verify acceptance outstanding** |

None of the seven is a *visibility bug*: each is deliberately held and the frontier
correctly excludes it. Five of seven are held on a premise that has since expired —
see [Follow-ups](#follow-ups).

## 4. has_dependencies — 38 beads, every blocker verified live

The 38 beads are held by **47 unique blockers** across **64 dependency edges**:

- **59 edges** → blocker is open/blocked right now: a legitimate hold; the work genuinely has not happened.
- **5 edges** held by 3 `deferred` blockers (`armor-3c278621` rev 258 ×3, `armor-5e7c78a2` ×1, `armor-8b79bac5` ×1) → deferred is a deliberate park status — deliberately not done, still holding its dependents. Not closed, so this is a park, not a visibility bug.
- **0 edges** → blocker closed or dangling. (A closed blocker still excluding a child would be the classic frontier bug; there are none.)

Cross-check: no dependency-blocked bead has zero live blockers, and no candidate bead
carries a live open blocker — the exclusion and inclusion sets are consistent with the
dependency graph.

Per-bead detail (blocker status as verified):

| bead | P | blocker(s): live status |
|---|---|---|
| `armor-103a2789` | P0 | `armor-6b126fd7`=open, `armor-be6e5146`=open |
| `armor-1aad5984` | P0 | `armor-d437839e`=open |
| `armor-38c6c4e3` | P0 | `armor-d437839e`=open |
| `armor-3a3e9544` | P0 | `armor-d437839e`=open |
| `armor-3d2834d7` | P0 | `armor-bd53db4a`=open, `armor-be6e5146`=open |
| `armor-63d6f94d` | P0 | `armor-ae75292d`=open, `armor-be6e5146`=open |
| `armor-6b126fd7` | P0 | `armor-b62ca340`=open |
| `armor-7cdd37dc` | P0 | `armor-48cd8d46`=open, `armor-ae75292d`=open, `armor-e1b2d8f1`=open |
| `armor-9519807b` | P0 | `armor-d437839e`=open |
| `armor-ae75292d` | P0 | `armor-bd53db4a`=open |
| `armor-b5070729` | P0 | `armor-3c278621`=deferred, `armor-d437839e`=open |
| `armor-b62ca340` | P0 | `armor-7b39fc12`=open, `armor-d3162d1a`=open |
| `armor-bd53db4a` | P0 | `armor-6b126fd7`=open |
| `armor-be6e5146` | P0 | `armor-2c7152ae`=open |
| `armor-cb6f29ce` | P0 | `armor-f3e8dfb1`=open |
| `armor-cfd36760` | P0 | `armor-7cdd37dc`=open |
| `armor-d3162d1a` | P0 | `armor-4b6fe1b0`=open |
| `armor-d437839e` | P0 | `armor-cfd36760`=open |
| `armor-e1b2d8f1` | P0 | `armor-103a2789`=open, `armor-3d2834d7`=open, `armor-63d6f94d`=open, `armor-dc591628`=open |
| `armor-f3e8dfb1` | P0 | `armor-1aad5984`=open, `armor-38c6c4e3`=open, `armor-3a3e9544`=open, `armor-5fb79ba8`=open, `armor-9519807b`=open, `armor-b5070729`=open |
| `armor-22caa1d9` | P1 | `armor-c00b9a10`=open |
| `armor-2c7152ae` | P1 | `armor-ee6ede36`=open |
| `armor-36b04ecb` | P1 | `armor-999bd5b4`=open |
| `armor-48cd8d46` | P1 | `armor-b62ca340`=open |
| `armor-5fb79ba8` | P1 | `armor-d437839e`=open |
| `armor-796944b7` | P1 | `armor-22caa1d9`=open |
| `armor-7b39fc12` | P1 | `armor-796944b7`=open |
| `armor-999bd5b4` | P1 | `armor-f9713a40`=open |
| `armor-9f70e916` | P1 | `armor-3c278621`=deferred, `armor-5e7c78a2`=deferred, `armor-8b79bac5`=deferred, `armor-a1be3e50`=open, `armor-af1a574a`=open, `armor-b931b46d`=open, `armor-cb6f29ce`=open, `armor-d3162d1a`=open, `armor-e7efb55a`=open |
| `armor-dc591628` | P1 | `armor-48cd8d46`=open, `armor-b62ca340`=open |
| `armor-f9713a40` | P1 | `armor-3e6da43d`=open, `armor-a041e0f1`=open |
| `armor-4b6fe1b0` | P2 | `armor-b9013d16`=open |
| `armor-61906972` | P2 | `armor-edc72676`=open |
| `armor-6d90d784` | P2 | `armor-61906972`=open |
| `armor-7accbcc8` | P2 | `armor-55a1fb4c`=open |
| `armor-b931b46d` | P2 | `armor-3c278621`=deferred, `armor-e7efb55a`=open |
| `armor-edc72676` | P2 | `armor-7accbcc8`=open |
| `armor-ee6ede36` | P2 | `armor-6d90d784`=open |

## 5. The candidates

`final_candidate_count` = **17** (>0) in the classified snapshot (15 by the 08:05:55Z
rewrite). All have zero live open blockers; `armor-5e135d53` depends on
`armor-1a098fe6`, which is **Closed** (satisfied), so it is genuinely ready rather than
a leaked child:

- `armor-af1a574a` P1 — cmd/armor CLI: config/fingerprint probes and client-config golden files all broken (§8.4)
- `armor-c1f4560a` P1 — Rotate ARMOR_ADMIN_TOKEN on iad-ci after the MEK rotation walk completes
- `armor-ed088685` P1 — Fix GitHub commit-status reporting for armor-build (dead status post + sensor omits the SHA)
- `armor-34c6b40b` P2 — [Unravel] Starvation alert: beads invisible in  — Close already-shipped blocker beads via commit evi
- `armor-41d9474c` P2 — [Unravel] Starvation alert: beads invisible in  — Fix the starvation-alert emitter so the alert body
- `armor-5be5f7bd` P2 — [Unravel] Starvation alert: beads invisible in  — Fix the starvation-alert emitter in NEEDLE to emit
- `armor-5e066941` P2 — [Unravel] Starvation alert: beads invisible in  — Re-validate starvation alerts against the live rea
- `armor-5e135d53` P2 — Propagate ARMOR 0.1.1963 create-only PUT across remaining deployments
- `armor-7edd1237` P2 — Sidecar lookups in restoreverifier and armor decrypt hash the stored key — same prefix bug as armor-
- `armor-90060d9b` P2 — [Unravel] Starvation alert: beads invisible in  — Align the starvation-alert predicate with the read
- `armor-d7261d71` P2 — [Unravel] Starvation alert: beads invisible in  — Gate pluck's stale-assignee auto-repair on worker 
- `armor-da872b29` P2 — Ship per-key ACL enforcement for batch DeleteObjects (POST ?delete) — deployed gate 403s barman/lite
- `armor-ddb6ee6c` P2 — [Unravel] Starvation alert: beads invisible in  — Make starvation alerts self-verifying: counters mu
- `armor-e7efb55a` P2 — Rotate the iad-kalshi ARMOR instance to a new active MEK through the key ring
- `armor-e927e767` P2 — ALERT: Agent crash on bead armor-3c278621
- `armor-be0357e8` P3 — Implement failure record persistence to migration state
- `armor-dbcf1139` P3 — Trim ARMOR_ADMIN_TOKEN at config load: a provisioned trailing newline silently disables the admin AP

Seven of them are the `[Unravel]` starvation-alert children spawned from this very
alert — the alert pipeline reacting to itself. That is self-consistent with a healthy
frontier: the alerts fired while the frontier held 17 claimable beads, all with a clear
path to a worker.

**Accounting closes:** 62 open = 17 candidates + 38 dependency-parked + 7 manually
parked + 0 assignee-stuck + 0 conflicts. Nothing is invisible.

## Follow-ups

Recommendations only — each is someone else’s bead; nothing here was unblocked by this
verification, and closing quarantined beads is exactly what the `false-close-detected`
quarantine exists to moderate:

1. **armor-a3b49aaf (P0)** still gates the `commitgr-02891dd3` chain on "an image
   >=0.1.1959 must exist". 0.1.1960+ exist. Whoever holds the quarantine should re-arm
   or close it on the registry evidence above.
2. armor-a041e0f1 / armor-3e6da43d: push-backlog and gitleaks gates were satisfied on
   2026-09-05 (HEAD == origin/main); the parks are inert but mislabel the frontier.
3. armor-55a1fb4c: the deliverable doc exists; the park should convert to a review/close.
4. armor-b9013d16’s precondition (5 format-migration tests) is met; only the
   full-suite comparison run remains.

*Generated by bead armor-5111b670; all queries run read-only against the live bead
store, registry, Forgejo remote, and the committed HEAD tree.*
