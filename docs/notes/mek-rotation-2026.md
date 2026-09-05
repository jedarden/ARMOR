# MEK Rotation 2026

Per-instance rotation records. The old key stays in the ring after each
rotation — retiring it is an operator step, never an agent step.

- [iad-ci ARMOR Instance](#iad-ci-armor-instance) — **re-run in progress
  2026-09-04 evening**: both code blockers are fixed and shipped in
  0.1.1960 (deployed on iad-ci ~18:00Z); the object re-wrap is walking the
  bucket under a detached resume client
  (see [the re-run](#re-run-on-011960-2026-09-04)); verification pending.
  The pending ARMOR_ADMIN_TOKEN rotation (bead armor-c1f4560a) is **gated on
  this walk** — see [the handoff](#handoff-armor_admin_token-rotation-is-gated-on-this-walk-bead-armor-c1f4560a)
  for why, and for the runbook.
- [rs-manager ARMOR Instance](#rs-manager-armor-instance) — procedure
  documented, awaiting operator execution.

## iad-ci ARMOR Instance

### Status
**Secret and configuration layer COMPLETE and verified. Object re-wrapping
was BLOCKED** by two ARMOR defects (admin 30 s hard write timeout; rotations
not resuming) — **both fixed and shipped in 0.1.1960**, deployed on iad-ci
2026-09-04 ~18:00Z. The re-wrapping re-run is [in progress](#re-run-on-011960-2026-09-04).
No key material appears in this document, only fingerprints.

### Identifiers

| | |
|---|---|
| Cluster / namespace | `iad-ci` / `armor` |
| Image at rotation | `ronaldraygun/armor:0.1.1957` |
| OpenBao path (keys) | `secret/rs-manager/iad-ci/armor` |
| OpenBao path (admin token) | `secret/rs-manager/iad-ci/armor/admin` |
| Escrow path | `secret/rs-manager/escrow/iad-ci-armor` |
| Env vars | `ARMOR_MEK`, `ARMOR_MEK_RING`, `ARMOR_ADMIN_TOKEN` |

### Fingerprints (never keys)

| Role | Fingerprint |
|---|---|
| **New active MEK** | `3d3e10bb0ba11bcb` |
| **Old MEK (retired into ring)** | `f68571480246d3d5` |

Fingerprints are `hex(sha256(mek_bytes)[:8])` — see
`crypto.MEKFingerprint` (`internal/crypto/fingerprint.go:12`).

### Timeline (UTC, 2026-09-04)

| Time | Event |
|---|---|
| ~04:00 | OpenBao `MEK_RING` created with the then-active MEK; new MEK generated into `MASTER_ENCRYPTION_KEY` (v4 → v5) |
| (earlier) | First write of the new MEK carried a trailing newline → pod crash-looped on `ARMOR_MEK must be hex-encoded` |
| 06:16:12 | `MASTER_ENCRYPTION_KEY` rewritten newline-free (v12); pod started clean at 06:17:43 once ESO synced |
| 06:45:57 | Escrow updated to v2, then re-written flat to **v3** at 06:48:28 (see [Escrow](#escrow)) |
| 06:55+ | Verified pre-rotation objects still decrypt through the ring |

### Ring state at close

```
active_fp   = 3d3e10bb0ba11bcb      (new)
ring_fps    = [f68571480246d3d5]    (old — STAYS, retirement is an operator step)
```

### What was verified

1. **Ring loads and backward compatibility holds.** A real pre-rotation
   object — `declarative-config-backups/declarative-config-2026-08-08.bundle`,
   written 2026-08-08 under the old MEK — was fetched through the S3 endpoint
   (SigV4, port 9000) and returned HTTP 200 with 7,024,767 bytes.
   `git bundle verify` reports `is okay` and resolves 2 refs, so the decrypted
   plaintext is valid, not merely non-empty. This exercises the ring path
   (old key unwraps the DEK, active key wraps new writes).
2. **Canary healthy.** `GET /armor/canary` → `status: healthy`,
   `decrypt_verified: true`, `hmac_verified: true`, multipart healthy, 0
   consecutive failures.
3. **Escrow updated** — see below.
4. **Deployment wiring** — `ARMOR_MEK`, `ARMOR_MEK_RING` (optional), and
   `ARMOR_ADMIN_TOKEN` all resolve from Secret `armor-secrets`; all nine
   ExternalSecrets in the namespace report `SecretSynced`.

### Escrow

`secret/rs-manager/escrow/iad-ci-armor` bumped **v1 → v3** at
2026-09-04T06:48:28Z. The stored shape is preserved from the operator's
original v1 write — a single `data` field containing
`{"mek": <active-hex>, "mek_ring": "<comma-joined hex>"}` — verified by
fingerprint, never by printing a value:

- `mek` → `3d3e10bb0ba11bcb` (the new active key)
- `mek_ring` → `f68571480246d3d5` (the old key)

Note: `bao kv put <path> -` reading JSON from stdin nests it under a `data`
key; that happens to match v1's existing shape, so it was kept. The v2 write
was correct in content but nested, and was superseded by v3.

### Blocker: object re-wrapping cannot run to completion

`POST /admin/key/rotate` (no body → fingerprint-based mode,
`internal/server/server.go:822`) re-wraps every object whose fingerprint
differs from the active one. It cannot complete on iad-ci:

1. **The admin listener hard-caps every request at 30 s.**
   `cmd/armor/cmd_serve.go:96-97` sets `ReadTimeout: 30s` and
   `WriteTimeout: 30s` on the admin `http.Server` (the S3 listener uses `0`).
   The deadline is armed when the request header is read, so a rotation that
   needs minutes is killed mid-walk.
2. **A full bucket walk takes minutes.** A read-only listing of the bucket
   through `/dashboard/api/list` (no per-object HEAD, no re-wrap) ran past
   **5 minutes**. The rotator is strictly slower: it lists everything and
   issues a HEAD per object (`objectMetadata`, because ListObjectsV2 on B2
   carries no user metadata) plus a CopyObject per object needing re-wrap.
3. **Interrupted rotations do not resume.** `initOrLoadState`
   (`internal/server/key_rotation.go:536-545`) only adopts existing state when
   `Status == "in_progress"`, but the cancel path at
   `key_rotation.go:200-209` saves `Status: "interrupted"`. Every subsequent
   invocation therefore starts a **fresh walk from `LastKey: ""`** — no
   progress ever accumulates, and no invocation can reach the end.
4. `GET /admin/key/ring` is unusable as a progress or verification signal for
   the same reason: it HEADs every manifest entry
   (`internal/server/server.go:970-1005`) and is killed at 30 s with an empty
   body.

Also checked and rejected: the dashboard rotate proxy
(`/dashboard/admin/key/rotate`) is **not** a workaround — it generates its own
random 32-byte MEK and posts it as the request body
(`internal/dashboard/dashboard.go:719-744`), which selects *legacy* rotation
mode. That would set an active key known only to the running process and not
to OpenBao, so the next pod restart would come up unable to unwrap any object
written in that window. Do not use it.

### What this means operationally

The rotation performed here is still safe and worthwhile: **new writes are
wrapped with the new MEK**, and **all existing objects remain readable**
through the ring. Nothing is at risk. What has not happened is the re-wrap of
existing objects onto the new key, so the ring cannot be retired yet — the old
MEK is still load-bearing for reads.

### Required follow-up (code, then re-run)

1. ~~Make the admin server timeouts configurable~~ **DONE** — shipped in
   `fb16d0fbe` (in 0.1.1960): admin `ReadTimeout`/`WriteTimeout` now default
   to disabled, bounded instead by `adminReadHeaderTimeout` (30 s) and
   `adminIdleTimeout` (2 min), overridable via `ARMOR_ADMIN_READ_TIMEOUT` /
   `ARMOR_ADMIN_WRITE_TIMEOUT`.
2. ~~Fix the resume-status mismatch~~ **DONE** — shipped in `c51695130`
   (in 0.1.1960): `interrupted` state is adopted exactly like
   `in_progress`, cancellation is recorded as an interruption (not an
   object-level failure), and progress checkpoints per object.
3. **IN PROGRESS** — re-run of the rotation started 2026-09-04 ~18:40Z;
   see [below](#re-run-on-011960-2026-09-04). Confirm via `GET /admin/key/ring`
   that `objects_by_fp` shows zero under `f68571480246d3d5` — or an explicit
   exception list (`ErrCopyObjectTooLarge` objects cannot be re-wrapped via
   CopyObject).
4. Only then retire `f68571480246d3d5` from the ring (operator step).

### Re-run on 0.1.1960 (2026-09-04)

Started ~18:40Z, ~35 min after the 0.1.1960 rollout, via
`POST /admin/key/rotate` (no body → fingerprint mode) against
`ronaldraygun/armor:0.1.1960@sha256:6be87aaa…`. Empirical confirmation the
timeout fix is live: a bare `GET /admin/key/ring` ran >20 min without being
killed — under 0.1.1957's hard 30 s cap that was impossible.

**The walk is tied to the client connection.** `rotateKey` calls
`rotator.Rotate(r.Context())` (`internal/server/server.go:910`), so a
client disconnect cancels the walk; the rotator saves `interrupted` state
under `context.Background()` (detached), and 0.1.1960's resume fix adopts it
on the next POST. Practical consequence: run the rotation under a
**detached** client so it survives the operator's session, e.g.

```bash
setsid nohup curl -sS --max-time 28800 -X POST \
  --header @"$H" http://127.0.0.1:19001/admin/key/rotate \
  -o /tmp/rotate-resume.json >/tmp/rotate-curl.status 2>&1 &
```

A kill-and-relaunch costs one pause/resume cycle (seconds), not a restart —
progress resumes from the persisted `LastKey`. Expected shape of the walk:
minutes-long (a read-only HEAD walk of the bucket alone exceeds 20 min; the
rotator adds a CopyObject per object still under the old fingerprint).

### Walk scale, and why it needs a detached driver (2026-09-05)

Measured 2026-09-05 ~00:40Z, while the re-wrap was running:

| Prefix | Objects |
|---|---|
| `declarative-config-backups/` | 30 |
| `forgejo/` (includes `forgejo/cnpg/` below) | 33,718 |
| `forgejo/cnpg/` | 30,998 |
| `queue-db/cnpg/` | 4,949 |
| `ci-cache/` | 0 |
| **Total user objects** | **38,697** |

The rotator sustains **~53 objects/min**, so a full walk is ~12 h, not the
20-40 min the ring GET alone needs. That is why the first ten dispatch cycles
kept finding a paused walk: each dispatch's client was killed at the 60-min
dispatch budget, the walk checkpointed, and the next dispatch re-attached.
Progress did accumulate (the 0.1.1960 resume fix works), but only in
60-minute slices with the bucket idle in between.

`/admin/key/rotate` has **no server-side guard against two concurrent
walks** — `stateMu` (`internal/server/key_rotation.go:114`) serialises state
*file* writes only, and each POST builds its own `KeyRotator`. Two attached
clients therefore walk simultaneously, double-counting `processed_objects`
and doubling backend load. Re-wrapping stays idempotent (an object already at
the target fingerprint is skipped), so no corruption — but exactly one client
should ever be attached.

Fix used from 00:47Z: a detached single-instance driver
(`/tmp/armor-rotate-driver.sh`, pid lockfile) that holds one POST open for up
to 4 h, re-attaches on disconnect, refuses to resume a `failed` state, and
when the POST returns 200 runs the ring GET itself. It re-establishes the
port-forward if it dies. With it, completion no longer depends on a dispatch
happening to arrive and re-attach a client.

Two verification-side corrections found while re-checking read-through:

- The `AUTH_ACCESS_KEY`/`AUTH_SECRET_KEY` fields on
  `secret/rs-manager/iad-ci/armor` are **stale** — ARMOR returns
  `InvalidAccessKeyId` for them. Live credentials come from Secret
  `armor-credentials` key `credentials.yaml` (`ARMOR_AUTH_FILE`), which is
  ACL-scoped per prefix: `FORGEJO_BACKUP` holds
  `iad-ci:declarative-config-backups/`, so a whole-bucket ListObjectsV2 is
  `AccessDenied` by design. Count the table above prefix by prefix with the
  matching credential.
- Re-verified read-through with the live credential:
  `declarative-config-backups/declarative-config-2026-08-08.bundle` (written
  2026-08-08, pre-rotation) → HTTP 200.

### Reaching the admin API (correction to Plan §8.6)

Plan §8.6's route — `kubectl proxy` plus a bearer token in the
`Authorization` header — **does not work**. Verified: with no `Authorization`
header the request reaches ARMOR (`Server: ARMOR/0.1.1957`, plain-text 401);
with any `Authorization` header the Kubernetes API server consumes it as the
*client* credential and returns a JSON `Status` 401 before proxying. The
header never reaches the backend service, so the admin token cannot be
delivered this way.

Use **`kubectl port-forward`** instead — it tunnels raw TCP to the pod and
leaves headers untouched:

```bash
kubectl port-forward -n armor deploy/armor 19001:9001 --address=127.0.0.1
# then, with the token read into a mode-600 header file (never argv):
curl -sS --header @"$H" http://127.0.0.1:19001/admin/key/ring
```

`GET /dashboard/admin/key/status` is a cheap alternative to
`/admin/key/ring` for rotation progress: it reads
`.armor/rotation-state.json` (one GET) instead of HEADing every object.

It needed no credentials on iad-ci — `ARMOR_DASHBOARD_*` is unset there, so
`auth.Wrap` is a no-op, and the admin gate waived the whole `/dashboard`
prefix. Fixed on main 2026-09-05 (ab187d6ba; beads armor-d7010d2d,
armor-cfd49e41): the mount now falls back to the admin bearer-token gate and
fails closed when no token is configured. Until the pod rolls to an image
with that fix the endpoint still answers anonymous; after the roll, send the
same mode-600 header file as the `/admin/key/ring` call above.

### Reading `/admin/key/ring` (response shape and two counting traps)

`internal/server/server.go:928` — the response is
`{"keys": {"<key-id>": {"active_fp", "ring_fps[]", "objects_by_fp{fp:count}"}}}`.
Two properties of how the histogram is built matter for the acceptance check:

- **Objects with an empty metadata fingerprint count under `"legacy"`, not
  under the old fingerprint.** `server.go:999` maps `""` → `"legacy"`. So
  "`objects_by_fp` has zero objects under the old fingerprint" does *not*
  imply the bucket is fully re-wrapped — the `legacy` bucket must be reported
  alongside it.
- **A transient `Head` failure silently skips an entry** (`server.go:984` —
  `continue` on error, no counter, no log). The histogram HEADs all 38,697
  manifest entries over ~4.3 h, so a handful of transient B2 errors is
  likely; each one undercounts by one object and, if it lands on an
  old-fingerprint object, makes that object look re-wrapped when it is not.
  Reconcile the histogram total against the expected object count and treat a
  shortfall as a reason to re-run the GET rather than to declare success.

### Driver v1's ring-GET defect, and a watcher that would have resurrected it

Driver v1 (`/tmp/armor-rotate-driver.sh`) ran the ring GET with a 4 h
`--max-time` and then **moved the `.part` file into place unconditionally**
— a timeout produced a truncated `ring-final.json` that sat at the output
path looking like a finished histogram. At the measured ~4.3 h the GET needs
on this bucket, 4 h was a coin flip.

Driver v2 (`/tmp/armor-rotate-driver2.sh`, in use from 2026-09-05 01:21Z)
raises the cap to 8 h, retries up to 3×, and only moves the file on
HTTP 200 **and** valid JSON.

The watcher that keeps the driver alive was still pointed at v1, so if v2
died mid-ring-GET the watcher would have respawned v1 — reintroducing the
4 h cap and the unconditional move, and then looping ring GETs forever
(v1 has no "already valid" check, so it re-runs rather than reuses). Found
and fixed 2026-09-05 ~01:55Z: the watcher now restarts v2, and v2 is what
the watcher holds for the rest of the run. Any future re-run should delete
v1 rather than leave it reachable.

### Mid-walk proof the re-wrap itself is correct (2026-09-05)

The read-through verified above now predates the re-wrap: at 06:55Z the
object still carried the old fingerprint, so it exercised the **ring**
path (old key unwraps the DEK), not the **re-wrap** path. The re-wrap is
where a defect would actually destroy readability — unwrap the DEK with a
ring key, re-wrap it with the active key, `CopyObject` the result — and
the canary does not cover it, because the canary only ever writes and
reads with the active key.

While the walk was at ~15.3k / 38.7k objects, the same bundle was fetched
again through the S3 endpoint:

| | pre-rotation (06:55Z) | after re-wrap (2026-09-05 ~03:05Z) |
|---|---|---|
| HTTP | 200 | 200 |
| bytes | 7,024,767 | 7,024,767 |
| `git bundle verify` | is okay, 2 refs | is okay, 2 refs, complete history |

Byte-for-byte identical size and a valid bundle after the re-wrap: the
DEK survived the unwrap/re-wrap round trip and the copy is faithful.

The object was in fact **rewritten by the rotator**, not skipped —
`GET /dashboard/api/list?prefix=declarative-config-backups/` reports
`last_modified 2026-09-04T18:31:48Z`, i.e. 38 s after the walk started
(18:31:10Z), which is the `CopyObject`. (That same listing reports
`encrypted: false`; that is a B2 listing artifact, not evidence about the
object — `ListObjectsV2` on B2 carries no user metadata, which is also why
the rotation state's `total_objects` stays at 0. Only a `Head`/`Get`
returns the ARMOR metadata.)

Procedural note for repeating this check: the S3 port speaks SigV4, so
`curl --aws-sigv4 "aws:amz:<region>:s3"` works — keep the pair out of
argv by building a mode-600 curl config file with a redirect-only
pipeline and passing `-K <file>`, then shred it. The ACL-scoped
`FORGEJO_BACKUP` credential covers `declarative-config-backups/`.

### Handoff: ARMOR_ADMIN_TOKEN rotation is gated on this walk (bead armor-c1f4560a)

Dispatched 2026-09-05 with the precondition "after the MEK rotation walk
completes". The walk was still running at dispatch time, so the rotation did
not execute — that ordering is load-bearing, not bureaucratic:

1. **A rotation rolls the pod.** Reloader (`reloader.stakater.com/auto: "true"`
   on `armor-deployment.yaml`) restarts the Deployment the moment the
   `armor-secrets` Secret changes, and `AdminToken` is read once at startup
   from env (`internal/config/config.go:490`, no hot reload). A mid-walk roll
   interrupts the walk and breaks the port-forward.
2. **The driver authenticates with the old token.** `/tmp/.armor-adm-hdr`
   (mode 600) holds the current token; after a rotation every driver
   reconnect — the re-POST after a `--max-time` disconnect, or a ring-GET
   retry — 401s, killing both the automated completion path and the
   verification GET.
3. **Auth is checked at request time only** (`adminAuthMiddleware`,
   `internal/server/admin_auth.go`), so an already-in-flight POST/ring GET
   survives a roll, but any *retry* after it does not. Verification must be
   finished before the token changes.

**State at handoff (2026-09-05 ~04:56Z).** Walk at 21,622 / ~38,697 objects
(~56/min), `status=in_progress`, driver2 (pid 214960, 24 h budget from
01:21Z) + watcher both alive, single attached client. Walk ETA ~10:00Z;
ring GET adds ~4.3 h, so `/tmp/ring-final.json` lands ~14:30Z.

**Already verified, do not re-derive.** Token flow: OpenBao
`secret/rs-manager/iad-ci/armor/admin` field `admin_token` (at **v1**, sole
field at that path, so a versioned write clobbers nothing) → ExternalSecret
`armor-secrets` (ESO `Ready=True`, `refreshInterval: 1h`, `force-sync`
annotation forces an immediate refresh) → Secret `armor-secrets` key
`admin-token` → pod env. Consumers of the token: the armor pod itself, the
`armor migrate` CLI, and the driver header file — restore-verifier and
armor-test use their own credentials. Write identity `bao-as
rs-manager-provision` exists and can create/update on this prefix.

**Runbook for whoever picks this up (after `ring-final.json` is valid):**

```bash
# 0. Gate: histogram reconciles — old fp 0, legacy 0 (or explained
#    exceptions), total ~38,697 modulo transient-HEAD undercounts.
jq . /tmp/ring-final.json

# 1. Write the new token — generated in place, never argv, never a literal.
V=$(bao-as rs-manager-provision bao kv metadata get -format=json \
      secret/rs-manager/iad-ci/armor/admin | jq .data.current_version)
openssl rand -hex 32 | bao-as rs-manager-provision bao kv put \
  -cas=$V secret/rs-manager/iad-ci/armor/admin admin_token=-

# 2. Force the ESO refresh: bump the force-sync annotation on
#    declarative-config k8s/iad-ci/armor/armor-externalsecret.yaml,
#    commit, push, let ArgoCD sync (or wait out refreshInterval).
# 3. Reloader rolls the pod. Verify by property, never by value:
#    with a header file built from the NEW token,
#    GET /admin/no-such-route -> 404 (accepted; bad token -> 401 —
#    adminAuthMiddleware runs before routing), and with the OLD token
#    -> 401. Then /armor/canary healthy and
#    /dashboard/admin/key/status still reporting the completed rotation.
# 4. Rebuild /tmp/.armor-adm-hdr from the new token (mode 600) if the
#    driver/watcher machinery is still needed.
```

**Progress re-check (2026-09-05 ~05:15Z lineage, bead armor-c1f4560a).**
`GET /dashboard/admin/key/status` at 05:01:43Z reported `in_progress` at
21,949 / ~38,697 (~57%). The last five watcher samples give 291 objects /
5 min = **58/min**, so the walk finishes ~**09:52Z** and the ring GET lands
~**14:10Z**. driver2 (pid 214960), the watcher (pid 331612) and the
port-forward (pid 3689471) were all alive with exactly one attached POST
client.

**The rotation is ARMED, not waiting for a human.** A parallel lineage
started `/tmp/armor-c1f4560a-rotate.sh` (pid 858152, log
`/tmp/armor-c1f4560a-rotate.log`, lock `/tmp/.armor-c1f4560a.lock`) at
05:02:09Z. It polls every 2 min and fires the runbook above on its own once
`/tmp/ring-final.json` is valid **and** the histogram reconciles (old fp
`f68571480246d3d5` = 0, `legacy` = 0, total ≥ 38,597) **and** the walk state
is `completed`; then it stops driver2 + the watcher, writes OpenBao under
`-cas`, waits ≤2 h for the ESO→Reloader roll, and verifies new=200 / old=401
on `/admin/creds` before logging `ROTATION_OK`. Do **not** run the runbook by
hand while it is armed — a second writer to `secret/rs-manager/iad-ci/armor/admin`
would race it and make its verification meaningless. Expected `ROTATION_OK`
~**14:15–16:15Z**; read the log first, and treat `GATE FAILED` /
`ROLL_TIMEOUT` / `VERIFY_FAILED` as the triage paths. Two things that gate
will flag for a human even though the walk itself is fine: it never rejects a
*third* stale fingerprint (only the old fp and `legacy` are checked), and
`legacy == 0` assumes no `.armor/` system object lands in the ring histogram
without a metadata fingerprint.

Two things the next lineage should expect, so neither reads as a failure:

- **At ~05:21Z the in-flight POST hits driver2's per-attempt cap**
  (`--max-time 14400`, started 01:21:17Z). curl exits 28, driver2 logs a
  non-200, sleeps 20 s and re-POSTs; the walk resumes from the persisted
  `LastKey`. A sample taken in that window reads `interrupted` — that is the
  designed pause/resume cycle, not a regression. The same blip recurs every
  ~4 h if the walk is still going.
- **The watcher log shows a repeating stderr line**
  `/tmp/armor-rotate-watch.sh: line 54: /tmp/ring-final.json: No such file or
  directory`. Benign: bash reports the failed `< "$RING"` redirection before
  the `2>/dev/null || echo 0` fallback can swallow it, so `ring_bytes=0` is
  still printed correctly. It stops once the ring JSON exists.

**Dashboard exposure, calibrated and split out (armor-cfd49e41).** The
no-credential `/dashboard` access noted above was re-confirmed live at
05:11Z and is broader than just `key/status` — `/dashboard` itself and
`/dashboard/api/list` also return 200 — but narrower in reach than it sounds:
`/dashboard/credential-activity` still returns 502 with the upstream admin
API's 401 (the admin API is not reachable through the dashboard), and admin
port 9001 is a ClusterIP Service with no Ingress, so the exposure is
cluster-internal rather than internet-facing. Root cause is the conjunction
of `dashboard.go:47` ("no auth configured - allow all") with
`admin_auth.go:22` bypassing the admin token gate for the whole
`/dashboard` prefix. Details and fix directions are on armor-cfd49e41.
Note that its deploy-side fix rolls the pod, so it inherits this same
walk-completion gate.

---

## rs-manager ARMOR Instance

## Status
**Procedure finalized and documented. Awaiting operator execution.**

## Context
This document describes the rotation of the Master Encryption Key (MEK) for the rs-manager ARMOR instance using the key ring mechanism (Plan §8.13). The old key remains in the ring for backward compatibility.

## Pre-Rotation State

### rs-manager ARMOR Deployment
- **Instance:** rs-manager (cluster: `rs-manager`)
- **Namespace:** `armor`
- **OpenBao Path:** `secret/rs-manager/backblaze/armor`
- **Admin Token Path:** `secret/rs-manager/rs-manager/armor/admin`

### Current MEK Configuration
- **Active MEK:** Retrieved from OpenBao (operator access required)
- **MEK Ring:** Not yet configured (this rotation establishes the ring)
- **Escrow Path:** `secret/rs-manager/escrow/armor`

## Rotation Procedure

### Phase 1: Prepare the Key Ring (Operator Action)

1. **Retrieve the current MEK from OpenBao:**
   ```bash
   # As operator with OpenBao write access:
   bao-as rs-manager -- bao kv get -field=master-encryption-key secret/rs-manager/backblaze/armor > /tmp/current_mek.tmp
   chmod 600 /tmp/current_mek.tmp
   ```

2. **Generate a new MEK:**
   ```bash
   openssl rand -hex 32 > /tmp/new_mek.tmp
   chmod 600 /tmp/new_mek.tmp
   ```

3. **Update OpenBao with the new ring structure:**
   ```bash
   # Build the MEK_RING value (current MEK becomes first ring member)
   CURRENT_MEK=$(cat /tmp/current_mek.tmp)
   NEW_MEK=$(cat /tmp/new_mek.tmp)

   # Update OpenBao: new MEK as active, current MEK in ring
   bao-as rs-manager -- bao kv patch secret/rs-manager/backblaze/armor \
     master-encryption-key=- <<< "$NEW_MEK" \
     MEK_RING=@/tmp/current_mek.tmp

   # Update escrow with both keys
   bao-as rs-manager -- bao kv patch secret/rs-manager/escrow/armor \
     mek=@/tmp/new_mek.tmp \
     mek_ring=@/tmp/current_mek.tmp

   # Secure cleanup
   shred /tmp/current_mek.tmp /tmp/new_mek.tmp
   rm -f /tmp/current_mek.tmp /tmp/new_mek.tmp
   ```

4. **Verify the update:**
   ```bash
   # Check metadata version increased
   bao-as rs-manager -- bao kv metadata get secret/rs-manager/backblaze/armor | jq .data.current_version

   # Verify escrow was updated
   bao-as rs-manager -- bao kv metadata get secret/rs-manager/escrow/armor | jq .data.current_version
   ```

### Phase 2: Update Kubernetes Deployment

1. **Add ARMOR_MEK_RING environment variable to deployment:**
   ```yaml
   # In declarative-config/k8s/rs-manager/armor/armor-deployment.yml
   env:
   # ... existing env vars ...
   - name: ARMOR_MEK_RING
     valueFrom:
       secretKeyRef:
         name: armor-secrets
         key: MEK_RING
         optional: true  # Ring may not exist for first rotation
   ```

2. **Update ExternalSecret to include MEK_RING:**
   ```yaml
   # In declarative-config/k8s/rs-manager/armor/armor-externalsecret.yml
   data:
     # ... existing data mappings ...
     - secretKey: MEK_RING
       remoteRef:
         key: rs-manager/backblaze/armor
         property: MEK_RING
   ```

3. **Commit and push changes:**
   ```bash
   git add declarative-config/k8s/rs-manager/armor/
   git commit -m "feat(armor): add MEK_RING support for key rotation"
   git push
   ```

4. **Wait for ArgoCD sync** (automatic within 5 minutes)

### Phase 3: Trigger Key Rotation via Admin API

1. **Start kubectl proxy:**
   ```bash
   kubectl proxy --port=8001 &
   PROXY_PID=$!
   ```

2. **Get admin token:**
   ```bash
   # Retrieve admin token from environment (never printed)
   ADMIN_TOKEN=$(bao-as rs-manager -- bao kv get -field=admin_token secret/rs-manager/rs-manager/armor/admin)
   ```

3. **Trigger rotation:**
   ```bash
   curl -X POST \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/key/rotate
   ```

4. **Monitor rotation progress:**
   ```bash
   # Poll the key ring endpoint to track progress
   watch -n 5 'curl -s -H "Authorization: Bearer $ADMIN_TOKEN" \
     http://127.0.0.1:8001/api/v1/namespaces/armor/services/armor:9001/proxy/admin/key/ring | jq .'
   ```

5. **Wait for completion criteria:**
   - `active_fp` shows the new MEK fingerprint
   - `objects_by_fp` histogram shows 0 objects under old fingerprint
   - All objects now encrypted with new MEK

6. **Cleanup:**
   ```bash
   kill $PROXY_PID
   unset ADMIN_TOKEN
   ```

### Phase 4: Verification

1. **Check canary health:**
   ```bash
   curl -s http://armor.armor.svc.cluster.local:9000/armor/canary | jq .
   ```

2. **Verify an object written before rotation:**
   ```bash
   # Write a test object before rotation (if not already exists)
   # Then verify it still decrypts correctly after rotation
   ```

3. **Check pod logs for rotation completion:**
   ```bash
   kubectl -n armor logs deploy/armor --tail=100 | grep -i rotation
   ```

## Post-Rotation State

### Fingerprints (to be filled in by operator)
- **Old MEK Fingerprint:** TBD (first 8 bytes of SHA-256)
- **New MEK Fingerprint:** TBD (first 8 bytes of SHA-256)
- **Rotation Timestamp:** TBD
- **Objects Rotated:** TBD count
- **Rotation Duration:** TBD

### Retiring Old Keys (Operator Action)

Once verification confirms all objects are on the new key, the old MEK can be removed from the ring:

```bash
# Remove old MEK from ARMOR_MEK_RING in OpenBao
bao-as rs-manager -- bao kv patch secret/rs-manager/backblaze/armor \
  MEK_RING=<new_ring_without_old_key>

# Update escrow
bao-as rs-manager -- bao kv patch secret/rs-manager/escrow/armor \
  mek_ring=<new_ring_without_old_key>

# Trigger deployment restart to pick up new ring
kubectl -n armor rollout restart deployment/armor
```

## References

- **Plan §8.13:** MEK key ring design and rotation procedure
- **Plan §8.1:** Cipher correctness and format migration
- **ADR-005:** CTR keystream reuse vulnerability (why this rotation is critical)
- **docs/disaster-recovery.md:** MEK escrow and backup procedures

## Notes

- The old key STAYS in the ring until explicitly retired by an operator
- This is a design feature to prevent irreversible data loss during rotation
- Any ARMOR instance with the old MEK in its ring can decrypt old objects
- Only the active MEK wraps new DEKs for new uploads
- Rotation is O(N) where N = number of objects in the bucket
- Rotation uses CopyObject metadata operations (no data re-upload)
- Rotation is idempotent and can be safely resumed if interrupted

## Exceptions

If any objects cannot be rotated (e.g., >5GiB CopyObject exception), they will remain encrypted with their current key and appear in the `objects_by_fp` histogram under the old fingerprint. These should be tracked and addressed separately.
