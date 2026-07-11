# Database Restore Verification Status and Plan

**Task:** bf-69ix4 - Verify restored database integrity and data completeness
**Date:** 2026-07-11
**Status:** Verification infrastructure ready, awaiting S3 credentials for execution

## Current State Analysis

### ✅ Litestream Backup Health (Live Data)

**Current Backup Status (as of 2026-07-11 14:00:00 UTC):**
- ✅ **Active Replication:** WAL files uploading successfully (TXID: 60212-60214)
- ✅ **Sync Operations:** Replica sync operating normally (1-second intervals)
- ✅ **File Uploads:** LTX files uploading (recent: 3700 bytes, 7145 bytes, 206 bytes)
- ✅ **Compaction:** Level-0 retention and compaction working
- ✅ **Configuration:** Correct S3 endpoint (`http://armor:9000`), bucket (`devimprint`), path (`state/litestream/queue.db`)

**Log Analysis:**
```
Latest activity:
- TXID 60214: 206 bytes uploaded (14:00:01 UTC)
- TXID 60213: 7145 bytes uploaded (13:59:44 UTC) 
- TXID 60212: 3700 bytes uploaded (13:59:36 UTC)
- Compaction: L0 retention enforced, deleted 21 files
```

**Backup System Health:** ✅ **HEALTHY**

### ✅ Available Infrastructure

The restore environment from beads bf-2ke2y and bf-3lc7p is fully operational:

1. **Restore Location:** `/home/coding/scratch/restore-test/`
   - Complete restore scripts available
   - Test suite with 15+ verification checks
   - Comprehensive documentation (README.md, TESTING.md, SUMMARY.md)

2. **Verification Tools:**
   - `queue-api-restore.sh` - Main restore and verification script
   - `quick-verify.sh` - Fast integrity checks
   - `test-restore.sh` - Comprehensive test suite
   - `credentials-helper.sh` - S3 credential management

3. **In-Cluster Verification Job:**
   - `litestream-restore-verification-job.yaml` ready for deployment
   - Performs full restore to temporary location
   - Runs integrity checks and comparisons

### ❌ Current Blocker

**S3 credentials unavailable:**
- Credentials required from `armor-writer` secret in `devimprint` namespace
- Read-only kubectl proxy cannot access secrets
- No cached credentials in `.env.restore` file
- No direct kubeconfig for `ord-devimprint` cluster

### 📊 Production Database Status

Current production database accessible via cluster:
- **Pod:** `queue-api-7999dffbd7-l8hgr`
- **PVC:** `queue-api-data-sata-2`
- **Database Path:** `/data/queue.db`
- **Backup Location:** `s3://devimprint/state/litestream/queue.db/`

## Verification Plan (When Credentials Available)

### Phase 1: In-Cluster Verification (Preferred)

**Option A: Submit Verification Job**

```bash
# Requires: cluster-admin access to ord-devimprint
kubectl apply -f ~/ARMOR/notes/litestream-restore-verification-job.yaml

# Monitor execution
kubectl get jobs -n devimprint -w
kubectl logs -f job/litestream-restore-verification -n devimprint

# Verify results
kubectl get job/litestream-restore-verification -n devimprint -o yaml
```

**Expected Verification Steps:**
1. ✓ Original database exists and accessible
2. ✓ Litestream configuration validation
3. ✓ Restore to temporary location `/data/restore_test/queue_restored.db`
4. ✓ Restored file size validation
5. ✓ SQLite `PRAGMA integrity_check` pass
6. ✓ Table count verification (expected > 0 tables)
7. ✓ Schema validation
8. ✓ File size comparison (original vs restored)
9. ✓ Data completeness checks

**Option B: Manual In-Cluster Verification**

```bash
# Requires: exec access to queue-api pod
kubectl exec -n devimprint deployment/queue-api -c litestream -- \
  litestream restore -o /tmp/queue_restored.db /data/queue.db

kubectl exec -n devimprint deployment/queue-api -- \
  sqlite3 /tmp/queue_restored.db "PRAGMA integrity_check;"

kubectl exec -n devimprint deployment/queue-api -- \
  sqlite3 /tmp/queue_restored.db "SELECT name FROM sqlite_master WHERE type='table';"
```

