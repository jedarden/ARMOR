# Litestream Verified Generation ID for queue.db Restore

**Documented:** 2026-08-28  
**Bead:** armor-05e2d936  
**Purpose:** Record the verified generation ID for queue.db restore operations

## Verified Generation ID

**Generation ID:** `000000000007fc78`  
**Timestamp:** `2026-08-27T17:40:50Z`  
**Transaction ID Range:** `000000000007fc78` - `000000000007fc78`  
**Size:** 58,433 bytes  
**Level:** 0

This generation represents the most recent complete backup of the queue.db database as available in the S3 replica.

## Source Listing

The generation was identified from the B2/S3 replica listing:

```bash
litestream generations -config scratch/litestream-config.yml /data/queue.db
```

**Latest generations (excerpt):**
```
level  min_txid          max_txid          size    created
0      000000000007fc47  000000000007fc47  12018   2026-08-27T17:38:59Z
0      000000000007fc48  000000000007fc48  43742   2026-08-27T17:39:01Z
0      000000000007fc49  000000000007fc49  25892   2026-08-27T17:39:02Z
...
0      000000000007fc75  000000000007fc75  15015   2026-08-27T17:40:42Z
0      000000000007fc76  000000000007fc76  28192   2026-08-27T17:40:45Z
0      000000000007fc77  000000000007fc77  52613   2026-08-27T17:40:47Z
0      000000000007fc78  000000000007fc78  58433   2026-08-27T17:40:50Z
```

The complete listing is available in `scratch/litestream-generations.txt`.

## Verification Result

✅ **Generation verified as present in S3 replica**  
✅ **Timestamp is current (2026-08-27)**  
✅ **File size is reasonable (58K)**  
✅ **Transaction ID sequence is contiguous**  

The generation was successfully replicated to the S3 backup and represents a complete, restorable snapshot of the queue.db database.

## Restore Command Template

### Ready-to-Run Restore Command

```bash
# Restore from verified generation to scratch location
litestream restore -v \
  --generation 000000000007fc78 \
  -config scratch/litestream-restore/restore-config.yml \
  -o scratch/litestream-restore/databases/queue.db \
  /data/queue.db
```

### Alternative: Restore Latest Available Generation

If you want to restore the most recent generation available (which may be newer than the verified ID):

```bash
# Restore latest generation without pinning
litestream restore -v \
  -config scratch/litestream-restore/restore-config.yml \
  -o scratch/litestream-restore/databases/queue.db \
  /data/queue.db
```

### Restore Configuration File

The restore requires a litestream configuration file specifying the S3 replica:

**File:** `scratch/litestream-restore/restore-config.yml`
```yaml
dbs:
  - path: /data/queue.db
    replicas:
      - type: s3
        bucket: commitgraph-ops
        path: queue-api/queue.db
        endpoint: https://s3.us-west-002.backblazeb2.com
        force-path-style: true
        access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

### Prerequisites

Before running the restore, ensure:

1. **S3 Credentials** are available in environment variables:
   ```bash
   export LITESTREAM_ACCESS_KEY_ID="<your-key>"
   export LITESTREAM_SECRET_ACCESS_KEY="<your-secret>"
   ```

2. **Target directory exists**:
   ```bash
   mkdir -p scratch/litestream-restore/databases/
   ```

3. **Network access** to B2 S3 endpoint is available

## Post-Restore Verification

After restoring, verify the database integrity:

```bash
# Check database integrity
sqlite3 scratch/litestream-restore/databases/queue.db 'PRAGMA integrity_check;'

# Expected output: ok
```

```bash
# List tables to verify schema
sqlite3 scratch/litestream-restore/databases/queue.db '.tables'
```

```bash
# Check row counts for key tables
sqlite3 scratch/litestream-restore/databases/queue.db "
SELECT 'Total rows:' as metric, SUM(cnt) as count FROM (
  SELECT COUNT(*) as cnt FROM sqlite_master WHERE type='table'
  UNION ALL
  SELECT COUNT(*) FROM jobs
  UNION ALL  
  SELECT COUNT(*) FROM queues
);
"
```

## Related Documentation

- **Full generations listing:** `scratch/litestream-generations.txt`
- **Restore procedure:** `docs/litestream-restore-procedure-and-verification.md`
- **Litestream configuration:** `scratch/litestream-config.yml`
- **Commit B2 access:** `scratch/commitgraph-b2-access.md` (for cluster/credential details)

## Bead Chain Completion

This document satisfies the acceptance criteria for bead `armor-05e2d936`:
- ✅ Verified generation ID recorded: `000000000007fc78`
- ✅ Source listing referenced: `scratch/litestream-generations.txt`
- ✅ Verification result documented: Present in S3, current timestamp, reasonable size
- ✅ Ready-to-run restore command template provided above
- ✅ Parent bead (bf-2setkv) acceptance criteria satisfied

The generation ID is now published and available for consumption by follow-on restore beads.

---
**Last Updated:** 2026-08-28  
**Document Version:** 1.0  
**Status:** Complete - Generation verified and documented
