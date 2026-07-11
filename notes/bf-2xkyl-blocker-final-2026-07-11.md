# bf-2xkyl: BLOCKER - Missing ord-devimprint Kubeconfig Access

## Task Status: BLOCKED - Cannot Complete

**Task**: Retrieve S3 credentials from armor-writer secret in devimprint namespace

## Blocker Summary

This task requires access to read secrets from the ord-devimprint cluster. The prerequisite bead (bf-2p1wr) was marked as closed but never actually obtained the required kubeconfig.

## Current State (2026-07-11)

### Missing Access

| Required Access | Status | Details |
|-----------------|--------|---------|
| `~/.kube/ord-devimprint.kubeconfig` | ❌ Does not exist | Prerequisite bead bf-2p1wr was closed without providing this |
| Read-only proxy `kubectl-proxy-ord-devimprint:8001` | ❌ Forbidden | devpod-observer SA lacks permission to get secrets |

### Verification

```bash
# Check for kubeconfig
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory

# Attempted access via read-only proxy
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): secrets "armor-writer" is forbidden: 
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" 
in API group "" in the namespace "devimprint"
```

## Secret Details

### What We Need to Retrieve

The `armor-writer` secret in the `devimprint` namespace contains the actual S3 credentials:

**Secret Name**: `armor-writer`  
**Namespace**: `devimprint`  
**Data Keys**:
- `auth-access-key` (maps to env var `LITESTREAM_ACCESS_KEY_ID`)
- `auth-secret-key` (maps to env var `LITESTREAM_SECRET_ACCESS_KEY`)

**Note**: The task instructions reference `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY` as the secret data keys, but these are actually the environment variable names used in deployments. The actual secret data keys are `auth-access-key` and `auth-secret-key`.

### ExternalSecret Configuration

The secret is synced from OpenBao via ExternalSecret:

```yaml
# ExternalSecret: armor-writer
# OpenBao path: rs-manager/ord-devimprint/armor-writer
# Sync interval: 1h

Data mapping:
- auth-access-key → OpenBao property auth-access-key
- auth-secret-key → OpenBao property auth-secret-key
```

### Usage in Deployments

These credentials are used for Litestream S3 backup in deployments like `queue-api`:

```yaml
env:
  - name: LITESTREAM_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: armor-writer
        key: auth-access-key
  - name: LITESTREAM_SECRET_ACCESS_KEY
    valueFrom:
      secretKeyRef:
        name: armor-writer
        key: auth-secret-key
```

## Acceptance Criteria - NOT MET

- ❌ Successfully retrieved auth-access-key value (base64-decoded)
- ❌ Successfully retrieved auth-secret-key value (base64-decoded)
- ❌ Credentials stored in secure temporary location

## Root Cause Analysis

1. **Prerequisite bead bf-2p1wr** ("Obtain ord-devimprint kubeconfig with write access") was marked as closed
2. However, the kubeconfig file was **never actually created** on this system
3. The acceptance criteria for bf-2p1wr were not actually met:
   - ❌ Kubeconfig file for ord-devimprint cluster is obtained
   - ❌ Kubeconfig has permissions to read secrets in the devimprint namespace
   - ❌ Can successfully run: kubectl get secrets -n devimprint
4. This bead (bf-2xkyl) has been attempted **10+ times** with the same blocker
5. Each attempt has documented the issue but the prerequisite remains unresolved

## What is Required to Complete

ONE of the following must be provided:

### Option 1: ord-devimprint kubeconfig (RECOMMENDED)
- **File**: `~/.kube/ord-devimprint.kubeconfig`
- **Required permissions**: Read secrets in `devimprint` namespace
- **Source**: Rackspace Spot console or cluster administrator

### Option 2: Direct S3 credentials
- The actual `auth-access-key` and `auth-secret-key` values
- Can be provided directly to avoid cluster access

### Option 3: Alternative access method
- OpenBao access to retrieve from path `rs-manager/ord-devimprint/armor-writer`
- Requires OpenBao authentication

## Alternative Approach: Retrieve from OpenBao Directly

The credentials are stored in OpenBao at path `rs-manager/ord-devimprint/armor-writer`. However, accessing OpenBao also requires authentication, which faces the same blocker - the read-only proxy cannot access OpenBao tokens.

## Documentation of Previous Attempts

This bead has been attempted multiple times (10+), with extensive documentation:
- `.beads/traces/bf-2xkyl/` - Multiple trace files from attempted completions
- `notes/bf-2xkyl-blocker-*.md` - Multiple blocker confirmations
- Git commits: d5bdafc, f26884a, bceef71, a9a57ca, cf1d5a6, 93d1b13, 25d12e8, 4535af4, 38ea984, 442010a
- Bead comments documenting the persistent blocker
- Previous agent runs all confirmed the same blocker

## Action Taken (Per Instructions)

Per bead instructions:
> "If you cannot complete the task OR cannot produce a commit:
> - Do NOT close the bead
> - The bead will be automatically released for retry"

**Action**: NOT closing bead bf-2xkyl - leaving it open for retry once cluster access is available

## Next Steps Required

1. **Re-open prerequisite bead bf-2p1wr** - It was closed incorrectly without completing the work
2. **Actually obtain kubeconfig** for ord-devimprint cluster (via Rackspace Spot console or cluster administrator)
3. **Save to** `~/.kube/ord-devimprint.kubeconfig` with appropriate permissions (`chmod 600`)
4. **Verify access**: Run `kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secrets -n devimprint`
5. **Retry this task** once kubeconfig is available

## Timestamp

Blocker confirmed: 2026-07-11 (Final verification after 10+ failed attempts)
