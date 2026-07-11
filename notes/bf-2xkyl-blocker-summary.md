# Blocker: bf-2xkyl - Prerequisite kubeconfig still missing

## Current Situation

Attempted to complete bf-2xkyl (Retrieve S3 credentials from armor-writer secret) but cannot proceed due to missing prerequisite.

## Prerequisite Issue

**Bead**: bf-2p1wr (Obtain ord-devimprint kubeconfig with write access)
**Status**: Closed (incorrectly - actual work incomplete)
**Required File**: `~/.kube/ord-devimprint.kubeconfig`
**Actual State**: FILE DOES NOT EXIST

## Verification

```bash
$ ls -la /home/coding/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory

$ find /home/coding/.kube -name "*.kubeconfig"
/home/coding/.kube/iad-acb.kubeconfig
/home/coding/.kube/iad-ci.kubeconfig
```

Only two kubeconfigs exist, neither for ord-devimprint.

## Current Access

**Read-only proxy**: `kubectl-proxy-ord-devimprint:8001`
- Can list secrets: YES
- Can read secret contents: NO (Forbidden by RBAC)

```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secrets -n devimprint
NAME                    TYPE             DATA   AGE
armor-writer            Opaque           2      79d

$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint -o json
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

## Required Action

The ord-devimprint kubeconfig must be obtained via:

1. **Rackspace Spot Portal** (recommended):
   - Login to https://spot.rackspace.com
   - Download admin kubeconfig for ord-devimprint cluster
   - Create serviceaccount with secret-read permissions
   - Store at `~/.kube/ord-devimprint.kubeconfig`

2. **Or coordinate with cluster administrator** who has Rackspace Spot access

## Pattern from Other Clusters

The iad-ci cluster has a working kubeconfig at `~/.kube/iad-ci.kubeconfig` with cluster-admin access. A similar setup is needed for ord-devimprint.

## ExternalSecret Reference

The cluster credentials are stored in OpenBao at `secret/rs-manager/ord-devimprint/cluster` for ArgoCD, but this does not provide direct kubectl access.

## Acceptance Criteria NOT Met

- [ ] Kubeconfig exists at `~/.kube/ord-devimprint.kubeconfig`
- [ ] Can read secrets in devimprint namespace
- [ ] Can retrieve armor-writer secret contents

## Next Steps

1. **DO NOT CLOSE bf-2xkyl** - prerequisite not met
2. **Coordinate with user** to obtain ord-devimprint kubeconfig
3. **Re-open bf-2p1wr** or create new bead for kubeconfig acquisition
4. **Re-attempt bf-2xkyl** once kubeconfig is available

## Date

2026-07-11
