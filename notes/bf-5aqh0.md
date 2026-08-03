# bf-5aqh0: Queue-API Restore Test - OBSOLETE

**Status:** NOT COMPLETED - Task premise obsolete

## Why This Task Cannot Be Completed

This task was written to test-restore queue-api litestream backups from `devimprint` bucket via ARMOR (`http://armor:9000`). However, the infrastructure changed and the backup target no longer exists at that location.

## Verified Facts (as of 2026-08-03)

### 1. Queue-API Migrated Namespaces
- **Original:** Assumed to be in `devimprint` namespace
- **Current:** `commitgraph` namespace on `ord-devimprint` cluster
- **Verified:** `kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n commitgraph` shows `queue-api-c5894c469-p9rhr`

### 2. Litestream Target Changed Completely
- **Original (from bead):** `devimprint` bucket, path `state/litestream/queue.db`, endpoint `http://armor:9000`
- **Current:** `commitgraph-ops` bucket, path `queue-api/queue.db`, endpoint `https://s3.us-west-002.backblazeb2.com`
- **Verified:** ConfigMap `queue-api-litestream-config` in `commitgraph` namespace shows:
  ```yaml
  bucket: commitgraph-ops
  path: queue-api/queue.db
  endpoint: https://s3.us-west-002.backblazeb2.com
  ```
- **Reason for change (from config comment):** The move to `commitgraph-ops` was intentional (bf-2z1jn) because queue.db contains PII-adjacent data (author emails), and `commitgraph-ops` is a private bucket, unlike the public corpus bucket.

### 3. Original Target Bucket is Empty
- **Bucket:** `devimprint` (the bead's target)
- **Verified:** The `restore-verifier` deployment in `devimprint` namespace checks this bucket every 6 hours
- **Logs:** Consistently show "no objects found" since at least 2026-07-31:
  ```
  2026/08/03 01:34:53 Verifying bucket: devimprint
  2026/08/03 01:34:53 Failed to get latest object for bucket devimprint: no objects found
  ```

### 4. Access Constraints
- **Read-only proxy:** The `devpod-observer` kubectl-proxy used to access `ord-devimprint` has no access to secrets (cannot get B2 credentials)
- **No create/exec permissions:** Cannot create restore pods or exec into existing ones from read-only proxy
- **New bucket location:** The `commitgraph-ops` bucket is on B2 directly, not accessible through ARMOR endpoint

## Conclusion

The acceptance criteria cannot be met because:
1. The backup that this task was written to verify does not exist at the specified location (`devimprint` bucket)
2. queue-api now backs up to `commitgraph-ops` bucket directly via B2
3. Without B2 credentials and write access to the cluster, we cannot perform a restore test

## Alternatives

The continuous `restore-verifier` deployment (which checks buckets every 6 hours) may have superseded this one-off drill. However, it only monitors `devimprint` bucket, not the new `commitgraph-ops` target.

To properly verify queue-api backups going forward, an operator would need to:
- Re-target the restore test to `commitgraph-ops` bucket
- Extend the `restore-verifier` to cover `commitgraph-ops` in addition to `devimprint`
- Close this bead as obsolete (requires operator decision - acceptance criteria cannot be met)

## Related Beads

- `bf-34xw9` - Litestream restore (also credential-gated, obsolete premise)
- `bf-54irmj` - Lifecycle test (delivered, but closure credential-gated)
- Memory: [[bf-5aqh0-queue-api-restore-obsolete]]
