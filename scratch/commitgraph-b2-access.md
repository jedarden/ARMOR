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
