# ADR-007: Restore-Verifier Discovery Reliability

**Status:** Accepted
**Date:** 2026-08-06

## Context

[ADR-004](004-continuous-restore-verification.md) built `restore-verifier` to close the second half of the 2026-06 incident: proving that backups are not just present but actually *restorable*, continuously, through two independent paths. That promise depends on a step upstream of both paths that ADR-004 didn't scrutinize: **discovery** — finding which objects to verify in the first place (`getLatestObject` and `getHistoricalSample` in `internal/restoreverifier/verifier.go`). Live investigation on 2026-08-06 against the real fleet found two independent bugs in discovery that are currently causing false "no objects found" readings across most of production, which silently defeats Phase 6's purpose on the affected buckets — not by reporting a false-positive restore, but by never attempting one at all.

Of 5 production ARMOR deployments checked, `restore-verifier` gave a false "empty" reading on **3 of 5**: `iad-kalshi` (Bug A), `iad-ci` (Bug B), and `rs-manager` (empty despite the bucket's own ARMOR proxy manifest logging 13 tracked objects at startup — root cause unconfirmed but consistent with Bug B's pattern). Only 2 of 5 gave a trustworthy signal.

### Bug A — discovery is blind to `ARMOR_PREFIX`-namespaced buckets

[ADR-001](001-bucket-prefix.md) lets one B2 bucket serve multiple ARMOR deployments under different key prefixes (`ARMOR_PREFIX`), transparently to any consumer going through the S3 API. `restore-verifier` intentionally bypasses the ARMOR S3 API to talk to the raw storage layer (that's the point of the direct path in ADR-004), which means it is responsible for replicating the proxy's prefix-handling itself — and it doesn't.

`cmd/restore-verifier/main.go`'s `BucketConfig` (`internal/restoreverifier/verifier.go`) already has a `Prefix` field, populated when an operator passes `-bucket=name,prefix,artifact_type,...` (`main.go` `bucketFlags.Set`, parsing `parts[1]` into `cfg.Prefix`). But that field is **never read anywhere else in the package** — both call sites that list objects, `getLatestObject` (`v.backend.List(ctx, bucket, "", "", "", 100)`, verifier.go ~line 1235) and `getHistoricalSample` (`v.backend.List(ctx, bucket, "", "", continuationToken, 1000)`, verifier.go ~line 1299), hardcode an empty prefix regardless of what `BucketConfig.Prefix` holds. It is dead configuration — plumbed in on parse, discarded before use.

Separately, and more commonly hit in practice: every currently-deployed instance uses the env-driven fallback path (confirmed via its own log line, `"No -bucket flags; verifying ARMOR_BUCKET=\"kalshi-tape\" from environment"`), which is taken whenever no `-bucket` flags are passed — the normal case, since every cluster already sets `ARMOR_BUCKET` for the main proxy and the restore-verifier Deployment reuses it. That fallback (`main.go`, the `if len(bucketFlag) == 0` branch) constructs a `BucketConfig{Bucket: envBucket, ...}` and never reads `ARMOR_PREFIX` at all — unlike the main proxy, whose `internal/config/config.go` reads and normalizes `ARMOR_PREFIX` on every startup (`cfg.Prefix = normalizePrefix(os.Getenv("ARMOR_PREFIX"))`). So even an operator who knows about the `-bucket` flag's prefix field and wants to use it must abandon the env-driven convention the rest of the fleet relies on.

Net effect: there is currently **no path** by which a running `restore-verifier` instance actually lists a prefixed subtree. Confirmed live on `iad-kalshi`: the ARMOR proxy there runs with `ARMOR_PREFIX: "iad-kalshi/"` (`armor-configmap.yml`) and is actively, successfully writing hourly (confirmed via the application's own upload-success logs), but `restore-verifier` reports "no objects found" every 6-hour cycle because it lists the bucket root with no prefix filter and finds nothing under the paths it's actually looking at (which are not where `iad-kalshi`'s objects live under a shared, prefixed bucket).

### Bug B — unpaginated latest-object listing gets swamped by ARMOR's own bookkeeping objects

`getLatestObject` (`internal/restoreverifier/verifier.go`, ~lines 1233-1268) makes a single unpaginated `List(..., maxKeys=100)` call and filters `.armor/`-prefixed internal objects (canary results, chain-head, chain audit entries, manifest snapshots/deltas — see plan.md Phase 4 and "Key Features #6/#7") out of that one page after the fact. `.armor/*` object keys sort alphabetically before ordinary object keys (`.` < most other leading characters), and they accumulate monotonically over the life of every bucket — canary runs every 5 minutes, hourly multipart canaries, provenance chain entries on every write. Confirmed live on `iad-ci`: the bucket holds 26,219 real objects but also 29,303 accumulated `.armor/*` bookkeeping objects (verified via a direct B2 listing with the cluster's own `armor-secrets` credentials). The single 100-object page `getLatestObject` fetches is therefore **100% internal noise** after filtering, every cycle, and it reports "no objects found."

This is not `iad-ci`-specific or a one-off configuration problem — it is a function of bucket age. Every long-lived ARMOR bucket accumulates `.armor/*` objects monotonically and will eventually cross the point where a single 100-object alphabetically-first page contains zero real objects. It is a matter of when, not if.

Notably, `getHistoricalSample` (verifier.go ~lines 1270+) — the sibling function responsible for sampling *older* objects for the same verification run — does not have this bug: it already paginates through the entire bucket (honoring `IsTruncated`/continuation tokens) into a bounded reservoir sampler (Algorithm R), and already skips `.armor/` objects correctly within that paginated walk. Only the "verify the most recent write" half of discovery is unpaginated. Both functions share Bug A (neither passes a prefix to `List`).

### Why this undermines Phase 6

ADR-004's whole premise is that object presence and canary health are insufficient — that "verified" must mean a completed restore with content assertions. A `restore-verifier` that never gets past discovery cannot make that determination at all: it isn't failing a restore, it's never finding anything to restore. On the affected clusters, Phase 6 is currently a no-op that happens to still emit "healthy"-shaped absence-of-failure metrics (no failed restore was recorded, because none was attempted), which is arguably worse than an honest failure signal for anyone reading `armor_last_verified_restore_timestamp` or the Grafana dashboard without also checking `armor_verified_object_ratio` against a known nonzero object count.

## Decision

Fix discovery so `restore-verifier` is a valid client of every bucket topology ARMOR itself supports (plain and `ARMOR_PREFIX`-namespaced), and so listing survives the bookkeeping-object volume every ARMOR bucket accumulates over time. This ADR does not prescribe exact code changes — it records the problem and the shape of an acceptable fix, leaving the tactical implementation to whoever picks up the tracking beads.

**Bug A — wire prefix through discovery, end to end:**
1. `BucketConfig.Prefix` must actually reach the `List()` calls in `getLatestObject` and `getHistoricalSample` (and any future discovery call) — today it's parsed and then dropped.
2. The env-driven fallback path in `cmd/restore-verifier/main.go` must populate `Prefix` from `ARMOR_PREFIX`, using the same normalization the main proxy already implements (`internal/config/config.go`'s `normalizePrefix`), so a `restore-verifier` Deployment reusing a cluster's existing `ARMOR_BUCKET`/`ARMOR_PREFIX` ConfigMap/Secret values behaves correctly without an operator also having to hand-construct a `-bucket=name,prefix,type` flag. The explicit `-bucket` flag path (which already models prefix) should keep working for operators who want to override it or verify a bucket the env-driven proxy doesn't own.
3. Any restored objects' keys presumably need prefix-stripping symmetric to how the main proxy strips it in responses (`internal/server/handlers/handlers.go`'s prefix helpers), if downstream reporting (status endpoints, escalation bead bodies) should show consumer-facing keys rather than raw B2 keys — implementer's call whether that's in scope now or a follow-up.

**Bug B — don't let a single unpaginated page be swamped by bookkeeping noise:**
Two directions are both plausible; this ADR doesn't pick one:
1. Make `getLatestObject` paginate past `.armor/*` noise the same way `getHistoricalSample` already does — walk pages, skip `.armor/`, and track the max `LastModified` seen among real objects, bounded by a reasonable page-count cap rather than a single 100-object call. The pattern to copy already exists in the same file.
2. Push the exclusion down to `B2Backend.List()` (`internal/backend/b2.go`) so callers never see `.armor/*` objects in the first place — e.g., a server-side or client-side-but-below-the-page-cap filtering mode discovery call can opt into. This would also incidentally benefit any future discovery-style caller, not just `restore-verifier`. S3-compatible `ListObjectsV2` doesn't support a native "exclude prefix," so this would need either continued client-side filtering (just moved and made loop-aware) or a different listing strategy (e.g., a delimiter-based common-prefix walk that skips the `.armor/` common prefix entirely before descending) — implementer's call on the right primitive.

Whichever direction is taken for Bug B, it should be verified against a bucket shaped like `iad-ci`'s today (bookkeeping objects outnumbering real objects, sorting first) — a fixture or test using a mock backend with a similar key-count skew would catch a regression here, since the existing testdata fixtures (per Phase 6's assertion tests) are not shaped this way.

## Evidence (2026-08-06 live fleet check)

| Deployment | Result | Root cause |
|---|---|---|
| `iad-kalshi` | False "no objects found" every 6h cycle | Bug A — `ARMOR_PREFIX: "iad-kalshi/"` set on the ARMOR proxy (`armor-configmap.yml`), proxy actively writing hourly (confirmed via app upload-success logs), but `restore-verifier` lists the bucket root unprefixed |
| `iad-ci` | False "no objects found" every cycle | Bug B — 26,219 real objects vs. 29,303 `.armor/*` bookkeeping objects (confirmed via direct B2 listing with cluster's `armor-secrets` credentials); `.armor/*` sorts first and fills the entire unpaginated 100-object page |
| `rs-manager` | False (or at least unconfirmed) "no objects found" | ARMOR proxy manifest logs 13 tracked objects at startup; `restore-verifier` still reports empty. Root cause unconfirmed but consistent with Bug B's pattern (a small object count sitting behind even a modest `.armor/*` accumulation would exhibit the same symptom) |
| (2 other deployments checked) | Trustworthy signal | Not investigated further as part of this ADR — out of scope |

Net: 3 of 5 checked deployments currently give a false-negative discovery result, undermining Phase 6's fleet-wide restorability guarantee on a majority of production ARMOR buckets.

## Alternatives Considered

**Leave discovery as-is; rely on the historical sampler alone.** Rejected — `getHistoricalSample` shares Bug A (no prefix passed) and is a probabilistic sample, not a substitute for "verify the most recent backup," which ADR-004 explicitly calls out as one of the two things every cycle must check (`getLatestObject`'s entire reason to exist). Bug A affects both functions identically regardless.

**Have restore-verifier go through the ARMOR S3 API instead of hitting B2 directly, inheriting the proxy's prefix handling for free.** Rejected — this is exactly the design ADR-004 rejected for the read-path check: the verifier must not share fate with the thing it verifies, and one of its two paths (the direct-to-ciphertext path) exists specifically to prove restorability with no ARMOR server in the loop. Bypassing B2 discovery through the proxy would reintroduce that coupling for discovery even though the two verification paths correctly avoid it for reads.

**Treat this as a documentation/deployment problem — require every restore-verifier Deployment to pass an explicit `-bucket=name,prefix,type` flag.** Rejected as the sole fix — it doesn't address Bug B at all, and it puts the burden of staying in sync with each cluster's `ARMOR_PREFIX` on a human remembering to update a second place (the restore-verifier Deployment manifest) every time a proxy's prefix changes, which is precisely the kind of drift ADR-001's "transparent to consumers" design was meant to avoid. The `-bucket` flag remains useful as an override, not as the only correct path.

## Consequences

- No change to the encryption/HMAC/provenance design, or to ADR-004's dual-path architecture — this is entirely a fix to what discovery feeds into paths that are otherwise sound.
- Until fixed, any dashboard or alert reading `armor_last_verified_restore_timestamp` / `armor_verified_object_ratio` for `iad-kalshi`, `iad-ci`, or `rs-manager` should be treated as **not proving anything** — a stale or zero-ratio reading on those buckets is currently indistinguishable from "verifier never found anything to check," not "verification failed." This ADR doesn't change that metrics gap; it's worth noting `armor_verified_object_ratio` alone can't currently distinguish "0 known-good objects verified because there are 0 real objects" from "0 verified because discovery is broken" — a future improvement (out of scope here) could have discovery report a distinct "objects seen but none passed filtering" signal.
- Fixing Bug A only helps prefixed buckets; fixing Bug B only helps buckets old enough to have accumulated enough `.armor/*` volume. Both need fixing — `iad-kalshi` would still fail after a Bug-B-only fix (it isn't the bookkeeping-volume case), and `iad-ci` would still fail after a Bug-A-only fix (it isn't prefixed).
- Once fixed, expect `iad-kalshi` and `iad-ci` restore-verifier instances to immediately start reporting real (possibly non-passing) verification results for the first time — any newly-surfaced failures are not new corruption, they're prior-hidden findings, and should be triaged with that context rather than treated as regressions introduced by this fix.

Related: [ADR-001](001-bucket-prefix.md), [ADR-004](004-continuous-restore-verification.md), plan.md Phase 6.
