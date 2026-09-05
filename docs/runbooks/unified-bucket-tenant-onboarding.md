# Unified Bucket Tenant Onboarding Runbook

Adding a tenant to the shared B2 bucket created for the ADR-001 consolidation,
and moving an existing tenant's objects into it.

**Audience:** whoever performs the onboarding — operator or agent. This document
is self-contained; no conversation, bead, or prior run is required to execute it.

**Status:** runbook for the consolidation described in
[ADR-001](../adr/001-bucket-prefix.md) and its naming-policy addendum. First
tenant onboarded 2026-09-04 (`needle-ledger`, the NEEDLE factory ledger); this
document generalises that run. The worked `ExternalSecret` for that tenant is
the reference implementation:
`declarative-config/k8s/apexalgo-iad/needle-observability/needle-ledger-armor-externalsecret.yml`.

---

## 0. The naming rule

The unified bucket's name is **not in git**. This is not a convention in
progress — the first shared bucket's name was published in ADR-001 on the day
it was created, which is the entire reason a replacement exists. The rule:

**Never write the name into:**

- anything under `docs/`, `tests/`, `cmd/`, `internal/` (including comments)
- a commit message
- **a bead** — `.beads/checkpoint/` is git-tracked, so a bucket name in a bead
  note, title, or close reason lands in git through the next checkpoint publish.
  This is the easiest one to get wrong: the bead is where you write status, and
  it is a git-tracked file.
- `declarative-config` — manifests carry the OpenBao path, never the value
- your own session transcript or a shell's `argv` (same mechanism, see below)

**Refer to it as "the unified bucket"** in all of the above.

**Where the name does live:**

| Location | Why it is acceptable |
|---|---|
| OpenBao, canonical record (§2) | the single authoritative copy |
| OpenBao, each tenant path (§2) | so a tenant's credentials and its bucket name travel together |
| Kubernetes `Secret` / pod env (`ARMOR_BUCKET`) | cluster state, not the repository; already secret-shaped |
| Pod logs | only as a fingerprint, never the name (§9) |

**Handling the name in a command:** never type it. Read it from OpenBao into a
shell variable so it never becomes visible text, and pass it to the tool by
reference:

```bash
BUCKET="$(bao-as rs-manager bao kv get -field=bucket \
  secret/rs-manager/rs-manager/armor/unified-bucket)"
```

Everything below assumes this pattern. A literal bucket name in a command line
lands in shell history, in `ps`, and in the transcript — the same failure the
secrets-by-reference rule exists for, applied to a name that is only slightly
less sensitive than a key.

**Variables used below**, all read from OpenBao rather than typed:

```bash
BUCKET_NAME="$(bao-as rs-manager bao kv get -field=bucket \
  secret/rs-manager/rs-manager/armor/unified-bucket)"
BUCKET_ID="$(bao-as rs-manager bao kv get -field=bucket_id \
  secret/rs-manager/rs-manager/armor/unified-bucket)"
TENANT_PATH=rs-manager/rs-manager/armor/unified-bucket/<tenant>
# SRC_REMOTE / SRC_BUCKET / DST_REMOTE / DST_BUCKET: §7's source and target.
# The target is the unified bucket; both remotes are the same B2 account.
```

---

## 1. Prerequisites

A tenant is prefix-scoped: its B2 application key can touch `<tenant>/` and nothing
else. Every ARMOR subsystem that writes must therefore compose its key with
`ARMOR_PREFIX`, or that subsystem silently 403s forever. Check all four on the
deployment you are onboarding onto **before** creating the key — three of them
were bucket-root writers and had to be fixed:

