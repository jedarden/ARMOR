# Task: Retrieve S3 credentials from armor-writer secret

## BLOCKED - Prerequisite Not Actually Met

### Current Situation

The prerequisite bead (bf-2p1wr) is marked as "closed" but the acceptance criteria were NOT fulfilled:

**Missing kubeconfig:**
```bash
$ ls -la ~/.kube/ord-devimprint.kubeconfig
ls: cannot access '/home/coding/.kube/ord-devimprint.kubeconfig': No such file or directory
```

**Access attempts:**
- Read-only proxy (`kubectl-proxy-ord-devimprint:8001`) - Cannot GET secrets (Forbidden)
- No rs-manager kubeconfig available to check synced ExternalSecret
- No other kubeconfig provides ord-devimprint cluster access

### What I Checked

1. **ord-devimprint read-only proxy** - Cannot access secrets:
   ```bash
   kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get secret armor-writer -n devimprint
   # Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

2. **rs-manager ExternalSecret** - Found cluster secret but cannot read:
   ```bash
   kubectl --server=http://traefik-rs-manager:8001 get secrets -n argocd | grep ord
   # cluster-ord-devimprint   Opaque   3   79d
   
   kubectl --server=http://traefik-rs-manager:8001 get secret cluster-ord-devimprint -n argocd
   # Error: User "system:serviceaccount:devpod-observer:devpod-observer" cannot get resource "secrets"
   ```

3. **Available kubeconfigs** - Only iad-ci and iad-acb exist (no ord-devimprint access):
   ```bash
   ~/.kube/iad-ci.kubeconfig      - iad-ci cluster only
   ~/.kube/iad-acb.kubeconfig     - iad-acb cluster only
   ```

### What's Needed

To complete this task, ONE of the following is required:

**Option 1: Direct ord-devimprint kubeconfig**
- File: `~/.kube/ord-devimprint.kubeconfig` with secret-read permissions
- Source: Rackspace Spot portal (see bf-2p1wr notes for acquisition steps)

**Option 2: rs-manager kubeconfig** 
- File: `~/.kube/rs-manager.kubeconfig` with secret-read permissions
- Can then check if ExternalSecret synced useful credentials

**Option 3: Secret provided directly**
- LITESTREAM_ACCESS_KEY_ID value
- LITESTREAM_SECRET_ACCESS_KEY value

### Acceptance Criteria - NOT MET

- [ ] Successfully retrieved LITESTREAM_ACCESS_KEY_ID value (base64-decoded)
- [ ] Successfully retrieved LITESTREAM_SECRET_ACCESS_KEY value (base64-decoded)
- [ ] Credentials stored temporarily in secure location

### Recommendation

**Do NOT close this bead.** The prerequisite was incorrectly marked as complete. This task requires actual ord-devimprint cluster access which does not exist.

Next steps:
1. Reopen bead bf-2p1wr to complete the kubeconfig acquisition, OR
2. Manually obtain and provide the ord-devimprint kubeconfig, OR  
3. Provide the S3 credentials directly

## References

- Prerequisite bead notes: `/home/coding/ARMOR/notes/bf-2p1wr.md`
- ExternalSecret config: `~/declarative-config/k8s/rs-manager/argocd/ord-devimprint-cluster-externalsecret.yml`
- Cluster docs: CLAUDE.md (Kubernetes Access section)
