# ARMOR_PREFIX Cutover and Legacy Internal-State Runbook

Operator procedure for arming `ARMOR_PREFIX` on a deployment that has already
been writing at the bucket root — the same bucket, a namespace added in
place. [ADR-001](../adr/001-bucket-prefix.md) ("Internal Namespaces") makes
the prefix rewrite every key ARMOR stores, internal bookkeeping included,
while listings and reads keep honouring the legacy root `.armor/` state a
pre-prefix era left behind. What the ADR does not give you is the sequence:
which of those states must physically move, which must not, what to do about
in-flight uploads, and how to undo it. This runbook is that sequence. One run
covers one deployment; a fleet repeats it per deployment.

This is the **same-bucket** cutover. Moving a tenant from its *own bucket*
into the shared one is the
[Unified Bucket Tenant Onboarding](unified-bucket-tenant-onboarding.md)
procedure (§7 there) — its data move and pause-window discipline apply here
too, but its source is a different bucket, not the bucket root.

The prefix is transparent to clients: the keys consumers name do not change.
What changes is where ARMOR stores everything — and the prefix is applied
unconditionally in **both** directions. After arming, a client key `X`
resolves to the stored key `<prefix>X`, and **no read path probes the
unprefixed location**. That asymmetry is the whole procedure: arming the
prefix and moving the data are one rollout, not two steps.

The behavior this runbook relies on is pinned by
`TestPrefixCutoverLegacyObjectReadsAfterDataMove`,
`TestPrefixCutoverMultipartObjectAndSidecarAcrossCutover`, and
`TestPrefixCutoverPostPrefixWritesAndBothEraListings`
(`internal/server/handlers/prefix_cutover_test.go`), which walk this exact
sequence — write pre-prefix, arm, observe the gap, move, verify both eras —
against one shared store.

## 0. Hard gates — read before arming anything

1. **Between arming and the data move, pre-prefix objects do not exist.**
   A GET of a legacy key through the armed deployment returns `NoSuchKey`
   until its stored bytes sit at `<prefix><key>`. The object is still in the
   bucket — nothing is deleted — but it is unreachable by its client name.
   The pause window (§4) must span both the config change and the final
   delta; never arm the prefix and resume writers with the move unfinished.
2. **A multipart object's stored state is three pieces, and the move must
   take all three** (§2). Moving the ciphertext without its
   `<stored-key>.armor-manifest` geometry sidecar produces an object that
   reads with HTTP 200 and **wrong bytes** — verified, not assumed; it is
   the exact failure the second cutover test pins. There is no error, no
   log line, and no HMAC failure: the read silently re-derives part
   geometry and decrypts garbage. Validate with byte comparisons or
   SHA-256, never with status codes.
3. **Do not move the manifest or the provenance chain** (§2). Both are
   correct where they are: the prefixed deployment deliberately never
   ingests root manifest deltas (ingesting them is the cross-tenant
   contamination ADR-001 describes), and the provenance chain stays at the
   bucket root by documented exception. Moving either corrupts or orphans
   it.
4. **Drain or abort in-flight multipart uploads before arming** (§3).
   Multipart upload state is resolved beneath the *current* prefix with no
   root fallback, so an upload initiated pre-cutover cannot be completed
   post-cutover.
5. **Bucket-side copies must preserve stored metadata** — the envelope and
   HMAC state travel in object metadata headers. A copy that rewrites them
   produces objects that list fine and fail verification. Same rule, same
   rationale as onboarding §7.
6. **Enable bucket versioning or take a verified backup before the move.**
   The move is a copy, and the root originals are retired only after
   validation — but the retirement is a real delete, and typos in the key
   map are permanent.

## 1. What a prefixed deployment actually does

Recap of the ADR-001 semantics this procedure depends on, with the code
that implements each:

- Every stored key — client objects and internal state alike — is
  `<prefix><key>`; the prefix is stripped from every response. Prefix
  normalization: one trailing slash, no leading slash; `T`, `T/`, `/T/` all
  mean `T/` (`internal/config` `normalizePrefix`).
- The reserved `.armor/` namespace sits at `<prefix>.armor/` when a prefix
  is in force, and listings hide it at **both** locations while a prefix is
  set (`internal/backend` `isInternalKey`: the root branch stays live
  precisely because a bucket that gained its prefix keeps pre-prefix
  internal objects at the root). Clients are refused the namespace outright
  (403 from the handler guard).
- The manifest delta walk, the HMAC sidecar probe order, and the multipart
  state location all compose onto the prefix — with the specific fallbacks
  and exceptions tabulated in §2.

## 2. What moves and what stays

The complete inventory of ARMOR state in the bucket, and its treatment:

