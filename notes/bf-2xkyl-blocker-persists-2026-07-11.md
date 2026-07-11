# bf-2xkyl: Blocker Persists (2026-07-11)

## Task: Retrieve S3 credentials from armor-writer secret

## Status: BLOCKED - Cannot complete without kubeconfig with secret access

## Investigation Summary

Verified on 2026-07-11 that the blocker identified in previous investigations remains unresolved:

### Missing Kubeconfigs
1. **ord-devimprint kubeconfig**: NOT FOUND at `~/.kube/ord-devimprint.kubeconfig`
2. **rs-manager kubeconfig**: NOT FOUND at `~/.kube/rs-manager.kubeconfig`

### Available Kubeconfigs
Only 2 kubeconfigs exist on this system:
- `~/.kube/iad-acb.kubeconfig`
- `~/.kube/iad-ci.kubeconfig`

Neither provides access to ord-devimprint or rs-manager clusters.

### Read-Only Proxy Status

**ord-devimprint proxy** (kubectl-proxy-ord-devimprint:8001):
- ✅ Service accessible (can list pods)
- ❌ Secret access: EXPLICITLY BLOCKED
- Error: `User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets" in namespace "devimprint"`

**rs-manager proxy** (traefik-rs-manager:8001):
- ✅ Service accessible
- ❌ Secret access: EXPLICITLY BLOCKED for all namespaces

### Prerequisite Bead Status

**bf-2p1wr** (Obtain ord-devimprint kubeconfig with write access):
- Status: **CLOSED**
- Actual kubeconfig obtained: **NONE**
- Conclusion: Bead was incorrectly marked as completed

### Acceptance Criteria - NOT MET

- ❌ Successfully retrieved `auth-access-key` value (base64-decoded)
  *Note: The actual key name is `auth-access-key`, not `LITESTREAM_ACCESS_KEY_ID` as the task suggests*
- ❌ Successfully retrieved `auth-secret-key` value (base64-decoded)
  *Note: The actual key name is `auth-secret-key`, not `LITESTREAM_SECRET_ACCESS_KEY`*
- ❌ Credentials stored temporarily in a secure location

### Commands That FAIL

```bash
# No kubeconfig exists
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
# Error: stat /home/coding/.kube/ord-devimprint.kubeconfig: no such file or directory

# Read-only proxy denies secret access
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
# Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

### What Would Work (if access existed)

```bash
# Direct ord-devimprint kubeconfig (if it existed)
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-access-key}' | base64 -d

kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint \
  -o jsonpath='{.data.auth-secret-key}' | base64 -d
```

## Root Cause

The prerequisite bead **bf-2p1wr** was marked as **closed** but never actually obtained the required kubeconfig with secret-read access. This is a false completion - the acceptance criteria for that bead (verify kubeconfig works before proceeding) were never met.

## Conclusion

**This bead CANNOT be completed without proper cluster access.**

The task requires retrieving secret values from Kubernetes, but:
1. No kubeconfig with appropriate permissions exists
2. Read-only proxy access explicitly denies secret access
3. No alternative access method to credentials is available

## Required Action

This bead should remain **OPEN and BLOCKED** until:
1. Actual ord-devimprint kubeconfig with secret-read access is obtained, OR
2. Actual rs-manager kubeconfig with secret-read access is obtained (for OpenBao access), OR
3. Credentials are provided through an alternative secure channel

## Recommendation

**DO NOT CLOSE this bead.** The bead should be automatically released for retry once the prerequisite kubeconfig access is properly established.

## References

- Prerequisite bead: bf-2p1wr (incorrectly marked closed)
- Previous blocker investigations: notes/bf-2xkyl-blocker-confirmed*.md
- ExternalSecret config: ~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml
