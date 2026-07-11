# bf-2xkyl: BLOCKER - Missing Kubeconfig Access

## Task Status: BLOCKED

Task: Retrieve S3 credentials from armor-writer secret in devimprint namespace

## Blocker Details

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
   - Returns: "User 'system:serviceaccount:devpod-observer:devpod-observer' cannot get resource 'secrets'"

### Secret Key Mapping

The task requests `LITESTREAM_ACCESS_KEY_ID` and `LITESTREAM_SECRET_ACCESS_KEY`, but investigation of the declarative-config reveals:

**ExternalSecret armor-writer** (`~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml`):
```yaml
secretKey: auth-access-key
  remoteRef:
    key: rs-manager/ord-devimprint/armor-writer
    property: auth-access-key
secretKey: auth-secret-key
  remoteRef:
    key: rs-manager/ord-devimprint/armor-writer
    property: auth-secret-key
```

The actual secret keys are:
- `auth-access-key` (not LITESTREAM_ACCESS_KEY_ID)
- `auth-secret-key` (not LITESTREAM_SECRET_ACCESS_KEY)

**Note:** The litestream-restore-verification-job.yaml references `access-key-id` and `secret-access-key` from armor-writer, but these keys are NOT defined in the ExternalSecret - this appears to be a configuration mismatch.

## Acceptance Criteria - NOT MET

- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)  
- [ ] Credentials are stored temporarily in a secure location

## What is Required to Complete

To complete this task, ONE of the following must be provided:

### Option 1: ord-devimprint Kubeconfig
```bash
# Location: ~/.kube/ord-devimprint.kubeconfig
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.auth-access-key}' | base64 -d
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint -o jsonpath='{.data.auth-secret-key}' | base64 -d
```

### Option 2: rs-manager Kubeconfig  
Access OpenBao at `rs-manager/ord-devimprint/armor-writer` path

### Option 3: Direct Credentials
Provide the actual credential values:
- auth-access-key value
- auth-secret-key value

## Verification Commands Blocked

```bash
# These commands fail due to missing kubeconfig
kubectl --kubeconfig=~/.kube/ord-devimprint.kubeconfig get secret armor-writer -n devimprint
# Error: kubeconfig file does not exist

kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint  
# Error: User cannot get resource 'secrets' (Forbidden)
```

## Recommendation

**DO NOT CLOSE this bead.** The task cannot be completed without proper cluster access. 

The prerequisite bead (bf-2p1wr) needs to be revisited to actually obtain the ord-devimprint kubeconfig, or an alternative access method must be provided.

## References

- Prerequisite bead: bf-2p1wr (marked closed but incomplete)
- ExternalSecret config: ~/declarative-config/k8s/ord-devimprint/devimprint/devimprint-externalsecrets.yml
- Cluster documentation: CLAUDE.md (Kubernetes Access section)
- Prior blocker documentation: notes/bf-2xkyl.md