| State | Where pre-cutover | Where post-cutover | Move? |
|---|---|---|---|
| Client object (single-part) ciphertext | `<key>` | `<prefix><key>` | **Move** — byte-preserving copy, metadata verbatim |
| Multipart object ciphertext | `<key>` | `<prefix><key>` | **Move** — same rule |
| ADR-016 part-geometry sidecar | `<key>.armor-manifest` | `<prefix><key>.armor-manifest` | **Move, rename is a prefix-prepend** — no rehash |
| ADR-003 multipart HMAC sidecar | `.armor/hmac/<sha256(key)>` | `<prefix>.armor/hmac/<sha256(<prefix>+key)>` | **Move, rename with recomputed hash** (recipe in §4) |
| Manifest snapshots + deltas | `.armor/manifest/<writer>/` | `<prefix>.armor/manifest/<writer>/` | **Do not move.** New writes land in the composed location; root deltas stay and are never ingested |
| Multipart upload state (v2) | `.armor/multipart/<id>.state` | `<prefix>.armor/multipart/<id>.state` | **Do not move — drain instead** (§3): an in-flight pre-cutover upload cannot be completed post-cutover |
| Multipart upload state (v3) | `.armor/multipart/<id>/{meta,part-*}.json` | `<prefix>.armor/multipart/<id>/…` | **Do not move — drain instead** |
| Provenance chain | `.armor/chain{,-head,-segments}/` | **unchanged — stays at the bucket root** | **Do not move.** Documented ADR-001 exception (`provenance.NewAuditorWithPrefix`); the auditor's manifest delta walk still follows the tenant prefix |
| Canary objects | `.armor/canary/` | `<prefix>.armor/canary/` | Do not move; the canary composes the new location on its own, old canaries stay hidden |
| Legacy HMAC sidecars of objects *not* being moved | `.armor/hmac/<sha256(key)>` | unchanged | Leave: `SidecarLocations` probes composed first, then the root name, so a sidecar left behind for an object that stayed put keeps working |

Two subtleties worth stating plainly:

- **The composed manifest index starts empty.** The armed deployment loads
  only `<prefix>.armor/manifest/`, so it has no index entries for
  pre-cutover objects until they are rewritten. The manifest is a
  performance optimisation — reads and listings take the backend path when
  an entry is missing, and `readyz` degrades gracefully — so this is a
  documented degradation, not an outage. Do **not** try to carry the root
  manifest over: loading it is the cross-tenant contamination ADR-001
  calls out, and it is pinned by
  `TestPrefixedLoadIgnoresPrePrefixRootManifest`
  (`internal/server/manifest_preprefix_coexistence_test.go`).
- **The ADR-016 sidecar rename is a prefix-prepend, the ADR-003 rename is
  not.** The manifest sidecar's name is the stored key plus a suffix, so
  `<key>.armor-manifest` → `<prefix><key>.armor-manifest`. The HMAC
  sidecar's name hashes `keyPrefix + clientKey`, so the hash input changes
  with the prefix and the destination name must be recomputed. Mixing the
  two recipes up is the most likely way to produce the silent-garbage
  failure in gate 2.

## 3. Pre-cutover inventory and drain

All enumeration is **bucket-side** (B2 CLI / rclone on the native remote) —
never through ARMOR, which would apply whatever prefix is configured at the
time.

1. **Choose the prefix** (§5 of the onboarding runbook has the full rules):
   a short tenant-style label written in normal form, e.g. `acme/`.
   Collision rules, in order of how badly they bite:
   - Never `.armor` or anything starting with it — that is ARMOR's reserved
     namespace, and clients are refused it outright.
   - **No legacy client-visible key may start with the prefix.** After
     arming, a new object with client key `X` is stored at `<prefix>X`; if
     any pre-prefix object already sits at that stored key (because its
     client key happened to start with the prefix), the new object shadows
     a live object with no error. List the bucket root's first path
     segment and pick a prefix absent from it.
   - One trailing slash, no leading slash; write it in normal form even
     though `ARMOR_PREFIX` normalizes.
   - Leave `ARMOR_MANIFEST_PREFIX` unset — the default `.armor/manifest` is
     relative to the prefix and composes to
     `<prefix>.armor/manifest/`, which is what a cutover wants. A
     bucket-root value is rejected at startup
     (`validateManifestPrefix`).
2. **Count both eras before touching anything** — objects at the root,
   objects already under the prefix (there should be none), root
   `.armor/hmac/` sidecars, root `*.armor-manifest` sidecars,
   `.armor/manifest/` writers. These counts are the cutover evidence's
   "before" row.
