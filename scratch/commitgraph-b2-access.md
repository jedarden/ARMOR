# commitgraph namespace — host cluster & access surface inventory

**Recon date:** 2026-08-09
**Bead:** bf-3hbw6f (de-risking reconnaissance for downstream commitgraph / B2 work)
**Access used:** READ-ONLY only (no exec, no secret values read). Observer SA.

---

## 1. Host cluster: `ord-devimprint`

The `commitgraph` namespace lives on the **`ord-devimprint`** cluster. Probed every
candidate cluster with `kubectl get ns commitgraph` (read-only). Result matrix:

| Cluster | Path tried | Result |
|---|---|---|
| ord-devimprint | observer kubeconfig **AND** kubectl-proxy | ✅ **FOUND** (Active, 17h old) |
| apexalgo-iad | proxy `traefik-apexalgo-iad:8001` | NotFound |
| ardenone-cluster | proxy `traefik-ardenone-cluster:8001` | NotFound |
| ardenone-manager | proxy `traefik-ardenone-manager:8001` | NotFound |
| rs-manager | proxy `traefik-rs-manager:8001` | NotFound |
| iad-ci | kubeconfig `iad-ci.kubeconfig` | NotFound |
| iad-kalshi | proxy `kubectl-proxy-iad-kalshi:8001` | NotFound |
| iad-options | observer kubeconfig | NotFound |
| iad-native-ads | — | DECOMMISSIONED (not probed) |

> Prior notes (bf-57d3fx / bf-3dntjx) found queue-api in `commitgraph` via an
> all-namespaces pod grep but never recorded which cluster that ran against.
> It was **ord-devimprint**.

### Reusable read-only command lines

```bash
# Primary (observer kubeconfig, long-lived SA token — never expires)
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-observer.kubeconfig get pods -n commitgraph

# Alt (read-only kubectl-proxy via Tailscale operator)
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n commitgraph

# All-namespaces grep (the original discovery path):
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-observer.kubeconfig get pods --all-namespaces | grep queue-api
```

For work needing **read/write or secret values** (downstream children): the admin
kubeconfig is `/home/coding/.kube/ord-devimprint-admin.kubeconfig`
(`cloudspace-admin` OIDC token, ~3 day expiry — regenerate from Spot UI).

---

## 2. queue-api pod (re-confirmed — pods rotate)

| Field | Value |
|---|---|
| Pod name | `queue-api-c5894c469-pzjsl` |
| Status | Running, 2/2, 0 restarts, ~17h |
| Prior (bf-57d3fx) | `queue-api-c5894c469-p9rhr` → **rotated** (suffix changed), as expected |
| Deployment | `queue-api` (1/1 ready, replica 1) |
| Node | `prod-instance-17862171401240367` |
| Pod IP | `10.20.59.197` |
| Service `queue-api` | ClusterIP `10.21.222.173:8080` |

> Pod names rotate on rollout. The stable handle is the Deployment `queue-api`
> or `kubectl get pods -n commitgraph -l app=queue-api`.

### Containers on the pod

| Container | Image | Role |
|---|---|---|
| `queue-api` | `ronaldraygun/commitgraph-queue-api:2.8.0` | main app |
| **`litestream`** ✅ | `litestream/litestream:0.5.11` | **sidecar (CONFIRMED name = `litestream`)** |
| `init-schema` (init) | — | schema init |

**Litestream sidecar container name = `litestream`** (confirmed against live pod spec).

---

## 3. ConfigMap `queue-api-litestream-config` — EXISTS ✅

```
NAME                          DATA   AGE
queue-api-litestream-config   1      17h
```

Lives in `commitgraph`. (Contents already documented in bf-57d3fx: backs up
`/data/queue.db` → B2 bucket `commitgraph-ops`, path `queue-api/queue.db`,
endpoint `https://s3.us-west-002.backblazeb2.com`.)

---

## 4. Secret `commitgraph-b2-workers` — expected DENIAL recorded

The observer SA **can list** secrets (metadata + data-key count only) but
**cannot `get`** an individual secret. Both behaviors observed:

### List (allowed — metadata only)

```
$ kubectl --kubeconfig=...ord-devimprint-observer.kubeconfig get secrets -n commitgraph
NAME                         TYPE                             DATA   AGE
armor-mek                    Opaque                           1      17h
armor-writer                 Opaque                           2      17h
b2-aggregator                Opaque                           5      17h
b2-aggregator-web            Opaque                           5      17h
b2-compact                   Opaque                           5      17h
b2-filter-worker             Opaque                           5      17h
commitgraph-b2-workers       Opaque                           5      17h
commitgraph-db-app           kubernetes.io/basic-auth         11     17h
commitgraph-db-ca            Opaque                           2      17h
commitgraph-db-replication   kubernetes.io/tls                2      17h
commitgraph-db-server        kubernetes.io/tls                2      17h
corpus-keyring               Opaque                           1      17h
devimprint-b2-workers        Opaque                           5      17h
docker-hub-registry          kubernetes.io/dockerconfigjson   1      17h
github-pat                   Opaque                           1      17h
queue-api-auth               Opaque                           2      17h
```