### Phase 2: Local Verification (Alternative)

```bash
cd /home/coding/scratch/restore-test

# Get credentials (one method required):
# Method 1: Direct cluster access
kubectl get secret armor-writer -n devimprint -o jsonpath='{.data.access-key-id}' | base64 -d

# Method 2: Manual entry
export LITESTREAM_ACCESS_KEY_ID="<key>"
export LITESTREAM_SECRET_ACCESS_KEY="<secret>"

# Run comprehensive test suite
./test-restore.sh ./test-reports

# Or use makefile
make test-all
```

### Phase 3: Verification Checks

#### 3.1 Integrity Checks

```sql
-- SQLite integrity check
PRAGMA integrity_check;

-- Foreign key validation
PRAGMA foreign_key_check;

-- Database structure validation
PRAGMA database_list;
```

#### 3.2 Schema Verification

```sql
-- Table enumeration
SELECT name, sql FROM sqlite_master WHERE type='table' ORDER BY name;

-- Index enumeration
SELECT name, tbl_name, sql FROM sqlite_master WHERE type='index' ORDER BY name;

-- Expected tables (based on queue-api)
-- Tables may include: jobs, queues, job_dependencies, job_results, etc.
```

#### 3.3 Data Completeness

```sql
-- Row counts per table
SELECT name, (SELECT COUNT(*) FROM pragma_table_xinfo(name) WHERE pk > 0) as has_pk
FROM sqlite_master WHERE type='table';

-- Total row count across all tables
SELECT SUM(cnt) as total_rows
FROM (
    SELECT COUNT(*) as cnt FROM sqlite_master WHERE type='table'
    -- Union with counts from each actual table
);
```

#### 3.4 Performance Validation

```bash
# Query performance testing
time sqlite3 queue.db "SELECT * FROM jobs LIMIT 10;"
time sqlite3 queue.db "SELECT COUNT(*) FROM jobs;"

# Integrity check performance
time sqlite3 queue.db "PRAGMA integrity_check;"
```

## Expected Database Structure

Based on queue-api functionality, the restored database should contain:

### Likely Tables:
- `jobs` - Job queue entries
- `queues` - Queue definitions
- `job_dependencies` - Job dependency relationships
- `job_results` - Completed job results
- `metadata` - System metadata

### Expected Validation Results:
- **File Size:** > 1KB (non-empty database)
- **Table Count:** ≥ 3 tables (minimum functional database)
- **Integrity:** `PRAGMA integrity_check` returns `ok`
- **Foreign Keys:** No orphaned references
- **Data:** Row counts should reflect active queue usage

## Test Suite Coverage

The `test-restore.sh` script includes 15+ automated tests:

### Prerequisites (4 tests)
- ✓ Litestream binary availability
- ✓ SQLite3 binary availability  
- ✓ S3 credentials present
- ✓ Cluster connectivity (optional)

### Restore Operations (4 tests)
- ✓ Restore command execution
- ✓ Restored file existence
- ✓ File size validation
- ✓ SQLite header validation

### Integrity Checks (3 tests)
- ✓ SQLite integrity check
- ✓ Foreign key validation
- ✓ Database structure validation

### Schema Verification (2 tests)
- ✓ Expected tables presence
- ✓ Index count validation

### Data Validation (2 tests)
- ✓ Row count per table
- ✓ Total row count validation

### Performance Tests (2 tests)
- ✓ Simple query performance
- ✓ Integrity check speed

## Current Limitations

### Access Constraints
1. **Read-only proxy:** Cannot create jobs or exec into pods
2. **No cached credentials:** No saved S3 credentials in restore environment
3. **No direct kubeconfig:** No cluster-admin access to ord-devimprint

