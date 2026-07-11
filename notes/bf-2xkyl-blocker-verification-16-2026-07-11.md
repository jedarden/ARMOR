# Task: Retrieve S3 credentials from armor-writer secret
# Bead: bf-2xkyl
# Date: 2026-07-11

## BLOCKER: Cannot Complete Task - Missing Prerequisite

### Prerequisite Status: FAILED

Bead **bf-2p1wr** (Obtain ord-devimprint kubeconfig with write access) was marked as `closed` but is actually **INCOMPLETE**.

**Evidence:**
1. No kubeconfig file exists at `~/.kube/ord-devimprint.kubeconfig`
2. Notes from bf-2p1wr confirm the bead was closed without meeting acceptance criteria
3. The notes explicitly state: "INCOMPLETE - Requires External Coordination"

### Why This Task Cannot Be Completed

To retrieve the `armor-writer` secret from ord-devimprint, I need one of:

1. **Direct kubeconfig with secret read access** to ord-devimprint ← DOES NOT EXIST
2. **Access to OpenBao on rs-manager** where the source credentials are stored ← NOT AVAILABLE

**Current access situation:**
- `kubectl-proxy-ord-devimprint:8001` - Read-only proxy, explicitly denies secret access
- `~/.kube/iad-ci.kubeconfig` - Only has access to iad-ci cluster
- `~/.kube/iad-acb.kubeconfig` - Only has access to iad-acb cluster
- No kubeconfig for ord-devimprint or rs-manager

### What Was Attempted

```bash
# Attempt 1: Read-only proxy (fails - no secret access)
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
    get secret armor-writer -n devimprint -o jsonpath='{.data.LITESTREAM_ACCESS_KEY_ID}'
Error from server (Forbidden): secrets "armor-writer" is forbidden:
User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"

# Attempt 2: rs-manager kubeconfig (file doesn't exist)
$ kubectl --kubeconfig=/home/coding/.kube/rs-manager.kubeconfig ...
error: stat /home/coding/.kube/rs-manager.kubeconfig: no such file or directory

# Attempt 3: Check for OpenBao access
$ kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
    get pods -n openbao -l app=openbao
No resources found in openbao namespace
```

### Secret Key Mapping (from code analysis)

Based on `notes/litestream-force-fresh-snapshot-job.yaml`:
- Environment variable `LITESTREAM_ACCESS_KEY_ID` → secret key `auth-access-key`
- Environment variable `LITESTREAM_SECRET_ACCESS_KEY` → secret key `auth-secret-key`

The ExternalSecret config (`~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`) confirms:
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: armor-writer
  namespace: devimprint
spec:
  data:
    - secretKey: auth-access-key
      remoteRef:
        key: rs-manager/ord-devimprint/armor-writer
        property: auth-access-key
    - secretKey: auth-secret-key
      remoteRef:
        key: rs-manager/ord-devimprint/armor-writer
        property: auth-secret-key
```

The source is OpenBao path `rs-manager/ord-devimprint/armor-writer`.

### What Is Needed

To complete either bead (bf-2p1wr OR bf-2xkyl), one of the following must be provided:

1. **A kubeconfig file for ord-devimprint** with secret read permissions, stored at:
   `/home/coding/.kube/ord-devimprint.kubeconfig`

2. **Direct access to the credential values** (can be provided via secure channel, not in chat)

3. **Rackspace Spot portal access** to download the admin kubeconfig and create a ServiceAccount with limited permissions

4. **OpenBao access** on rs-manager to retrieve `rs-manager/ord-devimprint/armor-writer` secret

### Recommendation

The bead dependency chain is broken. The correct order should be:

1. **Complete bf-2p1wr first** - Obtain ord-devimprint kubeconfig with write access
2. **Then complete bf-2xkyl** - Use that kubeconfig to retrieve the credentials

Bead bf-2p1wr should be **re-opened** and completed properly before bf-2xkyl can proceed.

### Acceptance Criteria Status

- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded) - **BLOCKED**
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded) - **BLOCKED**
- [ ] Credentials are stored temporarily in a secure location (not in git history) - **BLOCKED**

**All criteria blocked by missing kubeconfig from prerequisite bead.**