### Get of `commitgraph-b2-workers` (DENIED — expected, not a failure)

```
$ kubectl --kubeconfig=...ord-devimprint-observer.kubeconfig get secret commitgraph-b2-workers -n commitgraph
Error from server (Forbidden): secrets "commitgraph-b2-workers" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource
"secrets" in API group "" in the namespace "commitgraph"
```

This denial is **expected evidence**. Reading the secret VALUES requires the
admin kubeconfig (`/home/coding/.kube/ord-devimprint-admin.kubeconfig`,
OIDC ~3 day expiry) — the downstream task should plan for that.

### Secret consumption (from deployment env refs — read-only, observable)

`commitgraph-b2-workers` is consumed by **both** containers on the queue-api pod:
- `queue-api` container: 4 keys via `valueFrom.secretKeyRef`
- `litestream` container: 2 keys via `valueFrom.secretKeyRef`
  (the `LITESTREAM_ACCESS_KEY_ID` / `LITESTREAM_SECRET_ACCESS_KEY` envs from bf-57d3fx)

---

## 5. Other namespace resources (context for downstream)

**Postgres (CloudNativePG) cluster:** `commitgraph-db` — pod `commitgraph-db-1`,
3 services (`-r`/`-ro`/`-rw`) on `:5432`. DB creds in `commitgraph-db-app`
(basic-auth).

**ConfigMaps:** `admin-alias-map`, `bot-classification`, `cnpg-default-monitoring`,
`queue-api-litestream-config`, `kube-root-ca.crt`.

**B2 worker secret family present:** `commitgraph-b2-workers`, `devimprint-b2-workers`,
`b2-aggregator`, `b2-aggregator-web`, `b2-compact`, `b2-filter-worker`
(each Opaque, 5 data keys — same shape).

**ARMOR-related secrets present:** `armor-mek` (master encrypt key), `armor-writer`.

**ServiceAccounts:** `commitgraph-db`, `default`.

---

## Summary for downstream children

- **Host cluster:** `ord-devimprint`
- **Read-only access:** `kubectl --kubeconfig=/home/coding/.kube/ord-devimprint-observer.kubeconfig ... -n commitgraph`
  (alt: `--server=http://kubectl-proxy-ord-devimprint:8001`)
- **queue-api pod:** `queue-api-c5894c469-pzjsl` (rotates — use `-l app=queue-api` or the Deployment)
- **litestream container:** `litestream` (image `litestream/litestream:0.5.11`)
- **ConfigMap:** `queue-api-litestream-config` ✅ exists
- **Secret `commitgraph-b2-workers`:** list-OK / get-DENIED on observer; values need admin kubeconfig

---

## Appendix A — exec + secret-read kubeconfig probe (bf-3hvieu, 2026-08-09)

**Outcome: BLOCKED — no live kubeconfig grants secret-read OR exec against `commitgraph`.**
This is the gate the prior attempts (bf-1q3s0v etc.) never cleared, and it is still
closed. Detailed probe + exact remediation below. This blocker report is the
closable deliverable for bf-3hvieu.

### Probe method

Re-confirmed the live pod first (observer SA, read-only, never expires):

```
queue-api-c5894c469-pzjsl   2/2 Running  0  17h   (Deployment queue-api; use -l app=queue-api)
litestream sidecar container = litestream  (unchanged)
```

Two operations tested, each with a 35–45s timeout (stale Spot OIDC tokens HANG
rather than failing fast — the timeout is load-bearing):

- **(a) secret-read:** `kubectl <kc> --context=<ctx> get secret commitgraph-b2-workers -n commitgraph -o jsonpath='{.data}'`
- **(b) exec:**          `kubectl <kc> --context=<ctx> exec queue-api-c5894c469-pzjsl -c litestream -n commitgraph -- echo ok`

### Result matrix — every ord-devimprint direct/admin kubeconfig

