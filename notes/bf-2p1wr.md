# Bead bf-2p1wr: Obtain ord-devimprint kubeconfig with write access

## Current Situation

The ord-devimprint cluster currently has only a **read-only kubectl proxy** available:

- **Proxy endpoint:** `http://kubectl-proxy-ord-devimprint:8001`
- **ServiceAccount:** `system:serviceaccount:devpod-observer:devpod-observer`
- **Permissions:** Read-only, **explicitly denies access to secrets**

### Verification

```bash
# Check if proxy can access secrets
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 auth can-i get secrets -n devimprint
no

# Try to access armor-writer secret
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
in API group "" in the namespace "devimprint"
```

### Target Secret

The secret we need to access exists:
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE             DATA   AGE
armor-writer            Opaque           2      80d
```

## What's Needed

A kubeconfig file with sufficient permissions to read secrets in the devimprint namespace. Based on patterns from other clusters, this should be stored as:
- **Path:** `~/.kube/ord-devimprint.kubeconfig`
- **Required permissions:** Ability to read secrets in the `devimprint` namespace

## Blocker: Requires Rackspace Spot Console Access

The ord-devimprint cluster is a **Rackspace Spot cluster** (evident from cluster annotations). Obtaining a kubeconfig requires:

1. Access to the **Rackspace Spot console** at https://spot.rackspace.com
2. Navigating to the ord-devimprint cluster settings
3. Downloading the kubeconfig file via the Spot UI

### Cluster Details

- **Cluster name:** ord-devimprint
- **Provider:** Rackspace Spot
- **Region:** ORD (Chicago)
- **Organization ID:** `org-kselolwaoxl3zxfm` (from cluster annotations)
- **Cluster ID:** `c-5f30c973` (from cluster annotations)

## Resolution Path

This bead requires coordination with someone who has:
- Rackspace Spot console access
- Permissions to download kubeconfigs for the ord-devimprint cluster

Once obtained, the kubeconfig should be:
1. Saved to `~/.kube/ord-devimprint.kubeconfig`
2. Secured with appropriate permissions (`chmod 600`)
3. Tested with: `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint`

## Similar Clusters

For reference, other clusters with write-access kubeconfigs:
- `~/.kube/ardenone-manager.kubeconfig` - Management cluster
- `~/.kube/rs-manager.kubeconfig` - Rackspace Spot management cluster
- `~/.kube/iad-ci.kubeconfig` - CI/CD cluster
- `~/.kube/iad-acb.kubeconfig` - AI Code Battle cluster

## Date

2026-07-11
