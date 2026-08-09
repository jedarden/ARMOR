# commitgraph B2 access — downstream reuse doc

**Purpose:** the single self-contained file a downstream agent (enumerate
generations, run restore) reads to reach the `commitgraph` Litestream/B2 backup
**without** re-deriving cluster, kubeconfig, or credential steps.

**Last re-confirmed:** 2026-08-09 (live pod + kubeconfig mtimes + OIDC cache
re-checked). Source beads: bf-3hbw6f (recon), bf-3hvieu (kubeconfig probe),
bf-3bdye7 (Path A re-verify), bf-3e5ktj (Path B re-verify), bf-1k77wu (§1+§2
re-verify via observer kubeconfig), this doc bf-53li0z.

---

## 0. STATUS — BLOCKED on operator action (read this first)

> **Neither access path works today.** Both Path A (in-pod exec) and Path B
> (local secret-read) are blocked by the **same** gate: no live kubeconfig
> authenticates against `ord-devimprint`. The read-only observer works, but it
> grants neither `exec` nor `secret get`. **Unblock = operator refreshes the
> ord-devimprint admin kubeconfig via the Rackspace Spot dashboard** (see §6).
> Do **not** re-probe the kubeconfigs — all candidates were exhaustively tested
> on 2026-08-09 (evidence in Appendix). The recipes below fire verbatim the
> moment a single refreshed `ord-devimprint-admin` kubeconfig authenticates; that
> one kubeconfig grants **both** operations (same cluster-admin OIDC identity).

This doc records the exact commands to run once unblocked. It is deliberately
self-contained: bucket facts, cluster, kubeconfigs, both paths, and the verify
command are all here.

---

## 1. Bucket facts (constants — reuse verbatim)

| Field | Value |
|---|---|
| **Bucket** | `commitgraph-ops` |
| **S3 endpoint** | `https://s3.us-west-002.backblazeb2.com` |
| **Region** | `us-west-002` |
| **Backup path (key prefix)** | `queue-api/queue.db` |
| **In-pod DB path** | `/data/queue.db` |
| **Litestream config (in-pod)** | `/etc/litestream.yml` |

Source of truth: ConfigMap `queue-api-litestream-config` in namespace
`commitgraph` (originally documented in bf-57d3fx; matches the sidecar's mounted
config). Re-confirmed live via observer kubeconfig on 2026-08-09 (bf-1k77wu):
`bucket`, `endpoint`, `path` (backup key), and the in-pod `path: /data/queue.db`
are all literal fields in the ConfigMap's `litestream.yml`. The `region`
(`us-west-002`) is **not** a ConfigMap field — it is parsed from the endpoint
hostname (`s3.<region>.backblazeb2.com`). The `/etc/litestream.yml` in-pod path is
confirmed via the deployment's `volumeMount` (`litestream-config` →
`/etc/litestream.yml`), not the ConfigMap itself.

---

## 2. Host cluster & kubeconfigs

The `commitgraph` namespace exists **only** on **`ord-devimprint`**. Probed
every cluster on 2026-08-09: NotFound on apexalgo-iad, ardenone-cluster,
ardenone-manager, rs-manager, iad-ci, iad-kalshi, iad-options; iad-native-ads is
decommissioned.

| Purpose | Kubeconfig | Works today? |
|---|---|---|
| **Read-only** (get/describe/logs) | `/home/coding/.kube/ord-devimprint-observer.kubeconfig` | ✅ Yes (long-lived SA token, never expires). Grants neither `exec` nor `secret get`. |
| **Read-only (alt)** | `--server=http://kubectl-proxy-ord-devimprint:8001` | ✅ Yes (Tailscale operator proxy) |
| **Admin (exec + secret-read)** | `/home/coding/.kube/ord-devimprint-admin.kubeconfig` | ❌ **BLOCKED** — static token expired (401); OIDC exec path has no cached token + no browser headless (timeout). Needs operator refresh (§6). |

> The admin kubeconfig is the same one for **both** Path A and Path B. There is
> not a separate kubeconfig per operation. Mtime as of 2026-08-09:
> `2026-08-07 20:01` (not regenerated since).

### queue-api pod (rotates — use the label selector, not the name)

```bash
# Pod-name-independent (STABLE — use this):
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-observer.kubeconfig \
  get pods -n commitgraph -l app=queue-api
```

- Deployment: `queue-api` (replica 1). Current pod (2026-08-09):
  `queue-api-c5894c469-pzjsl`, 2/2 Running.
- Containers: `queue-api` (`ronaldraygun/commitgraph-queue-api:2.8.0`) and
  **`litestream`** sidecar (`litestream/litestream:0.5.11`). The sidecar
  container name is confirmed = **`litestream`** (use `-c litestream`).