3. **Drain multipart uploads.** `ListMultipartUploads` through ARMOR (or
   the bucket's native equivalent) and let producers finish or abort
   everything in flight. There is no root fallback for upload state
   (`internal/backend/multipart.go` resolves it beneath the current prefix
   only), so anything still open at cutover becomes a permanently
   incomplete upload — abort it and have the producer retry after the
   move.
4. **`armor check` green** — the MEK and ring must unwrap every object
   before you move them, for the same reason the format-migration runbook
   gates on it.
5. **Enable bucket versioning or take a verified backup** (gate 6).

## 4. The data move

Server-side copy inside the bucket account, `rclone` with
`--server-side-across-configs` against the same remote, exactly the
mechanics of onboarding §7 — no egress, idempotent, re-runnable. The
differences from onboarding are the key map (prefix-prepend on the same
bucket, not a bucket swap) and the two rename rules:

```
1. Bulk copy      every client object <key> -> <prefix><key>
                  rclone copy --metadata --server-side-across-configs
                  (safe with writers live; re-runnable)

2. Repeat         until a pass copies near-zero objects

3. Pause writers  the only window in the procedure — and this window must
                  also cover step 6, the config change itself

4. Final delta    one more pass; nothing new can arrive at the root now

5. Sidecars       for every moved MULTIPART object, both renames:
                  a. .armor-manifest (prefix-prepend, no rehash):
                     <key>.armor-manifest          -> <prefix><key>.armor-manifest
                  b. HMAC sidecar (recomputed hash):
                     .armor/hmac/<sha256(key)>     -> <prefix>.armor/hmac/<sha256(<prefix>+key)>

6. Arm            deploy the config with ARMOR_PREFIX=<prefix> and restart
                  (§5) — inside the same pause window

7. Verify         §6, before anything resumes and before any deletion

8. Retire roots   delete the root originals — objects, and both flavors of
                  moved sidecar — only after §6 is green
```

The HMAC destination name is `sha256` over the *normalized prefix plus the
client key*, hex-encoded:

```bash
python3 - "$PREFIX" <<'EOF'
import hashlib, sys
prefix = sys.argv[1] if len(sys.argv) > 1 else ""
for client_key in sys.stdin:
    client_key = client_key.rstrip("\n")
    name = hashlib.sha256((prefix + client_key).encode()).hexdigest()
    print(f"{prefix}.armor/hmac/{name}")
EOF
# feed it the client-key list; with an empty first argument it reproduces
# the legacy root names — useful for diffing before/after inventories.
```

(Multipart objects that were *completed* pre-cutover are the only things
with sidecars; single-part objects have none, and the pre-cutover uploads
you aborted in §3 left at most orphaned state that the drain step already
resolved.)

**Retention:** leave the root copies in place until §6 passes (gate 6). The
pause ends after step 7, not after step 6 — writers resume onto a verified
namespace or not at all.

## 5. Arming the prefix

The deployment change itself is ordinary config management —
`ARMOR_PREFIX=<prefix>` on the ARMOR Deployment in `declarative-config`,
ArgoCD syncs it, the pod restarts. The ordering constraints are the
procedure:

- Arm **inside the pause window**, after the final delta and the sidecar
  renames (step 6 above). Arming early is what creates the gate-1 gap.
- The armed instance boots with an **empty manifest index** (§2) — expect
  `manifest index loaded` with `entries: 0` in the startup log, not an
  error. Root deltas are still in the bucket and are still hidden from
  listings; they are simply no longer anyone's index.
- Replication (ADR-006 secondary) keys off the same prefix — post-cutover
  writes replicate under `<prefix>…`, which is correct. If the secondary
  is another prefixed tenant of the same bucket, confirm the secondary's
  key scope covers the new namespace **before** arming, or replication
  will fail closed on every write.
- `ARMOR_BUCKET_ALIASES` is out of scope here: there is no bucket rename
  in a same-bucket cutover. If the deployment is *also* moving buckets,
  that is onboarding §9, and its alias-arming order rides on top of this
  procedure.

## 6. Validation across both eras

Every check reads through the **armed** deployment, and every check covers
both eras — the point of the cutover is that neither era can tell which
side of it it was written on. The integration test
(`internal/server/handlers/prefix_cutover_test.go`) is this list, so a
green run there is evidence the mechanics work; the operator run is
evidence this bucket's data made it.

1. **Every legacy key, byte-for-byte.** Read each pre-cutover key through
   ARMOR and compare against a pre-move SHA-256 taken in §3 — every key,
   not a sample (gate 2 is silent). The restore-verifier, where deployed,
   is the continuous version of this check and should stay green across
   the cutover.
2. **Post-cutover write round-trip.** PUT a canary-shaped object through
   ARMOR, confirm bucket-side that it landed **only** at
   `<prefix><key>` — no new root keys — then GET it back and compare.
3. **Post-cutover multipart round-trip.** Same, via a real multipart
   upload; confirm bucket-side that the sidecars landed at the composed
   names (`<prefix>.armor/hmac/<sha256(<prefix>+key)>` and
   `<prefix><key>.armor-manifest`).
4. **Listing transparency.** `ListObjectsV2` through ARMOR returns both
   eras' client keys exactly once, none carrying the prefix, and no
   `.armor/` keys from either location — the both-branches hiding rule
   (`TestClientListingsHideInternalNamespaceInBothLocations`).
5. **Manifest split.** Bucket-side: new deltas appear under
   `<prefix>.armor/manifest/<writer>/`; the root `.armor/manifest/` tree is
   byte-identical to its §3 count — untouched, per gate 3.
6. **Provenance, if enabled.** The chain continues at the bucket root; its
   manifest delta walk follows the tenant prefix and still finds the
   composed deltas (ADR-001 addendum). Tenants restricted to their own
   prefix should not have provenance enabled at all — the same rule
   onboarding §1 already applies.
7. **Retire the roots** (step 8), then re-run checks 1–4: same results,
   and the bucket-side listing now shows **nothing outside
   `<prefix>`** except the root `.armor/` trees that deliberately stay
   (manifest history, provenance chain). That residual set is the cutover
   evidence's "after" row.

## 7. Rollback — a data move, not an env flip

Unsetting `ARMOR_PREFIX` is only a complete rollback if **no post-cutover
write has happened**: with the prefix off, lookups go to the root, and any
object written under the prefix is unreachable — the mirror image of gate
1. So:

- **Before any post-cutover write** (still inside the pause window):
  unset `ARMOR_PREFIX`, restart. The root objects never moved out of
  serving position... verify with check 1 of §6 and resume. The composed
  `.armor/` tree can be ignored; it holds no data.
- **After post-cutover writes**: the reverse data move, run inside a fresh
  pause window, with the same three-piece rule mirrored:
  1. Copy every post-cutover object `<prefix><key>` → `<key>`, metadata
     verbatim.
  2. Rename sidecars back: `<prefix><key>.armor-manifest` →
     `<key>.armor-manifest` (strip the prefix), and
     `<prefix>.armor/hmac/<sha256(<prefix>+key)>` →
     `.armor/hmac/<sha256(key)>` (the empty-prefix invocation of the §4
     recipe reproduces the legacy names).
  3. Final delta, then unset `ARMOR_PREFIX` and restart — one step, the
     same coupling gate 1 imposes forward.
  4. Validate (§6 mirrored: every post-cutover key round-trips, listings
     show one era again), then retire the `<prefix>` copies.

The manifest state does **not** merge on rollback, and that is safe in
both directions: the root manifest still holds every pre-cutover entry (it
was never touched), so the rolled-back deployment resumes with its old
index and records the returned objects as they are rewritten. The
`<prefix>.armor/manifest/` deltas are simply abandoned — dead history, not
a diverging index. Symmetrically, re-attempting the cutover later starts
from §3 again with fresh counts; the abandoned composed tree gets no
second life.

## 8. Where this fits

- **Onboarding** ([unified-bucket-tenant-onboarding.md](unified-bucket-tenant-onboarding.md))
  — different scenario (cross-bucket), shared mechanics (server-side
  `--metadata` copies, pause window, verification by property).
- **Format migration** ([format-migration.md](format-migration.md)) — run
  a pending V1/V2 → V3 migration **after** the cutover settles, not
  during: one state transition at a time per object, and its per-bucket
  evidence assumes a stable namespace.
- **ADR-001** ([../adr/001-bucket-prefix.md](../adr/001-bucket-prefix.md))
  — the design decision and its addenda; this runbook is the operational
  sequence it defers to.

## 9. Checklist

```
[ ] Prefix chosen: normal form, not .armor*, no legacy key starts with it
[ ] ARMOR_MANIFEST_PREFIX unset (default composes correctly)
[ ] Bucket-side counts recorded: objects, root hmac sidecars,
    root .armor-manifest sidecars, manifest writers   (§3.2)
[ ] Multipart uploads drained or aborted             (§3.3)
[ ] armor check green; bucket versioning enabled     (§3.4–5)
[ ] Bulk copy + delta passes complete                (§4.1–2)
[ ] Writers paused                                   (§4.3)
[ ] Final delta complete                             (§4.4)
[ ] Sidecar renames done: prefix-prepend + rehash    (§4.5)
[ ] ARMOR_PREFIX armed and pod restarted             (§4.6 / §5)
[ ] Both-era validation green, incl. byte compares   (§6.1–5)
[ ] Root originals retired; post-retirement recheck  (§6.7)
[ ] Writers resumed                                  (§4)
[ ] Evidence filed: before/after counts, validation output
```
