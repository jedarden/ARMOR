# bf-36zo2 Execution Summary: Litestream Fresh Snapshot

## Date: 2026-07-11

## Task Objective
Force fresh litestream backup baseline for queue-api after ARMOR multipart corruption fix.

## Current State Assessment

### ✅ Confirmed Working
1. **Litestream is configured and active** for queue-api
   - Database: `/data/queue.db`
   - PVC: `queue-api-data-sata-2`
   - Replication target: ARMOR S3 at `http://armor:9000`
   - Current status: Actively replicating (TXID: 000000000005ffa7)

2. **queue-api deployment is healthy**
   - 1/1 replicas running
   - Pod: `queue-api-7999dffbd7-l8hgr` (2/2 containers ready)
   - Litestream sidecar: Running and replicating

### ❌ Blocking Issue: No Write Access to ord-devimprint

#### Discovery
The ord-devimprint cluster is **only accessible via read-only proxy**:
- Endpoint: `http://kubectl-proxy-ord-devimprint:8001`
- ServiceAccount: `devpod-observer` (read-only RBAC)
- **Cannot create, delete, or modify resources**

#### Evidence
```bash
$ kubectl --server=http://kubectl-proxy-ord-devimprint:8001 scale deployment queue-api --replicas=0 -n devimprint
Error from server (Forbidden): deployments.apps "queue-api" is forbidden: User "system:serviceaccount:devpod-observer:devpod-observer" cannot patch resource "deployments/scale" in API group "apps" in the namespace "devimprint"
```

#### Investigation Results
- Checked for direct kubeconfig: No `ord-devimprint.kubeconfig` exists
- Checked ArgoCD: No applications found for ord-devimprint
- Checked declarative-config: No GitOps mechanism identified for this cluster
- Available kubeconfigs: Only `iad-acb.kubeconfig` and `iad-ci.kubeconfig`

## What Was Prepared

### 1. Complete Execution Guide
- **File**: `notes/bf-36zo2-execution-guide.md`
- **Status**: Complete and ready to execute
- **Steps**: 10-step procedure with verification and rollback

### 2. Kubernetes Job YAML
- **File**: `notes/litestream-force-fresh-snapshot-job.yaml`
- **Status**: Ready to apply
- **Function**: Clears litestream state to force fresh snapshot creation

### 3. Detailed Documentation
- **File**: `notes/bf-36zo2-litestream-fresh-snapshot-guide.md`
- **Status**: Complete with prerequisites and verification steps

## Required Actions to Complete This Task

### Option 1: Create Write Access for ord-devimprint
1. Create a kubeconfig with cluster-admin or deployment-edit access for ord-devimprint
2. Store at: `~/.kube/ord-devimprint.kubeconfig`
3. Update CLAUDE.md with access pattern
4. Execute the prepared job using the new kubeconfig

### Option 2: Establish GitOps for ord-devimprint
1. Add ord-devimprint applications to ArgoCD on ardenone-manager
2. Create the job in declarative-config repository
3. Sync through ArgoCD instead of direct kubectl
4. Monitor job execution through ArgoCD UI

### Option 3: Manual Execution by Cluster Administrator
1. Provide the prepared job YAML to cluster administrator
2. Request manual execution of the job
3. Verify completion and document generation ID

## Why This Task Is Critical

From ADR-002:
> `ord-devimprint`'s ARMOR deployment was on `0.1.19` (before the multipart fix) and queue-api's litestream backup couldn't be restored: the backup's only surviving snapshot didn't decode, and two more objects were corrupted live *during the investigation*. **queue-api's B2 backup chain had no valid restore point for a ~40-day window.**

The fresh snapshot is essential to:
1. Establish a clean baseline from the fixed ARMOR version
2. Ensure backup chain is free of multipart corruption
3. Enable reliable restore capability going forward

## Next Steps

1. **Resolve access limitation** - Choose one of the options above
2. **Execute the prepared job** - Use the execution guide
3. **Verify new snapshot** - Confirm generation ID and replication
4. **Update runbooks** - Document the new generation ID

## Files Created for This Task

- `notes/bf-36zo2-execution-guide.md` - Complete execution procedure
- `notes/bf-36zo2-litestream-fresh-snapshot-guide.md` - Prerequisites and verification
- `notes/litestream-force-fresh-snapshot-job.yaml` - Kubernetes job YAML
- `notes/bf-36zo2-execution-summary.md` - This summary

## References

- ADR-002: Multipart Corruption Detection Gaps
- Execution Guide: `notes/bf-36zo2-execution-guide.md`
- Job YAML: `notes/litestream-force-fresh-snapshot-job.yaml`