- Secret `commitgraph-b2-workers` feeds both containers via
  `valueFrom.secretKeyRef`. Observer-verifiable facts: `get secrets -n
  commitgraph` (list — allowed) shows it as **`Opaque`, 5 data keys**; the
  deployment env refs name four of those keys — `key-id`, `application-key`,
  `bucket`, `prefix` — and the two B2 keys the restore needs are `key-id` and
  `application-key`. What the observer *cannot* do is read the secret **values**:
  individual `get secret commitgraph-b2-workers` / `describe` returns `403
  Forbidden` (which is exactly what confirms the §2 table's "grants neither exec
  nor secret get").

---

## 3. Path A — RECOMMENDED (in-pod litestream exec)

**Why recommended:** running litestream **inside the sidecar** sidesteps the
local EC2-IMDS credential fallback entirely. The mounted `/etc/litestream.yml`
pulls credentials via env interpolation —
`access-key-id: ${LITESTREAM_ACCESS_KEY_ID}` /
`secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}` — and both vars are injected
into the pod from secret `commitgraph-b2-workers` (read-only-confirmed in the
deployment spec; see §2). So the B2 keys are present in-pod at exec time: no
`LITESTREAM_*` to remember to export locally, and no IMDS error path. This is the
structurally cleanest path; Path B is the fallback for when exec is unavailable.

**Status:** BLOCKED-pending-validation — no live kubeconfig grants `pods/exec`
today (see §0, Appendix), so the expected outputs below are **not** live-captured.
The recipe is nonetheless **correct-by-construction**: every literal it relies on
is read-only-confirmed via the observer kubeconfig — container name `litestream`
(`-c litestream`), label `-l app=queue-api`, DB path `/data/queue.db`, config path
`/etc/litestream.yml`, and the `${LITESTREAM_*}` env interpolation. Do **not**
treat §3 as validated until §6 unblock yields the expected output.

### Verify connectivity (copy-paste) — Path A

```bash
KC=/home/coding/.kube/ord-devimprint-admin.kubeconfig   # after operator refresh (§6)
NS=commitgraph

# (1) verify connectivity + replica/db mounts, NO IMDS error (creds are in-pod):
kubectl --kubeconfig=$KC exec -n $NS -l app=queue-api -c litestream -- \
  litestream databases
```

**Expected output (once unblocked):** a table listing `/data/queue.db` with its
replica(s) against `s3://commitgraph-ops/queue-api/queue.db` — and **no**
`failed to refresh cached credentials, no EC2 IMDS role found` line. *(Not yet
captured live — exec is blocked today.)*

### Enumerate generations (the downstream target) — Path A

```bash
kubectl --kubeconfig=$KC exec -n $NS -l app=queue-api -c litestream -- \
  litestream generations /data/queue.db
```

### Optionally re-confirm the mounted config in-pod

```bash
kubectl --kubeconfig=$KC exec -n $NS -l app=queue-api -c litestream -- \
  cat /etc/litestream.yml
# expect: bucket=commitgraph-ops, path=queue-api/queue.db,
#         endpoint=https://s3.us-west-002.backblazeb2.com
```

---

## 4. Path B — FALLBACK (local secret-read → export → B2 call)

**Use when:** exec is unavailable but a secret-read-capable kubeconfig exists.
Read the B2 keys locally, base64-decode, **export them as env vars** (this is
load-bearing — exporting them is exactly what prevents litestream's IMDS
fallback, the root cause of the bf-3dntjx restore failure).

**Status:** BLOCKED (same kubeconfig-auth gate — no live kubeconfig grants
`secret get` against `commitgraph`; see §0, Appendix). Local CLIs needed for
this path are all present: `litestream` (`/usr/local/bin/litestream`), `aws`,
`b2`.

### Decode + export the two B2 keys (copy-paste) — Path B

```bash
KC=/home/coding/.kube/ord-devimprint-admin.kubeconfig   # after operator refresh (§6)
NS=commitgraph

# Decode the two keys the litestream sidecar mounts (NO IMDS — creds explicit):
export LITESTREAM_ACCESS_KEY_ID=$(kubectl --kubeconfig=$KC -n $NS \
  get secret commitgraph-b2-workers -o jsonpath='{.data.key-id}' | base64 -d)
export LITESTREAM_SECRET_ACCESS_KEY=$(kubectl --kubeconfig=$KC -n $NS \
  get secret commitgraph-b2-workers -o jsonpath='{.data.application-key}' | base64 -d)
```

> No credential values are recorded in this file. Once decoded, verify the vars
> are non-empty in the shell (`test -n "$LITESTREAM_ACCESS_KEY_ID"`) before
> proceeding — if they are empty, the `get secret` was denied and downstream
> calls will fall back to IMDS.

### Verify connectivity (copy-paste) — Path B

```bash
# (2a) aws-cli bucket listing (returns objects, NO IMDS error):
aws s3 ls s3://commitgraph-ops/ \
  --endpoint-url https://s3.us-west-002.backblazeb2.com --region us-west-002

# (2b) OR local litestream generations (returns a generation, NO IMDS error):
litestream generations -config scratch/litestream-restore/restore-config.yml /data/queue.db
```

**Expected output (once unblocked):** (2a) a bucket object listing including
`queue-api/queue.db` WAL segments; or (2b) a generation row — with **no**
`failed to refresh cached credentials, no EC2 IMDS role found` error. *(Not yet
captured live — secret-read is blocked today.)* If the IMDS error reappears, the
env vars were not exported into that shell (the failure mode this bead exists to
prevent).

---

## 5. Verify connectivity — quick reference (both paths)

| Path | One-liner (run after §6 unblock) |
|---|---|
| **A (in-pod, recommended)** | `kubectl --kubeconfig=$KC exec -n commitgraph -l app=queue-api -c litestream -- litestream databases` |
| **B (local, fallback)** | `aws s3 ls s3://commitgraph-ops/ --endpoint-url https://s3.us-west-002.backblazeb2.com --region us-west-002` (with `LITESTREAM_*` exported) |

**Pass criterion:** a real result (db/replica list or bucket listing) with NO
`EC2 IMDS` / `context deadline exceeded` error. **Fail criterion** (current
state, pre-unblock): `401 Unauthorized` (static token) / `403 Forbidden`
(observer) / OIDC timeout (admin OIDC exec).

---

## 6. How to unblock (operator action only)

This headless agent **cannot** clear the gate — Spot OIDC has no CLI refresh
path (per bf-1q3s0v), and the OIDC token cache holds only a 0-byte lock file
(no token blob), so the `oidc-login` exec provider must run an interactive
browser auth-code flow impossible on this Hetzner box.

The operator (jedarden) must:

1. **Rackspace Spot dashboard → ord-devimprint cloudspace → generate fresh admin
   kubeconfig.** Overwrite `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
   (`chmod 600`).
2. **Prime the OIDC token cache once from a browser-capable machine** — run any
   command (`kubectl --kubeconfig=…ord-devimprint-admin.kubeconfig get ns`) and
   complete the `login.spot.rackspace.com` SSO in a browser. This writes the
   access + refresh token into `~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm/`,
   after which the OIDC exec path works headless until the refresh token lapses.
   - Shortcut: the freshly-downloaded kubeconfig also carries a **static
     `ngpc-user` token** that works headless immediately — but it is short-lived
     (the Aug-07 20:01 regeneration's static token was already `Unauthorized`
     by 2026-08-09, i.e. < ~37h). Treat it as a temporary unlock, not a fix.
3. **Verify both operations before handing off:**
   ```bash
   KC=/home/coding/.kube/ord-devimprint-admin.kubeconfig
   kubectl --kubeconfig=$KC get secret commitgraph-b2-workers -n commitgraph -o jsonpath='{.data}'  # secret-read
   kubectl --kubeconfig=$KC exec -n commitgraph -l app=queue-api -c litestream -- echo ok            # exec
   ```

Once one refreshed `ord-devimprint-admin` kubeconfig authenticates, it grants
**both** secret-read and exec — run the §3/§4 recipes; no re-derivation needed.

---

## Appendix — probe evidence (why not to re-probe)

Every ord-devimprint kubeconfig was exhaustively tested for both operations on
2026-08-09 (each with a 30–45s timeout — stale Spot OIDC tokens HANG rather than
fail fast). Auth fails at the API layer, so it blocks **both** operations
identically.

| Kubeconfig | Context (user) | secret-read | exec | Root cause |
|---|---|---|---|---|
| `ord-devimprint-observer.kubeconfig` | observer (`devpod-observer` SA) | ❌ `403 Forbidden` (by design) | ❌ `403 Forbidden` (by design) | read-only RBAC — never grants these |
| `ord-devimprint-admin.kubeconfig` | `apexalgo-ord-devimprint` (`ngpc-user` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static bootstrap token expired |
| `ord-devimprint-admin.kubeconfig` | `apexalgo-ord-devimprint-oidc` (`oidc` exec) | ❌ TIMEOUT | ❌ TIMEOUT | OIDC cache empty + no browser headless |
| `ord-devimprint.kubeconfig` | `apexalgo-ord-devimprint` (`ngpc-user` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static token expired |
| `ord-devimprint-token.kubeconfig` | `token-context` (`ngpc-user` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static token expired |

No other cluster's kubeconfig can reach the namespace (`commitgraph` exists only
on ord-devimprint); cross-cluster RBAC is not a thing here. Direct API server
reachability was confirmed independently (unauth curl returns `http_code=403`,
i.e. the control plane answers — this is an auth failure, not DNS/network).

**Bottom line:** there is no working secret-read OR exec path today. The
read-only observer is the only working kubeconfig and grants neither. Re-probing
is wasted effort until §6 unblock.
