# Queue-API Backup Restore Verification Plan (bf-5aqh0)

## Task Summary

Actually test-restore the fresh litestream backup into a scratch location before trusting it. This is the critical verification step - do NOT assume the version bump alone fixes already-written data.

## Current Situation

**Constraint Identified**: The `kubectl-proxy-ord-devimprint:8001` endpoint is read-only. We cannot create/apply the verification job directly through this proxy.

**Current Backup State**:
- Litestream is actively replicating `/data/queue.db` to ARMOR
- Storage path: `devimprint/state/litestream/queue.db`
- Endpoint: `http://armor:9000`
- Replication is active (confirmed from logs showing continuous WAL uploads)

## Verification Strategy

Since we don't have direct write access through the proxy, we have several options:

### Option 1: Use ArgoCD Application (Recommended)

The queue-api deployment is managed by ArgoCD. We can add the verification job to the declarative-config repo and let ArgoCD deploy it.

**Steps**:
1. Add the job YAML to `jedarden/declarative-config/k8s/ord-devimprint/devimprint/`
2. ArgoCD will automatically sync the job
3. The job will run once and complete
4. We can check logs via the read-only proxy

### Option 2: Request Write Access

Request write access to ord-devimprint cluster for this specific verification task.

### Option 3: Manual Documentation (Current Approach)

Document the complete restore procedure and provide the job YAML for future execution.

## Created Verification Job

The job file `/home/coding/ARMOR/notes/litestream-restore-verification-job.yaml` has been created and will:

### Verification Steps (Built into Job)

1. **Check Original Database**: Verify `/data/queue.db` exists and note its size
2. **Display Configuration**: Show current litestream configuration
3. **Check Status**: Query litestream for database status
4. **Create Restore Location**: Create `/data/restore_test/` temporary directory
5. **Execute Restore**: Run `litestream restore -o /data/restore_test/queue_restored.db /data/queue.db`
6. **Verify File Created**: Check that restored file exists
7. **Integrity Check**: Run SQLite `PRAGMA integrity_check`
8. **Verify Data Content**: Ensure tables exist and data is accessible
9. **Compare Sizes**: Compare original vs restored file sizes
10. **Schema Overview**: Display database schema to verify structure

### Success Criteria

The verification job succeeds when:
- ✅ Restore command completes without errors
- ✅ Restored database file is created with reasonable size
- ✅ SQLite integrity check passes (no corruption)
- ✅ Database contains tables and data
- ✅ File sizes are comparable within expected tolerance

## Manual Execution Instructions

### Via ArgoCD (Preferred)

1. Add the job to your declarative-config repository:
```bash
cd ~/declarative-config
cp /home/coding/ARMOR/notes/litestream-restore-verification-job.yaml \
   k8s/ord-devimprint/devimprint/litestream-restore-verification-job.yaml
git add k8s/ord-devimprint/devimprint/litestream-restore-verification-job.yaml
git commit -m "Add litestream restore verification job (bf-5aqh0)"
git push
```

2. Wait for ArgoCD to sync (usually within 1-2 minutes)

3. Monitor the job:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  get job litestream-restore-verification -n devimprint -w
```

4. Check logs:
```bash
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  logs job/litestream-restore-verification -n devimprint
```

5. Cleanup after successful run:
```bash
# Add to declarative-config
# k8s/ord-devimprint/devimprint/litestream-restore-verification-cleanup-job.yaml
kubectl --server=http://kubectl-proxy-ord-devimprint:8001 \
  delete job litestream-restore-verification -n devimprint
rm -rf /path/to/pvc/restore_test  # If needed
```

### Via Direct Cluster Access

If you have direct kubeconfig access:

```bash
# Apply the job
kubectl apply -f /home/coding/ARMOR/notes/litestream-restore-verification-job.yaml

# Monitor execution
kubectl wait --for=condition=complete job/litestream-restore-verification -n devimprint --timeout=600s

# Check logs
kubectl logs job/litestream-restore-verification -n devimprint

# Cleanup
kubectl delete job litestream-restore-verification -n devimprint
```

## What This Verification Proves

1. **Backup Chain Integrity**: Confirms the litestream backup generation is complete and readable
2. **ARMOR Backend Health**: Verifies ARMOR can serve the backup data correctly
3. **Restore Procedure**: Tests the actual restore command that would be used in disaster recovery
4. **Data Integrity**: SQLite integrity check ensures no corruption in the backup
5. **No Data Loss**: Verifying record counts and schema confirms no data was lost

## Expected Results

### Successful Run Output (Expected):

```
=== Litestream Restore Verification Job ===
Task: bf-5aqh0
Timestamp: 2026-07-11T...

Step 1: Checking original database exists
✓ Original database found: /data/queue.db
  Size: 98304 bytes

Step 2: Displaying litestream configuration
Configuration:
[dbs configuration shown]

Step 3: Checking litestream databases status
[database info]

Step 4: Creating temporary restore location
✓ Created temporary directory: /data/restore_test

Step 5: Running restore to temporary location
✓ Restore command completed successfully

Step 6: Verifying restored database
✓ Restored database created
  Size: 98304 bytes

Step 7: Checking database integrity
Running integrity check...
ok|✓ Database integrity check passed

Step 8: Verifying database has data
  Found X tables
✓ Database contains tables

Step 9: Showing schema overview
[SQL schema]

Step 10: Comparing file sizes
  Original size: 98304 bytes
  Restored size: 98304 bytes
  Difference: 0 bytes (0%)
✓ File sizes are comparable

Step 11: Cleanup (keeping restored file for manual inspection)
  Restored database available at: /data/restore_test/queue_restored.db

=== Verification Complete ===
✓ All checks passed - restore test successful
```

## Follow-up Actions

### After Successful Verification

1. **Document the generation ID**: Note which backup generation was verified
2. **Update runbooks**: Add the verified generation ID to disaster recovery docs
3. **Schedule regular tests**: Add quarterly restore tests to maintenance schedule
4. **Monitor for issues**: Watch for any ARMOR errors in the restored backup

### If Verification Fails

1. **Check ARMOR logs**: Look for errors in armor pod logs
2. **Verify litestream config**: Ensure configuration is correct
3. **Check credentials**: Verify armor-writer secret is valid
4. **Test ARMOR health**: Run ARMOR health checks
5. **Investigate data**: Check if the original database has issues

## Disaster Recovery Implications

Once this verification passes, we have confirmed that:

1. **The backup is reliable**: In a disaster scenario, we can restore from this backup generation
2. **The procedure works**: The litestream restore command functions correctly with our ARMOR backend
3. **No corruption exists**: The multipart corruption fix is working for new backups
4. **We can recover**: The complete disaster recovery procedure is viable

## Files Created

- `/home/coding/ARMOR/notes/litestream-restore-verification-job.yaml` - Kubernetes Job for restore testing
- `/home/coding/ARMOR/notes/bf-5aqh0-restore-verification-plan.md` - This documentation

## Next Steps

1. ✅ Created verification job YAML
2. ✅ Documented complete restore procedure
3. ⏳ Execute verification job (requires write access or ArgoCD integration)
4. ⏳ Document results after execution
5. ⏳ Update disaster recovery runbook with verified generation ID

## References

- Litestream Restore Documentation: https://litestream.io/reference/restore/
- Original Fresh Snapshot Task: bf-36zo2
- ARMOR Disaster Recovery Guide: /home/coding/ARMOR/docs/disaster-recovery.md
- Queue-API Litestream Configuration: queue-api-litestream-config ConfigMap