| Kubeconfig | Context (user) | secret-read | exec | Root cause |
|---|---|---|---|---|
| `ord-devimprint-token.kubeconfig` | `token-context` (`ngpc-user` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static bootstrap token expired |
| `ord-devimprint-admin.kubeconfig` | `apexalgo-ord-devimprint` (`ngpc-user-apex…` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static bootstrap token expired |
| `ord-devimprint.kubeconfig` | `apexalgo-ord-devimprint` (`ngpc-user` static) | ❌ `401 Unauthorized` | ❌ `401 Unauthorized` | static bootstrap token expired |
| `ord-devimprint-admin.kubeconfig` | `apexalgo-ord-devimprint-oidc` (`oidc` exec) | ❌ TIMEOUT (no token) | ❌ TIMEOUT (no token) | OIDC cache empty + no browser headless |
| `ord-devimprint.kubeconfig` | `apexalgo-ord-devimprint-oidc` (`oidc` exec) | ❌ TIMEOUT (no token) | ❌ TIMEOUT (no token) | OIDC cache empty + no browser headless |
| `ord-devimprint-observer.kubeconfig` | observer (`devpod-observer` SA) | ❌ `403 Forbidden` (known) | ❌ `403 Forbidden` (known) | read-only RBAC by design — never grants these |

Auth fails at the API layer, so it blocks **both** operations identically.

### Evidence (representative, truncated)

Static token path — fast 401 (server IS reachable; the token is the problem):
```
$ kubectl --kubeconfig=...ord-devimprint-admin.kubeconfig --context=apexalgo-ord-devimprint \
    get secret commitgraph-b2-workers -n commitgraph -o jsonpath='{.data}'
error: You must be logged in to the server (Unauthorized)
```
Direct API server reachability confirmed independently — unauth curl returns
`http_code=403`, i.e. the control plane answers; this is an auth failure, not a
network/DNS problem.

OIDC path — hangs then times out (cache empty, no browser):
```
error: could not open the browser: exec: "xdg-open,x-www-browser,www-browser": executable file not found in $PATH
Please visit the following URL in your browser manually: http://localhost:8000/
error: get-token: authentication error: … authorization error: context canceled   (timeout 124)
```
OIDC token cache state (`--token-cache-dir=~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm`):
```
2026-05-03 06:46  0  …/90e1…cc6.lock          (0-byte lock, no token blob)
2026-08-07 19:21  0  …/org_KsELolwAOxl3Zxfm/90e1…cc6.lock   (0-byte lock, no token blob)
```
No cached access/refresh token exists anywhere under `~/.kube/cache/oidc-login/`, so
`kubectl oidc-login get-token` must run the interactive browser authorization-code
flow — impossible on this headless Hetzner box (and per bf-1q3s0v, Spot's OIDC has
**no CLI refresh** path).

### Why the other-cluster admin kubeconfigs are not candidates

`commitgraph` exists ONLY on ord-devimprint (recon §1). Every other admin/direct
kubeconfig in `~/.kube/` points at a different API server where the namespace is
`NotFound`:

| Kubeconfig | Points at | `get ns commitgraph` |
|---|---|---|
| `apexalgo-iad.kubeconfig`, `apexalgo-iad-alpha/-ts` | apexalgo-iad | NotFound |
| `ardenone-manager-temp.kubeconfig` | ardenone-manager | NotFound |
| `rs-manager.kubeconfig` (+ `.bak*`) | rs-manager | NotFound |
| `iad-ci.kubeconfig` | iad-ci | NotFound |
| `iad-options.kubeconfig`, `iad-options-observer` | iad-options | NotFound |
| `iad-kalshi(-admin).kubeconfig`, default `~/.kube/config` | iad-kalshi | NotFound (and token also dead) |
| `iad-native-ads*` | decommissioned 2026-07-27 | cluster gone |

Cross-cluster RBAC is not a thing here; only ord-devimprint kubeconfigs can reach
the namespace, and all of those fail auth (matrix above).

### Exact remediation needed (operator self-service — mirrors bf-1q3s0v)

The token is NOT something this headless agent can refresh. The operator (jedarden)
must:

1. **Rackspace Spot dashboard → ord-devimprint cloudspace → generate fresh admin
   kubeconfig** (regenerates the `cloudspace-admin` OIDC credential). Overwrite
   `~/.kube/ord-devimprint-admin.kubeconfig` (`chmod 600`).
2. **Prime the OIDC token cache once from a browser-capable machine** — run any
   `kubectl --kubeconfig=…ord-devimprint-admin.kubeconfig …` (e.g. `get ns`) and
   complete the `login.spot.rackspace.com` SSO in a browser. This writes the access
   + refresh token into `~/.kube/cache/oidc-login/org_KsELolwAOxl3Zxfm/`, after
   which the OIDC `exec` path works headless until the refresh token lapses.
   - Shortcut: the freshly-downloaded kubeconfig also carries a **static
     `ngpc-user` token** that works headless immediately — but it is short-lived
     (the Aug-07 20:01 regeneration's static token was already `Unauthorized` by
     2026-08-09, i.e. < ~37h). Treat it as a temporary unlock, not a fix.
3. **Verify both operations** before handing off:
   ```
   kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig \
       get secret commitgraph-b2-workers -n commitgraph -o jsonpath='{.data}'   # (a) secret-read
   kubectl --kubeconfig=~/.kube/ord-devimprint-admin.kubeconfig \
       exec -n commitgraph -l app=queue-api -c litestream -- echo ok             # (b) exec
   ```

Once a single refreshed `ord-devimprint-admin` kubeconfig authenticates, it grants
**both** secret-read and exec (same `cluster-admin` OIDC identity) — no separate
kubeconfig per operation is needed.

### Bottom line for downstream children

- **No working kubeconfig today.** Both Path A (pod-exec on queue-api/litestream)
  and Path B (secret-read on commitgraph-b2-workers) are blocked by the SAME
  auth failure on ord-devimprint.
- **Unblock = Spot-dashboard OIDC refresh + cache prime** (operator action).
- Until then, the **only** working access is the read-only observer
  (`ord-devimprint-observer.kubeconfig`), which grants neither secret-read nor exec.