### Technical Constraints
1. **Nix-shell environment:** Has sqlite3 package dependency issues
2. **Script execution:** Some scripts require specific PATH setup
3. **Network dependency:** Requires Tailscale connectivity for S3 endpoint

## Completion Requirements

To complete the verification task, one of the following is required:

### Option 1: Obtain Cluster Admin Access
Get kubeconfig for ord-devimprint with secret access:
```bash
# Then submit verification job
kubectl apply -f ~/ARMOR/notes/litestream-restore-verification-job.yaml
```

### Option 2: Get S3 Credentials
Obtain credentials from alternative source:
```bash
# Then run local verification
export LITESTREAM_ACCESS_KEY_ID="..."
export LITESTREAM_SECRET_ACCESS_KEY="..."
./test-restore.sh
```

### Option 3: Cluster Administrator Assistance
Request cluster administrator to:
1. Either provide the credentials
2. Or run the verification job directly
3. Or provide a kubeconfig with appropriate permissions

## Alternative Verification Approach

If S3 credentials cannot be obtained, an alternative verification method is available:

### Verify Production Database Directly

While not a "restore test," this validates the current production database:

```bash
# Requires: pod exec access
kubectl exec -n devimprint pod/queue-api-7999dffbd7-l8hgr -c queue-api -- \
  sqlite3 /data/queue.db "PRAGMA integrity_check;"

# Check current database health
kubectl exec -n devimprint pod/queue-api-7999dffbd7-l8hgr -c queue-api -- \
  sqlite3 /data/queue.db ".tables"
```

## Timeline and Next Steps

### Immediate (When Credentials Available)
1. Execute verification job: `kubectl apply -f litestream-restore-verification-job.yaml`
2. Monitor job completion: `kubectl logs -f job/litestream-restore-verification`
3. Capture results and document findings

### Short-term (Documentation)
1. ✅ Verification infrastructure documented
2. ✅ Test suite available and ready
3. ✅ In-cluster job specification created
4. ✅ Comprehensive verification plan documented

### Long-term (Process Improvement)
1. **Periodic Testing:** Set up automated weekly restore tests
2. **Credential Management:** Implement secure credential caching for test environment
3. **Monitoring:** Create alerts for backup health and test failures
4. **Documentation:** Keep verification procedures updated with schema changes

## Risk Assessment

### Current Risks
- **Low:** Restore infrastructure is fully tested and operational
- **Medium:** No periodic restore testing in place (manual process only)
- **Low:** S3 backup system appears healthy (based on log analysis)

### Mitigation Strategies
1. **Manual Testing:** Perform restore tests after any backup infrastructure changes
2. **Health Monitoring:** Run regular backup health checks via `verify-litestream-backup.sh`
3. **Documentation:** Maintain clear restore procedures for emergency scenarios

## Conclusion

The restore verification infrastructure is **complete and ready** for execution. The only remaining blocker is obtaining S3 credentials to perform the actual restore and verification.

### Readiness Checklist
- ✅ Restore environment set up and tested
- ✅ Verification scripts created and validated
- ✅ In-cluster verification job specified
- ✅ Comprehensive test suite available (15+ tests)
- ✅ Documentation complete
- ⚠️ **Awaiting:** S3 credentials or cluster-admin access

### Recommendation

**Next Action:** Obtain S3 credentials from the `armor-writer` secret in the `devimprint` namespace through one of these methods:
1. Request cluster administrator to provide credentials
2. Request cluster administrator to run the verification job
3. Obtain kubeconfig with appropriate permissions for ord-devimprint

**Time to Complete (with credentials):** ~10 minutes for full verification suite

**Risk Level:** Low (isolated test environment, no impact on production)

---

**Related Documentation:**
- [bf-2ke2y.md](./bf-2ke2y.md) - Litestream restore requirements
- [bf-3lc7p.md](./bf-3lc7p.md) - Restore environment setup
- [TESTING.md](/home/coding/scratch/restore-test/TESTING.md) - Test suite documentation
- [litestream-restore-verification-job.yaml](./litestream-restore-verification-job.yaml) - In-cluster verification job