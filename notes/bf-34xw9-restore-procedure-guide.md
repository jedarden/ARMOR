# Litestream Restore Procedure Guide

**Bead:** bf-34xw9  
**Purpose:** Complete restore procedure for queue-api backup  
**Status:** Ready to execute once prerequisites are met

## Prerequisites (Must Be Complete Before Restore)

### 1. Credentials (bf-24hrg)
- [ ] LITESTREAM_ACCESS_KEY_ID available: `lcs18qaArvWltpK/3oSfFrqiZ/oD7bcGMNYVkW2buD0=`
- [ ] LITESTREAM_SECRET_ACCESS_KEY retrieved and stored
- [ ] Both credentials accessible in `/tmp/` files or environment

### 2. ARMOR Endpoint Connectivity
- [ ] ARMOR endpoint reachable from restore host
- [ ] Connection test passes: `curl -I http://100.80.255.8:9000`
- [ ] Alternative access path established (port-forward, NodePort, etc.)

### 3. Infrastructure Ready
- [ ] Restore directory exists: `/home/coding/scratch/fresh-restore/`
- [ ] Litestream CLI installed: `~/.local/bin/litestream`
- [ ] Sufficient disk space: 40G+ available
- [ ] SQLite3 available for verification

## Backup Source Information

**S3 Path:** `s3://devimprint/state/litestream/queue.db`  
**Bucket:** `devimprint`  
**Endpoint:** `http://100.80.255.8:9000` (ARMOR S3 proxy)  
**In-Cluster Endpoint:** `http://armor:9000`  
**Force Path Style:** `true`

This is the "new generation" backup configured in the queue-api deployment's litestream configuration.

## Complete Restore Procedure

### Step 1: Prepare Environment

```bash
cd /home/coding/scratch/fresh-restore

# Create directories
mkdir -p databases logs restored

# Verify disk space
df -BG /home/coding/scratch/
# Should show 40G+ available
```

### Step 2: Set Up Credentials

```bash
# Load ACCESS_KEY_ID (cached)
export LITESTREAM_ACCESS_KEY_ID="$(cat /tmp/litestream_access_key_id_clean.txt)"

# Load SECRET_ACCESS_KEY (from bf-24hrg)
export LITESTREAM_SECRET_ACCESS_KEY="$(cat /tmp/litestream_secret_access_key.txt)"

# Verify both are set
echo "ACCESS_KEY_ID: ${LITESTREAM_ACCESS_KEY_ID:0:20}..."
echo "SECRET_ACCESS_KEY: ${LITESTREAM_SECRET_ACCESS_KEY:0:20}..."
```

### Step 3: Create Litestream Restore Configuration

```bash
cat > litestream-restore.yml <<EOF
dbs:
  - path: databases/queue.db
    replica:
      type: s3
      bucket: devimprint
      path: state/litestream/queue.db
      endpoint: http://100.80.255.8:9000
      force-path-style: true
      access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
      secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
EOF
```

**Configuration Parameters:**
- `path`: Local target for restored database
- `type`: s3 (using ARMOR as S3-compatible endpoint)
- `bucket`: devimprint (ARMOR bucket name)
- `path`: state/litestream/queue.db (backup prefix in bucket)
- `endpoint`: ARMOR S3 proxy URL
- `force-path-style`: true (required for ARMOR)
- `access-key-id`: ARMOR authentication
- `secret-access-key`: ARMOR authentication

### Step 4: Test ARMOR Endpoint Connectivity

```bash
# Test basic connectivity
curl -I -s -m 5 http://100.80.255.8:9000

# Expected: HTTP 200 or similar response
# If connection timeout, endpoint is not reachable - do not proceed
```

If using port-forward (alternative method):
```bash
kubectl --kubeconfig=/path/to/kubeconfig \
  port-forward -n devimprint svc/armor 9000:9000 &

export ARMOR_ENDPOINT="http://localhost:9000"
```

### Step 5: Execute Litestream Restore

```bash
# Create log file with timestamp
LOG_FILE="logs/restore-$(date +%Y%m%d-%H%M%S).log"

# Execute restore
litestream restore -config litestream-restore.yml \
  -if-exists overwrite \
  databases/queue.db \
  > "$LOG_FILE" 2>&1

# Check exit code
RESTORE_EXIT=$?
if [ $RESTORE_EXIT -eq 0 ]; then
  echo "✅ Restore completed successfully"
else
  echo "❌ Restore failed with exit code $RESTORE_EXIT"
  echo "Check log file: $LOG_FILE"
  cat "$LOG_FILE"
  exit 1
fi
```

**Litestream Restore Process:**
1. Connects to S3 endpoint (ARMOR)
2. Authenticates with provided credentials
3. Lists available snapshots in `s3://devimprint/state/litestream/queue.db/`
4. Downloads latest snapshot file
5. Applies WAL files for point-in-time recovery
6. Writes restored database to `databases/queue.db`
7. Reports final restored position/timestamp