| # | Requirement | If missing | How to check |
|---|---|---|---|
| 1 | **Manifest prefix is composed with `ARMOR_PREFIX`** | the manifest write is denied by the tenant's key, and with a bucket-root path every tenant indexes every other tenant's deltas | confirm `manifest_prefix` in `GET /admin/config`-equivalent output sits under `<tenant>/`; a tenant that must ship today can set `ARMOR_MANIFEST_ENABLED=false`, but that also disables the manifest read path and the `readyz` manifest fallback |
| 2 | **Canary honours `ARMOR_PREFIX`** | the canary write is denied, so `/readyz` never reports Ready — the instance looks permanently unhealthy | roll with `ARMOR_PREFIX=<tenant>/` and confirm the canary `last_error` metric stays empty |
| 3 | **Startup log redacts the bucket name** | the name lands in pod logs and every log pipeline on every roll (§9) | `kubectl logs` a freshly rolled pod and confirm `primary backend initialized (b2)` carries a fingerprint, not the name |
| 4 | **Bucket alias** (`ARMOR_BUCKET_ALIASES`) — optional | only needed if a client's URL hardcodes the old bucket name and cannot be updated | set the old name as an alias and issue one request against it |

Requirements 1–3 are code capabilities, not configuration. If any is absent
from the deployed image, either ship it first or run the tenant on a
deployment that has it. Onboarding onto an image without them produces a tenant
that writes fine and then fails readiness in a way that reads as an infra
problem.

**Provenance is not on this list because it is not yet composable.** The
provenance chain (`.armor/chain/`, `.armor/chain-head/`, `.armor/chain-segments/`)
still writes at the bucket root, so a prefix-scoped key cannot write it and
tenants in the shared bucket would share one chain tree. **Tenants with a
prefix-scoped key must not enable provenance.** See the internal-namespaces
addendum in ADR-001.

---

## 2. OpenBao layout

All of it lives on the **rs-manager** instance (reachable from ex44 as
`bao-as rs-manager …`; it owns `secret/rs-manager/*`). Two kinds of record:

```
secret/rs-manager/rs-manager/armor/unified-bucket              canonical record
secret/rs-manager/rs-manager/armor/unified-bucket/<tenant>     one per tenant
```

The double `rs-manager` is not a typo: the first is the instance's KV prefix,
the second is the owning cluster.

**Canonical record** — written once, when the bucket was created. Properties:

| Property | Notes |
|---|---|
| `bucket` | the bucket name. The one authoritative copy. |
| `bucket_id` | B2 `bucketId` — what `b2 create-key --bucket` takes |
| `account_id` | B2 account the bucket lives in |
| `b2_region` | S3 endpoint region |
| `created` | 2026-09-03 |
| `naming_policy` | `not-published-to-git` |

**Tenant record** — one per tenant. Properties:

| Property | Notes |
|---|---|
| `bucket` | copy of the canonical name, so a tenant's credentials and its bucket travel together |
| `prefix` | `<tenant>/` |
| `b2-region` | |
| `b2-access-key-id`, `b2-secret-access-key` | the tenant's prefix-scoped B2 application key (§3) |
| `master-encryption-key` | the tenant's MEK (§4) |
| `writer-access-key`, `writer-secret-key`, `writer-acl` | the ARMOR client credential ARMOR must accept (§5) |
| `reader-access-key`, `reader-secret-key`, `reader-acl` | optional; for a reader-only instance |
| `prunes` | `true` only if `deleteFiles` was granted (§3) |
| `onboarded` | date |

**Writing:** use the provisioning identity (`bao-as rs-manager-provision`) and
never put a value in `argv` — pipe it or use `@file`. Get the current version
first, because the mount is `cas_required`:

```bash
# Do not name this variable PATH — that shadows the shell's own $PATH.
TENANT_PATH=rs-manager/rs-manager/armor/unified-bucket/<tenant>
V=$(bao-as rs-manager-provision bao kv metadata get -format=json "secret/$TENANT_PATH" \
      | jq .data.current_version)
# 0 means the path does not exist yet
```

Generate key material into the record without it ever crossing a terminal:

