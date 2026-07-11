# bf-2xkyl Blocker: Missing ord-devimprint Kubeconfig

## Current Situation

**Task**: bf-2xkyl - Retrieve S3 credentials from armor-writer secret
**Prerequisite**: bf-2p1wr - Obtain ord-devimprint kubeconfig with write access

## Blocker Details

The prerequisite bead `bf-2p1wr` is marked as **closed** but the kubeconfig file **does not exist**:

```bash
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory
```

### Verification Attempt (Read-Only Proxy)

Attempted to use the read-only proxy, but it cannot access secret data:

```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get secret armor-writer -n devimprint \
  -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'

Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" 
cannot get resource "secrets" in API group "" in the namespace "devimprint"
```

### Root Cause

The `devpod-observer` ServiceAccount only has `verbs: ["list"]` for secrets, NOT `get`.
This means it can list secret names but cannot read secret contents.

## Required Actions

To complete bf-2xkyl, we need:

1. **Obtain ord-devimprint kubeconfig with write access**
   - Access Rackspace Spot portal (https://spot.rackspace.com)
   - Download admin kubeconfig for cluster `hcp-5f30c973-cde7-42d9-8c7b-5d0573821330`
   - Create ServiceAccount with secret-read permissions in devimprint namespace
   - Store kubeconfig at `~/.kube/ord-devimprint.kubeconfig`

2. **Alternative: Re-open and complete bf-2p1wr**
   - The bead was marked closed but acceptance criteria were not met
   - Documentation from bf-2p1wr clearly states "INCOMPLETE - Requires External Coordination"

## Acceptance Status

**bf-2xkyl Acceptance Criteria NOT met:**
- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)
- [ ] Credentials are stored temporarily in a secure location

## Next Steps

This task cannot be completed without the kubeconfig. Options:
1. Coordinate with cluster administrator to obtain ord-devimprint kubeconfig
2. Access Rackspace Spot portal to download admin kubeconfig
3. Re-open and properly complete bf-2p1wr first

## Cluster Information

- **Cluster**: ord-devimprint (Rackspace Spot)
- **Server**: `https://hcp-5f30c973-cde7-42d9-8c7b-5d0573821330.spot.rackspace.com`
- **Target Secret**: `armor-writer` in namespace `devimprint`
- **Required Keys**: `LITESTREAM_ACCESS_KEY_ID`, `LITESTREAM_SECRET_ACCESS_KEY`

## Date: 2026-07-11
