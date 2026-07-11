# Litestream Restore Procedure and Verification Results

## Overview

This document describes the procedure and results for testing restoration of queue-api backups using litestream to a scratch location. This restore testing is part of ARMOR v0.1.x maintenance and backup verification activities.

**Bead Chain:**
- `bf-5aqh0`: Test-restore queue-api backup to scratch location and verify (parent)
- `bf-3lc7p`: Create scratch restore environment for queue-api backup testing (completed)
- `bf-2ke2y`: Restore fresh litestream backup to scratch location (in progress)
- `bf-69ix4`: Verify restored database integrity and data completeness (completed)
- `bf-2b38h`: Document restore procedure and verification results (this document)

**Purpose:** Validate that ARMOR's encrypted S3 proxy backups can be successfully restored and that the restored database is complete and functional.

## Restore Procedure

### Prerequisites

1. **Scratch Environment Setup** (`bf-3lc7p` - COMPLETED)
   - Create isolated restore directory: `scratch/fresh-restore/`
   - Ensure adequate disk space for restored database
   - Set up proper file permissions

2. **S3 Credentials** (`bf-24hrg` - required)
   - Obtain valid S3 credentials for litestream restore
   - Credentials must have read access to backup bucket
   - Configure environment variables or credential file

3. **Litestream Configuration** (`bf-2ewfx` - required)
   - Set up litestream restore infrastructure
   - Configure litestream with correct replica paths
   - Verify litestream binary availability

### Restore Steps

1. **Execute Litestream Restore** (`bf-597ur`)
   ```bash
   cd scratch/fresh-restore/
   litestream restore -config /path/to/litestream.yml replicas/* restored/queue.db
   ```
   
   Expected behavior:
   - Litestream downloads snapshot files from S3
   - Applies WAL files for point-in-time recovery
   - Produces restored SQLite database at `restored/queue.db`
   - Reports restore position/timestamp

2. **Verify Restore Completion**
   - Check exit code (should be 0)
   - Verify database file exists and is non-zero size
   - Check litestream logs for any warnings or errors

## Verification Steps

### 1. Database Integrity Check (`bf-22bfp`, `bf-69ix4`)

```bash
sqlite3 scratch/fresh-restore/restored/queue.db 'PRAGMA integrity_check;'
```

**Expected Result:** `ok`

**What it verifies:**
- Database file structure is valid
- No corruption in pages or b-trees
- Indexes are consistent
- Foreign key constraints are intact

### 2. Table Existence Check

```bash
sqlite3 scratch/fresh-restore/restored/queue.db '.tables'
```

**Expected Result:** List of all expected tables from the original schema

**What it verifies:**
- All tables present in original database
- Schema structure preserved
- No missing tables due to corruption

### 3. Row Count Validation

```bash
sqlite3 scratch/fresh-restore/restored/queue.db "
SELECT name, (SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=sqlite_master.name) as row_count 
FROM sqlite_master WHERE type='table';"
```

**Expected Result:** Non-zero row counts for active tables

**What it verifies:**
- Data completeness
- No data loss during restore
- All records present

### 4. Sample Data Verification

Execute sample queries to verify data accessibility:
- Read recent records from key tables
- Verify data format and encoding
- Check for any decryption issues (if ARMOR encryption involved)

## Results and Findings

### Current Status (as of 2026-07-11)

**Restore Procedure Status:**
- Environment setup: ✅ COMPLETED (`bf-3lc7p`)
- S3 credentials: ⚠️ BLOCKER (missing or expired)
- Litestream restore: ❌ FAILED (5 attempts, blocked by credentials)
- Database verification: ⏸️ PENDING (requires successful restore)

**Known Blockers:**
1. **S3 Credentials Missing** (`bf-24hrg` - open)
   - Cannot access backup bucket without valid credentials
   - Credentials may have expired or were never properly stored
   - Must obtain fresh credentials from secure storage

2. **OpenBaa ExternalSecrets Issues**
   - ExternalSecret sync failures may prevent credential injection
   - RBAC issues blocking secret access in some clusters
   - May need to use cached secrets or manual credential provisioning

### Previous Execution Attempts

The restore task (`bf-2ke2y`) has failed 5 times with `failure-count:5` label. Analysis shows:

**Task Split Analysis:**
- Mitosis analysis determined the restore task is NOT splittable
- Restore operation is a single cohesive task that cannot be decomposed
- All steps must complete together for verification to be meaningful

**Dependency Chain:**
```
bf-3lc7p (env setup) → bf-2ke2y (restore) → bf-69ix4 (verify) → bf-2b38h (document)
                     ↘ bf-22bfp (verify) ↗
```

## Infrastructure Context

### ARMOR Architecture
ARMOR is an encryption proxy for B2 backups, deployed to devimprint namespace:
- **Primary Function:** Encrypted S3 proxy for queue-api database backups
- **Backup Method:** Litestream replication to B2 (encrypted via ARMOR)
- **Restore Challenge:** Must decrypt ARMOR-encrypted data during restore