```bash
B2_KEY=$(openssl rand -base64 32 | tr -d '/+=' | cut -c1-40)
printf 'b2-secret-access-key: %s\n' "$B2_KEY" \
  | bao-as rs-manager-provision bao kv put -cas=$V "secret/$TENANT_PATH" -
```

**Reading:** the read identity, and never to stdout — materialise into the
consuming file or a variable.

**Verifying:** by property, not by reading back.

```bash
bao-as rs-manager-provision bao kv metadata get -format=json "secret/$TENANT_PATH" \
  | jq '.data.current_version, .data.created_time'
```

A bumped `current_version` proves the write landed. You did not need to see it.

### The per-cluster grant is exact-leaf

ESO on each cluster reaches OpenBao through a `ClusterSecretStore` using
kubernetes auth, and that auth role's policy grants **named leaves**. A glob on
`unified-bucket/` does not make `unified-bucket/<tenant>` readable — add the
tenant's leaf to the policy of every cluster that must read it, then prove the
grant with the cluster's own view rather than yours:

```bash
kubectl --server=http://traefik-<cluster>:8001 get externalsecret <name> \
  -n <namespace> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# must print True
```

`SecretSynced=True` is the property that proves the cluster's identity can read
the leaf. Do not verify by printing the value.

Concretely, per cluster that needs the tenant, add an `ExternalSecret` whose
`remoteRef.key` is the tenant path and whose `remoteRef.property` entries name
the properties above. Two `Secret`s from one tenant path is the established
shape: one for the ARMOR server (B2 credential, MEK, and the writer credential
it must accept), one for the client (writer key pair only, under the AWS SDK's
standard names, plus `bucket` for `${env:}` expansion). The client never sees
the B2 credential or the MEK; ARMOR never sees anything it does not need.

---

## 3. B2 application key

One key per tenant, scoped twice — to the bucket and to the prefix:

```bash
# namePrefix is the mechanism that makes the key tenant-local.
# Read $BUCKET_ID from the canonical record, not from memory.
# keyName is positional; capabilities follow it.
b2 create-key --bucket "$BUCKET_ID" --namePrefix "<tenant>/" \
  "<tenant>-armor" \
  listBuckets listAllBucketNames listFiles readFiles writeFiles
```

| Capability | Why |
|---|---|
| `listBuckets`, `listAllBucketNames` | ARMOR's `ListBuckets` returns bucket metadata; without these the instance's bucket listing fails |
| `listFiles` | `ListObjectsV2`, multipart part listing, and the restore-verifier's prefix walk |
| `readFiles` | downloads, `Head` |
| `writeFiles` | uploads, multipart create/upload/complete |
| `deleteFiles` | **only for tenants that prune.** Omit it otherwise. |

`deleteFiles` is the one capability that can destroy data, so it is granted on
a per-tenant decision recorded in the tenant record's `prunes` property.
Tenants that only ever append (an append-only ledger, an audit trail) do not
get it, and their record says so. Granting it "so deletes work if we ever need
them" is the mistake this row exists to prevent — a `namePrefix` limits the
blast radius to the tenant's own namespace, which is still the whole namespace.

**Why the scoping matters:** `namePrefix <tenant>/` is what turns a
misconfigured internal writer from a cross-tenant incident into a 403 in one
tenant's logs. That is exactly how the manifest and canary bugs were found.

Record the resulting key id in the tenant record (§2). The secret key goes into
OpenBao by pipe, never as an argument.

---

## 4. Per-tenant MEK

Every tenant has its own MEK, stored at `master-encryption-key` in the tenant
record. Two cases:

**Existing tenant moving in: keep the MEK.** Ciphertext moves as-is. The MEK
wraps each object's DEK inside the envelope; the envelope does not encode the
bucket or the key path, so a copy to a new prefix decrypts identically. **No
re-wrap, no rotation, no migration of key material is needed** — do not build
one. The only change is that the *same* MEK now arrives at the pod from the new
OpenBao path.

**New tenant: generate one.** Never let ARMOR mint it — a self-minted key is an
un-escrowed key.

