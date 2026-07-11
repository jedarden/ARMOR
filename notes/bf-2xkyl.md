# Bead bf-2xkyl: Retrieve S3 credentials from armor-writer secret - BLOCKED

## Status
**BLOCKED** - Prerequisite bead bf-2p1wr did not provide required kubeconfig

## Issue
Bead bf-2p1wr ("Obtain ord-devimprint kubeconfig with write access") shows as **closed**, but the required kubeconfig file does not exist:

```bash
# No kubeconfig exists:
$ ls /home/coding/.kube/*ord*devimprint*
# (no files found)

# Read-only proxy explicitly denies secret access:
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i get secrets -n devimprint
no

$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): secrets "armor-writer" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

## What's Needed
1. A kubeconfig file with write access to ord-devimprint cluster (able to read secrets)
2. The kubeconfig should be stored at `~/.kube/ord-devimprint.kubeconfig` or similar
3. Must be able to run: `kubectl get secret armor-writer -n devimprint`

## Verification Summary (2026-07-11)

Attempted to complete this bead but encountered blocker:

**Available kubeconfigs:**
- `~/.kube/iad-acb.kubeconfig` (not relevant)
- `~/.kube/iad-ci.kubeconfig` (not relevant)
- **MISSING**: `~/.kube/ord-devimprint.kubeconfig` (required)

**Read-only proxy access:**
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE             DATA   AGE
armor-writer            Opaque           2      79d  # ← Target secret exists
admin-oauth             Opaque           3      62d
armor-credentials       Opaque           7      79d
armor-readonly          Opaque           2      79d
```

**RBAC constraint:**
```
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

The devpod-observer ServiceAccount has `verbs: ["list"]` for secrets but NOT `get`, meaning it can list secret names but cannot read their contents.

## Acceptance Criteria Status

- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)
- [ ] Credentials are stored temporarily in a secure location

**ALL CRITERIA NOT MET** - cannot proceed without kubeconfig with secret-read permissions.

## Comment Added to Bead

Added comment #33 to bead bf-2xkyl documenting this blocker.

## Date

2026-07-11

## Next Steps

1. **Coordinate with user** to obtain ord-devimprint kubeconfig via:
   - Rackspace Spot Portal (https://spot.rackspace.com)
   - Or cluster administrator who can provide credentials

2. **Re-open and complete bf-2p1wr** to create the kubeconfig

3. **Re-attempt bf-2xkyl** once kubeconfig is available

## Commands to Run Once Kubeconfig is Available

```bash
# Retrieve LITESTREAM_ACCESS_KEY_ID
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}' | base64 -d

# Retrieve LITESTREAM_SECRET_ACCESS_KEY
kubectl --kubeconfig=/home/coding/.kube/ord-devimprint.kubeconfig \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_SECRET_ACCESS_KEY}' | base64 -d
```
