# bf-2xkyl: BLOCKER - Missing Kubeconfig Access (2026-07-11)

## Task Status: BLOCKED

Task: Retrieve S3 credentials from armor-writer secret in devimprint namespace

## Blocker Confirmed

### Required Access Missing

1. **ord-devimprint kubeconfig** (`~/.kube/ord-devimprint.kubeconfig`)
   - Required for direct access to ord-devimprint cluster
   - Does NOT exist on system
   - Prerequisite bead bf-2p1wr was marked closed but kubeconfig was never obtained

2. **rs-manager kubeconfig** (`~/.kube/rs-manager.kubeconfig`)
   - Could provide alternative access via OpenBao
   - Does NOT exist on system
   - Documented in CLAUDE.md but not present

3. **Read-only proxy access** (`kubectl-proxy-ord-devimprint:8001`)
   - Exists but CANNOT access secrets
   - Verified: Returns "User 'system:serviceaccount:devpod-observer:devpod-observer' cannot get resource 'secrets'"

### Verification of Blocker

```bash
# Attempt 1: Direct kubeconfig (doesn't exist)
$ kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
error: stat ~/.kube/ord-devimprint.kubeconfig: no such file or directory

# Attempt 2: rs-manager kubeconfig (doesn't exist)
$ kubectl --kubeconfig=~/.kube/rs-manager.kubeconfig get secret armor-writer -n devimprint
error: stat ~/.kube/rs-manager.kubeconfig: no such file or directory

# Attempt 3: Read-only proxy (exists but forbidden)
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
```

### Secret Key Mapping Issue

The task requests `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`, but investigation reveals:

**ExternalSecret armor-writer** defines:
- `auth-access-key` (not LITESTREAM_ACCESS_KEY_ID)
- `auth-secret-key` (not LITESTREAM_SECRET_ACCESS_KEY)

This suggests either:
1. The task description uses wrong key names
2. The ExternalSecret is missing the correct keys
3. There's a mismatch in the configuration

## Acceptance Criteria - NOT MET

- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)
- [ ] Credentials are stored temporarily in a secure location

## What is Required to Complete

To complete this task, ONE of the following must be provided:

### Option 1: ord-devimprint Kubeconfig
```bash
# Location: ~/.kube/ord-devimprint.kubeconfig
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data}'
```

### Option 2: rs-manager Kubeconfig
Access OpenBao at `rs-manager/ord-devimprint/armor-writer` path

### Option 3: Direct Credentials
Provide the actual credential values

## Recommendation

**DO NOT CLOSE this bead.** The task cannot be completed without proper cluster access.

The prerequisite bead (bf-2p1wr) needs to be revisited to actually obtain the ord-devimprint kubeconfig.

## References

- Prerequisite bead: bf-2p1wr (marked closed but incomplete)
- ExternalSecret config: ~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml
- Cluster documentation: CLAUDE.md (Kubernetes Access section)
- Prior blocker documentation: notes/bf-2xkyl-blocker-confirmed.md

## Timestamp

Blocker confirmed: 2026-07-11