```bash
openssl rand -hex 32 | bao-as rs-manager-provision \
  bao kv put -cas=$V "secret/$TENANT_PATH" master-encryption-key=-
```

Escrow it before making it active, per
[disaster-recovery.md — MEK Backup and Escrow](../disaster-recovery.md). A MEK
that exists only in the tenant record is a MEK that is one bad `bao` call away
from a completely unreadable prefix.

Ring semantics are unchanged: if this tenant rotates later, the old value moves
to the ring and both are loaded by every replica of that tenant's instance.
See the [Key Rotation Runbook](../key-rotation-runbook.md).

---

## 5. Client credentials and verb-scoped ACLs

Each of a tenant's clients gets its own named credential — never the tenant's B2
key, and never a shared default credential. ARMOR enforces the prefix a second
time at the proxy layer.

ACL syntax is `<bucket>:<prefix>:<verbs>`; verbs are drawn from
`{get, put, delete, list}`, joined with `+`; a comma-separated list carries
multiple entries; an omitted third segment means all verbs.

```
# A writer that can only put and list under the tenant prefix:
<bucket>:<tenant>/:put+list

# A reader:
<bucket>:<tenant>/:get+list

# A tenant that prunes, via its own client:
<bucket>:<tenant>/:get+put+delete+list
```

Unlike everything else in this runbook, the ACL string **does** contain the
bucket name — `<bucket>` above is the real name at provisioning time. That is
fine: the string lands in OpenBao and in pod env, both of which are acceptable
carriers. It does not land in git.

Defaults for a new tenant:

| Credential | ACL | Delivered to |
|---|---|---|
| writer | `<bucket>:<tenant>/:put+list` | the producing workload |
| reader | `<bucket>:<tenant>/:get+list` | the reading instance, if separate |
| admin/pruning | `<bucket>:<tenant>/:get+put+delete+list` | only if the tenant prunes |

Write them to `writer-access-key` / `writer-secret-key` / `writer-acl` (and
`reader-*`) in the tenant record. Generation and delivery follow §2's
by-reference rules; the shape to aim for is the reference `ExternalSecret`
linked at the top.

Note what the ACL does **not** cover: it constrains what a client may do
through ARMOR. It does not widen the B2 key, and it does not give a client
access to `<tenant>/.armor/` — internal state is ARMOR's, not the client's.

---

## 6. Prefixes

Three environment values, one decision each:

```bash
ARMOR_PREFIX=<tenant>/            # required
# ARMOR_MANIFEST_PREFIX: leave unset. An empty value is treated the same
# as unset, so either way the default applies.
# canary follows ARMOR_PREFIX     # nothing to set
```

- **`ARMOR_PREFIX=<tenant>/`** — one trailing slash, no leading slash. `T`, `T/`, and
  `/T/` all normalise to `T/`, but write it in the normal form anyway.
- **`ARMOR_MANIFEST_PREFIX`** — **relative to `ARMOR_PREFIX`**, default
  `.armor/manifest`. Leave it unset: the default already composes to
  `<tenant>/.armor/manifest/`, which is what a tenant wants. Setting it relocates
  manifests *within the tenant's namespace*; it can never escape the prefix. A
  bucket-root value is the bug the relative rule exists to prevent.
- **Canary** — no configuration. It composes `<tenant>/.armor/canary/` from the
  prefix, given prerequisite 2.
- **Provenance** — leave off (§1).

`<tenant>/.armor/` is inside the tenant's B2 key scope, which is the point: the
tenant's key can write its own internal state and nobody else's.

---

## 7. Moving an existing tenant's objects

Server-side copy, inside the B2 account, with `rclone`. No bytes traverse the
network boundary, so the move costs no egress; per the repo's own
[pricing research](../research/b2-pricing-and-features.md) a `CopyObject` lands
in Class C rather than Class A, so it draws on the 2,500/day free tier rather
than being unconditionally free (and all standard API calls become free from
2026-05-01). Either way the cost driver this whole design exists to avoid —
egress on read — is not incurred by a copy.

**Preserve the stored metadata.** ARMOR's envelope, HMAC table, and multipart
sidecars live in object metadata and in sibling objects; a copy that drops or
rewrites headers produces an object that lists fine and fails verification.
Use a copy mode that carries metadata verbatim and verify with the
restore-verifier, not by re-reading a sample by hand.

**Incremental, then a final delta:**

```
1. Bulk copy            rclone copy --metadata --server-side-across-configs \
                          "$SRC_REMOTE:$SRC_BUCKET/<tenant>/" \
                          "$DST_REMOTE:$DST_BUCKET/<tenant>/"
                        (safe to run with writers live; re-runnable; idempotent)
                        --metadata preserves the stored file-info headers ARMOR
                        reads envelope and HMAC state from

2. Repeat               until a pass copies near-zero objects — this converges
                        the backlog while writers keep working

3. Pause writers        the only window in the whole procedure

4. Final delta          one more pass; now it is genuinely near-zero, and
                        nothing new can arrive

5. Verify               §8, before anything writes to the new prefix

6. Cut over             §9

7. Resume writers
```

Pause writers before the final delta, not before the bulk copy — the bulk copy
hours are the ones you cannot afford to be down for, and they do not need a
pause. Practically, "pause" means scaling the tenant's producers to zero; the
ARMOR instance itself can stay up for the copy.

**Leave the source in place until §10.** Do not delete the old bucket's objects
as part of the move.

---

## 8. Verification by property

Never print a bucket name, a key id, or key material to prove something works.
Each check below is a property — a count, a status code, a version number — that
is true only if the thing it stands for is also true.

**Object count, listed with the tenant's own key.** This is the load-bearing
check: it proves the copy landed *and* that the tenant's B2 key can see its own
prefix. Run it with the tenant key, not an admin key:

```bash
# source count and target count must match
b2 ls --recursive --json "$BUCKET_NAME" "<tenant>/" | jq 'length'
```

Compare three things, not one: object count, summed byte size, and a checksum
sample. A count match with a size mismatch means a truncated or re-encoded
copy.

**Restore-verifier, configured with the tenant prefix.** This is the check that
actually means "the tenant works", because it exercises read, prefix
composition, HMAC verification, and the MEK together:

```json
{ "bucket": "<tenant-view>", "prefix": "<tenant>/", "enabled": true }
```

A green verifier run over the moved prefix is the strongest single signal
available. A red one is a stop — do not cut over over a failing verifier.
See the [Restore Verifier Deployment Guide](../restore-verifier-deployment-guide.md).

**OpenBao writes**, by metadata version (§2) — never by reading a value back.

**Readiness**, once the instance is rolled: `/readyz` Ready, and the canary
`last_error` metric empty. This is prerequisite 1's payoff — a canary that
cannot write its own prefix fails here, loudly, before the tenant's traffic
does.

**What a check must never be:** "I ran `b2 list` and eyeballed the names." That
prints the bucket name into the transcript and proves nothing the count does not.

---

## 9. Cutover order

Run in this order. Steps 4–7 are the pause window; everything before it is
preparation that can be rehearsed and rolled back for free.

```
 1. Confirm prerequisites        §1 — all four, on the target deployment
 2. Write the tenant record      §2 — OpenBao, by pipe, cas'd
 3. Create the B2 key            §3 — record the id, pipe the secret
 4. PAUSE WRITERS                scale the tenant's producers to zero
 5. Final delta                  §7 step 4
 6. Verify                       §8 — count, bytes, checksums, restore-verifier
 7. Cut over the instance        §A — env and Secret changes
 8. Roll the instance            and wait for Ready
 9. Resume writers               through ARMOR, on the new prefix
10. Post-cutover verification    §8 again, with real traffic flowing
11. Retire the source            §10 — deliberately last, deliberately manual
```