### Step 6: Verify Restore Success

```bash
# 6a. Verify database file exists and is non-empty
if [ -f databases/queue.db ]; then
  DB_SIZE=$(ls -lh databases/queue.db | awk '{print $5}')
  echo "✅ Database file exists: $DB_SIZE"
  
  # Check file is not empty
  if [ $(stat -f%z databases/queue.db 2>/dev/null || stat -c%s databases/queue.db) -gt 0 ]; then
    echo "✅ Database file is non-empty"
  else
    echo "❌ Database file is empty"
    exit 1
  fi
else
  echo "❌ Database file not created"
  exit 1
fi

# 6b. Database integrity check
echo "Running integrity check..."
sqlite3 databases/queue.db "PRAGMA integrity_check;" | grep -q "^ok$"
if [ $? -eq 0 ]; then
  echo "✅ Database integrity check passed"
else
  echo "❌ Database integrity check failed"
  sqlite3 databases/queue.db "PRAGMA integrity_check;"
  exit 1
fi

# 6c. List tables
echo "Tables in restored database:"
sqlite3 databases/queue.db ".tables"

# 6d. Check row counts
echo "Row counts per table:"
for table in $(sqlite3 databases/queue.db \
  "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;"); do
  count=$(sqlite3 databases/queue.db "SELECT COUNT(*) FROM $table;")
  printf "  %-30s: %d rows\n" "$table" "$count"
done

# 6e. Sample data verification
echo "Sample records from jobs table:"
sqlite3 databases/queue.db "SELECT * FROM jobs LIMIT 5;"
```

### Step 7: Copy to Final Location

```bash
# Move to restored/ directory
cp databases/queue.db restored/queue.db

# Verify copy
ls -lh restored/queue.db

echo "✅ Restore complete: restored/queue.db"
```

## Verification Checklist

- [ ] Database file exists in `restored/queue.db`
- [ ] Database file size is reasonable (not 0 bytes)
- [ ] `PRAGMA integrity_check;` returns `ok`
- [ ] All expected tables present
- [ ] Row counts are non-zero for active tables
- [ ] Sample data can be queried successfully
- [ ] No errors in restore log file

## Troubleshooting

### Restore Fails with "Authentication Failed"

**Cause:** Invalid credentials  
**Solution:**
1. Verify ACCESS_KEY_ID: `echo $LITESTREAM_ACCESS_KEY_ID`
2. Verify SECRET_ACCESS_KEY: `echo $LITESTREAM_SECRET_ACCESS_KEY`
3. Re-retrieve credentials from cluster (requires bf-24hrg)

### Restore Fails with "Connection Timeout"

**Cause:** ARMOR endpoint unreachable  
**Solution:**
1. Test connectivity: `curl -I http://100.80.255.8:9000`
2. Check ARMOR pods: `kubectl get pods -n devimprint -l app=armor`
3. Establish alternative access path (port-forward, NodePort)

### Restore Fails with "No Such Key"

**Cause:** Incorrect backup path or bucket  
**Solution:**
1. Verify bucket name: `devimprint`
2. Verify path prefix: `state/litestream/queue.db`
3. Check litestream config in cluster: `kubectl get configmap queue-api-litestream-config -n devimprint -o yaml`

### Database Integrity Check Fails

**Cause:** Corruption during restore or incomplete download  
**Solution:**
1. Check litestream log for errors: `cat logs/restore-*.log`
2. Re-run restore from scratch
3. If persists, investigate backup integrity in cluster

### Row Counts Are Zero

**Cause:** Backup was empty when snapshot was taken  
**Solution:**
1. Check snapshot creation time: `cat logs/restore-*.log | grep snapshot`
2. Verify queue-api was running when backup was created
3. Consider restoring from earlier generation if available

## Expected Restore Time

- Small database (< 100MB): ~30 seconds
- Medium database (100MB-1GB): ~2-3 minutes  
- Large database (> 1GB): ~5-10 minutes

## Cleanup (After Successful Restore)

```bash
# Keep database and logs for verification
# Temporary files can be removed:
rm -f litestream-restore.yml

# Move restore log to permanent location
mv logs/restore-*.log logs/latest-successful-restore.log
```

## Post-Restore Steps (Bead bf-28vhc)

Once restore is complete and verified, proceed to bead bf-28vhc for:
- Comprehensive data integrity verification
- Test queries to verify data accessibility
- Comparison with expected record counts
- Verification of critical table structures

---

**Document Version:** 1.0  
**Last Updated:** 2026-07-14  
**Author:** Claude Code (claude-code-glm-4.7-alpha)  
**Bead ID:** bf-34xw9  
**Related Beads:** bf-24hrg (credentials), bf-28vhc (verification)