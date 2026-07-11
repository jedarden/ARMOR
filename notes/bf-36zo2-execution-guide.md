# Execution Guide: Force Fresh Litestream Backup Baseline (bf-36zo2)

## Overview

This guide provides step-by-step instructions to force queue-api's litestream to establish a fresh backup baseline after ARMOR has been upgraded to the fixed version (0.1.467).

## Prerequisites

- ARMOR is running version 0.1.467 or higher (✅ VERIFIED: currently 0.1.467)
- You have kubectl access to ord-devimprint cluster with write permissions
- Queue-api is currently running and healthy

## Quick Reference

```bash
# 1. Check current state
kubectl get deployment queue-api -n devimprint
kubectl get pods -n devimprint -l app=queue-api

# 2. Scale down queue-api
kubectl scale deployment queue-api --replicas=0 -n devimprint
kubectl wait --for=delete pod -l app=queue-api -n devimprint --timeout=120s

# 3. Apply and run the job
kubectl apply -f /home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml

# 4. Monitor job execution
kubectl logs job/litestream-force-fresh-snapshot -n devimprint -f
kubectl wait --for=condition=complete job/litestream-force-fresh-snapshot -n devimprint --timeout=300s

# 5. Check job results
kubectl logs job/litestream-force-fresh-snapshot -n devimprint
kubectl delete job litestream-force-fresh-snapshot -n devimprint

# 6. Scale up queue-api
kubectl scale deployment queue-api --replicas=1 -n devimprint
kubectl wait --for=condition=available deployment/queue-api -n devimprint --timeout=180s

# 7. Verify fresh snapshot creation
kubectl logs deployment/queue-api -c litestream -n devimprint --tail=100 | grep -i "snapshot\|generation"
```

## Detailed Steps

### Step 1: Verify Pre-Execution State

```bash
# Check ARMOR version
kubectl get deployment armor -n devimprint -o jsonpath='{.spec.template.spec.containers[0].image}'
# Expected: ronaldraygun/armor:0.1.467 or higher

# Check queue-api status
kubectl get pods -n devimprint -l app=queue-api
# Expected: 1 running pod with 2/2 containers ready

# Check litestream is running
kubectl logs deployment/queue-api -c litestream -n devimprint --tail=20
# Should show replication activity
```

### Step 2: Scale Down queue-api

**IMPORTANT**: This stops the queue-api service briefly (~1-2 minutes).

```bash
# Scale down to zero replicas
kubectl scale deployment queue-api --replicas=0 -n devimprint

# Wait for pod termination
kubectl wait --for=delete pod -l app=queue-api -n devimprint --timeout=120s

# Verify no pods running
kubectl get pods -n devimprint -l app=queue-api
# Expected: No resources found
```

**Impact**: Queue-api will be unavailable during this period. The seeder/workers will retry and queue naturally repopulates on restart.

### Step 3: Apply the Fresh Snapshot Job

```bash
# Apply the job YAML
kubectl apply -f /home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml

# Verify job was created
kubectl get job litestream-force-fresh-snapshot -n devimprint
```

### Step 4: Monitor Job Execution

```bash
# Watch job logs in real-time
kubectl logs job/litestream-force-fresh-snapshot -n devimprint -f

# Or wait for completion (background)
kubectl wait --for=condition=complete job/litestream-force-fresh-snapshot -n devimprint --timeout=300s

# Check final job status
kubectl describe job litestream-force-fresh-snapshot -n devimprint
```

**Expected Job Output**:
```
=== Litestream Fresh Snapshot Job ===
Task: bf-36zo2
Timestamp: [timestamp]

Step 1: Checking database exists
✓ Database found: /data/queue.db

Step 2: Displaying litestream configuration
[config content]

Step 3: Checking current litestream status
[database status or "no databases found"]

Step 4: Clearing litestream local state
This forces a fresh snapshot on next sync
[reset output]

Step 5: Verifying reset
✓ Litestream state successfully cleared

=== Job Complete ===
Next steps:
1. Scale queue-api deployment back to 1
2. Monitor litestream logs for new snapshot creation
3. Verify new generation in ARMOR S3 bucket
```

### Step 5: Cleanup Job

```bash
# Get final job logs
kubectl logs job/litestream-force-fresh-snapshot -n devimprint > /tmp/litestream-reset.log

# Delete the job
kubectl delete job litestream-force-fresh-snapshot -n devimprint
```

### Step 6: Scale Up queue-api