**Step 7 — what changes.** The instance's env moves to the unified bucket:

| Variable | Becomes |
|---|---|
| `ARMOR_BUCKET` | the unified bucket (from the tenant record's `bucket`) |
| `ARMOR_PREFIX` | `<tenant>/` — **new for tenants that previously ran at the bucket root** |
| `ARMOR_BUCKET_ALIASES` | the old bucket name, if any client URL still names it |
| `ARMOR_AUTH_*` | the tenant's named client credentials (§5) |
| `ARMOR_MEK` | unchanged for an existing tenant (§4) |
| B2 access key | the tenant's prefix-scoped key (§3) |

The `ARMOR_BUCKET` value reaches the pod as an ordinary Secret property read
from the tenant record — it is cluster state, and that is the intended carrier.
It is not a reason to relax the naming rule.

**Two facts that bite.**

1. **Every ARMOR instance logs its bucket name at startup.** Until
   prerequisite 3 is in the deployed image, `primary backend initialized (b2)`
   prints the name in clear, on every roll, into pod logs and every log
   pipeline attached to the cluster. Cutover puts the name into a *new* set of
   pods' env, so cutover is exactly when this fires. If prerequisite 3 is not
   live, accept that the name reaches the log pipeline — that is a bounded,
   known exposure, not a git leak — and fix the image before the next roll
   rather than abandoning the cutover.
2. **Reloader on apexalgo-iad does not react to Secret creation.** It reacts to
   an update to an already-annotated Secret. A brand-new `Secret` (which is
   what a new tenant's first `ExternalSecret` produces) is created *after* the
   Deployment was annotated, so Reloader has no prior checksum to diff against
   and no rollout happens — the pod keeps running with the old env
   indefinitely, and the failure reads as "the config change didn't take."

   Handle it deliberately: either pre-create the `Secret` in one change (let
   ESO sync it) and flip the annotation in the next, or roll the Deployment
   yourself. Do not wait for Reloader. The same applies to a `SecretSynced`
   that flips true but no pod restarts.

---

## 10. Decommissioning the source

Deliberately last, deliberately manual, and out of scope for onboarding. After
cutover:

- leave the source bucket and its objects in place through at least one full
  restore-verifier cycle and one backup rotation of every workload that moved
- keep the old B2 key alive while `ARMOR_BUCKET_ALIASES` still names the old
  bucket, so an un-migrated client's read still resolves
- deletion of the source bucket and a revocation of its key is an operator
  action on the B2 account, done after a stated retention window, with the
  tenant record updated to point at the unified bucket as the only copy

Nothing in this runbook requires the source to go away, and nothing in it is
safe to shortcut by making it go away early.

---

## 11. Checklist

Printable form of everything above.

- [ ] §1 prerequisites confirmed on the target deployment (manifest, canary,
      startup redaction, alias if needed); provenance left off
- [ ] §2 tenant record written to `unified-bucket/<tenant>`, by pipe, cas'd,
      version verified by metadata
- [ ] §2 leaf added to every cluster's ESO grant that needs it; `SecretSynced=True`
- [ ] §3 B2 key created, `--bucket $BUCKET_ID --namePrefix <tenant>/`;
      `deleteFiles` granted only if `prunes=true`
- [ ] §4 MEK: kept (existing) or generated into OpenBao and escrowed (new)
- [ ] §5 named client credentials written with verb-scoped ACLs
- [ ] §6 `ARMOR_PREFIX=<tenant>/`, manifest prefix left unset
- [ ] §7 bulk copy converged, writers paused, final delta, source left in place
- [ ] §8 count + bytes + checksums under the tenant key, restore-verifier green
- [ ] §9 cut over, rolled, Ready, writers resumed, re-verified under load
- [ ] §10 source retirement scheduled, not assumed
- [ ] No bucket name, key id, or key material in any command line, log, bead,
      commit message, or document