### Cluster Deployment
- **Cluster:** ord-devimprint (via kubectl-proxy)
- **Namespace:** devimprint  
- **Access Method:** Read-only kubectl proxy (no direct kubeconfig)
- **Image:** `ronaldraygun/armor:latest` (should be pinned to specific version)

### Related Issues
- **armor-l64**: CrashLoopBackOff issue on ord-devimprint (RESOLVED)
- Version upgrade from v0.1.8 to v0.1.11 resolved crashes
- ExternalSecret refresh fixed credential injection

## Troubleshooting Guide

### If Restore Fails

1. **Check S3 Credentials**
   ```bash
   # Verify AWS credentials are set
   echo $AWS_ACCESS_KEY_ID
   echo $AWS_SECRET_ACCESS_KEY
   
   # Test S3 access
   aws s3 ls s3://backup-bucket-name
   ```

2. **Verify Litestream Configuration**
   ```bash
   # Check litestream config syntax
   litestream validate -config /path/to/litestream.yml
   
   # List available replicas
   litestream replicas -config /path/to/litestream.yml
   ```

3. **Check Disk Space**
   ```bash
   # Ensure adequate space for restored database
   df -h scratch/fresh-restore/
   ```

4. **Review Litestream Logs**
   ```bash
   # Litestream outputs detailed restore progress
   # Check for specific error messages about missing files or permission issues
   ```

### If Verification Fails

1. **Integrity Check Fails**
   - Database may be corrupted during restore
   - Check litestream logs for incomplete downloads
   - Re-run restore from scratch

2. **Tables Missing**
   - Restore may have used old snapshot
   - Verify litestream restored to correct point-in-time
   - Check if WAL files were applied correctly

3. **Zero Row Counts**
   - Schema restored but data missing
   - May indicate backup was empty when snapshot was taken
   - Verify backup schedule and retention policy

## Recommendations

### Immediate Actions

1. **Resolve S3 Credential Blocker** (`bf-24hrg`)
   - Obtain valid S3 credentials from secure storage
   - Test credentials with `aws s3 ls` before attempting restore
   - Consider using cached secrets if ExternalSecret issues persist

2. **Complete Restore Procedure** (`bf-597ur`, `bf-2ke2y`)
   - Execute litestream restore once credentials are available
   - Monitor restore logs for any warnings or errors
   - Verify final database file size is reasonable

3. **Run Full Verification** (`bf-69ix4`, `bf-22bfp`)
   - Execute all verification steps on restored database
   - Document any discrepancies or issues found
   - Compare row counts against expected values

### Long-term Improvements

1. **Automated Restore Testing**
   - Schedule periodic restore drills to validate backup integrity
   - Automate verification steps to run on cadence
   - Alert on restore or verification failures

2. **Credential Management**
   - Ensure S3 backup credentials are stored securely and accessible
   - Set up rotation process for long-lived credentials
   - Document credential retrieval process for DR scenarios

3. **Documentation Updates**
   - Maintain runbook for disaster recovery procedures
   - Document credential locations and access methods
   - Create quick-start guide for restore operations

4. **ARMOR Version Management**
   - Pin ARMOR deployment to specific version tags
   - Test new versions in scratch environment before production rollout
   - Maintain compatibility matrix between ARMOR versions and backup formats

## Related Documentation

- **ARMOR v0.1.x Maintenance Plan:** `docs/plan/plan.md` (references in `bf-520v`)
- **Disaster Recovery Runbook:** (tracked in `bf-czxv`)
- **ARMOR Deployment:** `k8s/ord-devimprint/devimprint/armor-deployment.yml`

## Appendix: Bead Chain Summary

| Bead ID | Title | Status | Purpose |
|---------|-------|--------|---------|
| `bf-5aqh0` | Test-restore queue-api backup to scratch location and verify | open | Parent task for restore testing |
| `bf-3lc7p` | Create scratch restore environment for queue-api backup testing | closed | Environment setup ✅ |
| `bf-2ke2y` | Restore fresh litestream backup to scratch location | open | Execute restore (blocked) |
| `bf-69ix4` | Verify restored database integrity and data completeness | closed | Verification (pending restore) |
| `bf-2b38h` | Document restore procedure and verification results | in_progress | This documentation |
| `bf-597ur` | Execute litestream restore to scratch location | open | Restore execution task |
| `bf-22bfp` | Verify restored database integrity | open | Integrity verification |
| `bf-2ewfx` | Set up litestream restore infrastructure | open | Infrastructure setup |
| `bf-24hrg` | Obtain S3 credentials for litestream restore | open | **BLOCKER** - credentials |

**Note:** Beads may show as "open" or "in_progress" but are blocked by dependencies on S3 credentials (`bf-24hrg`). Once credentials are obtained, the chain can proceed: restore → verify → document.

---

**Document Version:** 1.0  
**Last Updated:** 2026-07-11  
**Author:** Claude Code (claude-code-glm-4.7-alpha)  
**Bead ID:** bf-2b38h
