# Litestream Fresh Snapshot for queue-api (bf-36zo2)

## Task Summary
Force a fresh litestream backup baseline for queue-api after ARMOR multipart corruption fix. This is critical because existing backups may contain the multipart corruption bug.

## Prerequisites Verified ✅

1. **Litestream Configuration**: Confirmed for queue-api database
   - Database path: `/data/queue.db`
   - PVC: `queue-api-data-sata-2`
   - ConfigMap: `queue-api-litestream-config`
   - Replication target: ARMOR S3 (`http://armor:9000`) → `devimprint/state/litestream/queue.db`

2. **Current State**: queue-api deployment running 1 replica
   - Litestream sidecar: active and replicating
   - Init container: `litestream-restore` (handles recovery on startup)

## Execution Steps

These steps require write access to ord-devimprint cluster. Execute in order:

### Step 1: Scale down queue-api
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 scale deployment queue-api --replicas=0 -n devimprint
```

### Step 2: Wait for pod termination
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 wait --for=delete pod -l app=queue-api -n devimprint --timeout=60s
```

### Step 3: Apply the reset job
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 apply -f /home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml
```

### Step 4: Monitor job completion
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 wait --for=condition=complete job/litestream-force-fresh-snapshot -n devimprint --timeout=300s
```

### Step 5: Check job logs
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 logs job/litestream-force-fresh-snapshot -n devimprint
```

Expected output includes:
- `✓ Database found: /data/queue.db`
- `Configuration: [...]` (showing litestream config)
- `✓ Litestream state successfully cleared` or similar

### Step 6: Scale up queue-api
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 scale deployment queue-api --replicas=1 -n devimprint
```

### Step 7: Wait for pod ready
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 wait --for=condition=ready pod -l app=queue-api -n devimprint --timeout=120s
```

### Step 8: Verify fresh snapshot creation
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 logs deployment/queue-api -c litestream -n devimprint --tail=50
```

**Look for these log messages**:
- `taking new snapshot`
- `generating new snapshot`
- `generation` followed by a new generation ID (e.g., `0000000000000002` or higher)

### Step 9: Note the generation ID
Record the generation ID from the logs for the next verification step (bf-5uehq). This is needed to verify the backup is valid and can be restored.

### Step 10: Cleanup (optional)
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 delete job litestream-force-fresh-snapshot -n devimprint
```

## Verification Checklist

- [ ] Job completed successfully (exit code 0)
- [ ] queue-api scaled back up and healthy
- [ ] Litestream logs show "taking new snapshot" or similar
- [ ] New generation ID noted for downstream verification
- [ ] Job cleanup completed (optional)

## What the Reset Does

The `litestream reset /data/queue.db` command:
1. Clears litestream's local tracking database for `/data/queue.db`
2. On next startup, litestream sees no prior state
3. Triggers creation of a new snapshot generation
4. New snapshot is created from the fixed ARMOR version

This ensures the backup chain starts from a clean baseline, free of any multipart corruption that may have occurred with the old ARMOR version.

## Next Steps

After completion:
1. Use the noted generation ID for backup restore verification (see bf-5uehq)
2. Monitor litestream logs for ongoing replication
3. Verify no corruption errors in ARMOR logs for the new objects

## References

- Job file: `/home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml`
- ADR on multipart corruption: `/home/coding/ARMOR/docs/adr/002-multipart-corruption-detection-gaps.md`
- Bead: bf-36zo2