```bash
# Scale back to 1 replica
kubectl scale deployment queue-api --replicas=1 -n devimprint

# Wait for pod to be ready
kubectl wait --for=condition=available deployment/queue-api -n devimprint --timeout=180s

# Verify pod is healthy
kubectl get pods -n devimprint -l app=queue-api
# Expected: 1 running pod with 2/2 containers ready
```

### Step 7: Verify Fresh Snapshot Creation

**This is the critical verification step.**

```bash
# Monitor litestream logs for snapshot creation
kubectl logs deployment/queue-api -c litestream -n devimprint -f --tail=100 | grep -i "snapshot\|generation"

# Look for messages like:
# - "taking new snapshot"
# - "generating new snapshot"
# - "snapshot uploaded"
# - generation IDs
```

**Expected Behavior**:
1. Litestream sidecar starts
2. Detects no local state (cleared by reset job)
3. Initiates fresh snapshot upload to ARMOR
4. Logs show snapshot creation progress
5. Continues normal WAL replication

### Step 8: Note Generation ID

```bash
# Extract generation ID from litestream logs
kubectl logs deployment/queue-api -c litestream -n devimprint --tail=200 | grep -i "generation" > /tmp/litestream-generation.log

# Or check litestream databases status (if you can exec)
kubectl exec -n devimprint deployment/queue-api -c litestream -- litestream databases
```

**Save the generation ID** for the next verification step (restore testing).

## Troubleshooting

### Job Fails to Start

```bash
# Check job status
kubectl describe job litestream-force-fresh-snapshot -n devimprint

# Common issues:
# - PVC not mounted (check volume claim name)
# - ConfigMap not found (check litestream-config)
# - Secrets not accessible (check armor-writer secret)
```

### Job Times Out

```bash
# Check job pod logs
kubectl logs -l app=litestream-reset -n devimprint

# Increase timeout if needed
# Edit activeDeadlineSeconds in job YAML
```

### No Fresh Snapshot Created

```bash
# Verify litestream configuration
kubectl exec -n devimprint deployment/queue-api -c litestream -- cat /etc/litestream.yml

# Check litestream can reach ARMOR
kubectl exec -n devimprint deployment/queue-api -c litestream -- litestream replicate -v

# Manually trigger snapshot (if needed)
kubectl exec -n devimprint deployment/queue-api -c litestream -- litestream snapshot /data/queue.db
```

### Queue-api Unhealthy After Scale-Up

```bash
# Check pod status
kubectl get pods -n devimprint -l app=queue-api

# Check logs
kubectl logs deployment/queue-api -c queue-api -n devimprint --tail=100

# Common issues:
# - Database locked (litestream still initializing)
# - PVC mount issues
# - Resource constraints
```

## Success Criteria

You have successfully completed this task when:

- [x] Confirmed litestream is configured for queue-api database
- [x] Created Kubernetes Job to trigger fresh snapshot/generation
- [ ] Verified new backup generation is active and replicating
- [ ] Noted the generation ID for the next verification step

## Next Steps (Separate Tasks)

1. **bf-4qq1**: Complete ARMOR upgrade verification (parent task)
2. **Restore Testing**: Test restore from the fresh snapshot to verify backup integrity
3. **Cleanup**: Remove old backup generations (after verification succeeds)
4. **Documentation**: Update runbooks with new backup generation ID

## Files Created

- `/home/coding/ARMOR/notes/bf-36zo2-litestream-fresh-snapshot.md` - Task documentation
- `/home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml` - Kubernetes Job YAML
- `/home/coding/ARMOR/notes/bf-36zo2-execution-guide.md` - This execution guide

## Rollback Plan

If something goes wrong:

1. **Job fails**: Delete job, investigate logs, retry with fixes
2. **Queue-api won't start**: Check PVC mount, database integrity, litestream restore logs
3. **Litestream won't replicate**: Check ARMOR connectivity, credentials, configuration
4. **Critical failure**: Can rollback to previous ARMOR version (but loses data after old backup)

## Estimated Timeline

- Scale down queue-api: 30 seconds
- Job execution: 1-2 minutes
- Scale up queue-api: 1-2 minutes
- Fresh snapshot creation: 5-15 minutes (depends on DB size)
- **Total downtime**: ~2-3 minutes
- **Total time**: ~20-30 minutes

## References

- [Litestream Reset Command](https://litestream.io/reference/reset/)
- [Litestream Kubernetes Guide](https://litestream.io/guides/kubernetes/)
- [Queue API Deployment](https://github.com/jedarden/declarative-config)
- [ARMOR Multipart Corruption Fix](https://github.com/jedarden/ARMOR/commit/b96d7eb)
