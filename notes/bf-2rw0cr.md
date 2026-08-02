# Queue-API Litestream Backup Generation Identification

## Task Context

Queue-api migrated from devimprint namespace (ARMOR-backed) to commitgraph namespace (B2-backed) in July 2026. The old ARMOR backup location `s3://devimprint/state/litestream/queue.db` is no longer maintained.

This document identifies the correct litestream backup generation for restore and documents the current backup configuration.

## Historical Backup Configuration

### Phase 1: ARMOR-Backed (devimprint namespace) - DEPRECATED

**Time Period:** Before July 20, 2026
**Location:** `s3://devimprint/state/litestream/queue.db`
**Endpoint:** `http://armor:9000` (ARMOR S3 proxy)
**Credentials:** `armor-writer` secret (devimprint namespace)
**ConfigMap:** `queue-api-litestream-config` (devimprint namespace)
**PVC:** `queue-api-data-sata-2`

**Litestream Configuration:**
```yaml
dbs:
  - path: /data/queue.db
    replica:
      type: s3
      bucket: devimprint
      path: state/litestream/queue.db
      endpoint: http://armor:9000
      force-path-style: true
      access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
      secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

**Status:**
- ❌ NO LONGER MAINTAINED
- ❌ Database wiped July 20, 2026 (verified empty, 0 rows)
- ❌ Namespace decommissioned (commit 73953915)
- ❌ PVC stuck in Cinder `detaching` state since 2026-06-21
- ❌ ARMOR endpoint unreachable (ClusterIP-only, port-forward forbidden)

### Phase 2: Initial B2 Migration (commitgraph namespace) - SHORT-LIVED

**Time Period:** July 20, 2026 - July 21, 2026
**Location:** `s3://commitgraph-corpus/queue-api/queue.db`
**Endpoint:** `https://s3.us-west-002.backblazeb2.com`
**Credentials:** `commitgraph-b2-workers` ExternalSecret
**Issue:** PUBLIC bucket (PII exposure risk - author emails in queue.db)

**Commit History:**
- `0b5c9c55` (July 20): Initially configured for `commitgraph-corpus`
- `07d75333` (July 21): Moved to `commitgraph-ops` (private bucket)

### Phase 3: Current B2 Configuration (commitgraph namespace) - ACTIVE

**Time Period:** July 21, 2026 - Present
**Location:** `s3://commitgraph-ops/queue-api/queue.db`
**Endpoint:** `https://s3.us-west-002.backblazeb2.com`
**Credentials:** `commitgraph-b2-workers` ExternalSecret
**Bucket:** `commitgraph-ops` (PRIVATE - allPrivate)
**ConfigMap:** `queue-api-litestream-config` (commitgraph namespace)
**PVC:** `queue-api-data`

**Current Litestream Configuration:**
```yaml
dbs:
  - path: /data/queue.db
    replica:
      type: s3
      bucket: commitgraph-ops
      path: queue-api/queue.db
      endpoint: https://s3.us-west-002.backblazeb2.com
      force-path-style: true
      access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
      secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

**ExternalSecret Details:**
```yaml
externalSecret:
  secretStoreRef:
    name: openbao
  data:
    - secretKey: key-id
      remoteRef.key: rs-manager/ord-devimprint/b2-workers
      remoteRef.property: key_id
    - secretKey: application-key
      remoteRef.key: rs-manager/ord-devimprint/b2-workers
      remoteRef.property: application_key
    - secretKey: bucket
      remoteRef.key: rs-manager/ord-devimprint/b2-workers
      remoteRef.property: bucket
    - secretKey: prefix
      remoteRef.key: rs-manager/ord-devimprint/b2-workers
      remoteRef.property: prefix
```

## Database Wipe and Migration

**July 20, 2026 (commit c164a36b):**
- Pre-2.0.0 queue.db verified empty (0 rows in all tables)
- Database schema migration from 1.0.3 to 2.0.0 failed
- One-time wipe executed: `rm -f /data/queue.db /data/queue.db-wal /data/queue.db-shm`
- Litestream state cleared: `rm -rf /data/.queue.db-litestream /data/queue.db-ltx`
- Fresh 2.0.0 database initialized at pod startup

**Migration Path:**
```
devimprint namespace (ARMOR)
    ↓ July 20, 2026
commitgraph namespace (B2 - commitgraph-corpus) [1 day]
    ↓ July 21, 2026
commitgraph namespace (B2 - commitgraph-ops) [CURRENT]
```

## Backup Generation Status

### Old ARMOR Location (`s3://devimprint/state/litestream/queue.db`)

**Status: ❌ UNUSABLE FOR RESTORE**

**Blockers:**
1. **Credential Gate:** `armor-writer` secret inaccessible via read-only proxy
2. **Endpoint Gate:** `http://armor:9000` unreachable (ClusterIP-only, port-forward forbidden)
3. **Data Obsolescence:** Pre-2.0.0 database was empty before wipe
4. **Litestream State:** Local `.queue.db-litestream` directory deleted during wipe

**Verification Attempts:**
- `bf-34xw9`: 22+ retries hit identical credential gate
- `bf-24hrg`: S3 credential retrieval still OPEN
- NEEDLE retry-storm anti-pattern (ADR-004)

**Litestream Restore Command (for reference):**
```bash
litestream restore -if-replica-exists /data/queue.db \
  --config /path/to/litestream.yml
```

Expected failure: `get credentials: failed to refresh cached credentials, no EC2 IMDS role found`

### Current B2 Location (`s3://commitgraph-ops/queue-api/queue.db`)

**Status: ✅ ACTIVE AND VALID**

**Backup Generation:** Fresh baseline established July 21, 2026

**Verification:**
- ✅ Litestream sidecar running and replicating
- ✅ Pod healthy: `queue-api-c5894c469-p9rhr` (2/2 Running)
- ✅ ExternalSecret synced: `commitgraph-b2-workers` (last refreshed 2026-08-02)
- ✅ Bucket private: `commitgraph-ops` (allPrivate)
- ✅ No PII exposure: Author emails protected

**Current Activity (as of 2026-08-02):**
- Current TXID: 0x27ffc (~163,840 transactions)
- Actively uploading ltx files (level 0) and performing compaction
- Retention policy active (deleting old l0 files)
- Compaction to level 1 files (196KB size)

**Credential Access:**
- Source: OpenBao via `rs-manager/ord-devimprint/b2-workers`
- ExternalSecret: `commitgraph-b2-workers` (commitgraph namespace)
- Last Sync: 2026-08-02T18:17:56Z
- Keys: `key-id`, `application-key`, `bucket`, `prefix`, `region`

## Recommended Restore Target

### For Disaster Recovery: Current B2 Generation

**Target:** `s3://commitgraph-ops/queue-api/queue.db` (current active generation)

**Rationale:**
1. ✅ Only valid, accessible backup location
2. ✅ Contains post-2.0.0 database schema
3. ✅ Private bucket (no PII exposure)
4. ✅ Actively maintained and replicating
5. ✅ Credentials available via ExternalSecret

**Restore Procedure (Conceptual):**
```yaml
# Litestream restore init container (pattern from old deployment)
initContainers:
  - name: litestream-restore
    image: litestream/litestream:0.5.11
    command:
      - /bin/sh
      - -c
      - |
        if [ -f /data/queue.db ]; then
          echo "Database exists, skipping restore"
        else
          echo "No database found, restoring from WAL backup"
          litestream restore -if-replica-exists /data/queue.db || {
            echo "Restore failed — starting with empty database"
            exit 0
          }
        fi
    env:
      - name: LITESTREAM_ACCESS_KEY_ID
        valueFrom:
          secretKeyRef:
            name: commitgraph-b2-workers
            key: key-id
      - name: LITESTREAM_SECRET_ACCESS_KEY
        valueFrom:
          secretKeyRef:
            name: commitgraph-b2-workers
            key: application-key
    volumeMounts:
      - name: data
        mountPath: /data
      - name: litestream-config
        mountPath: /etc/litestream.yml
        subPath: litestream.yml
```

### For Historical Recovery: ARMOR Location - BLOCKED

**Target:** `s3://devimprint/state/litestream/queue.db` (old generation)

**Status:** ❌ CREDENTIAL + ENDPOINT GATED

**Blockers:**
1. No `armor-writer` secret access (read-only proxy)
2. ARMOR endpoint `http://armor:9000` unreachable
3. Empty pre-wipe database (0 rows, no data loss)
4. Premise obsolete - queue-api migrated to B2

**Unblocking Requirements:**
1. Valid ARMOR credentials (bf-24hrg)
2. Reachable ARMOR endpoint (port-forward access)
3. Verify old backup generation still exists
4. Confirm data value (database was empty)

**Recommendation:** Do NOT pursue ARMOR restore - no data value, high effort, multiple blockers

## Acceptance Criteria Status

- ✅ **Documented current queue-api backup location and configuration**
  - Location: `s3://commitgraph-ops/queue-api/queue.db`
  - Endpoint: `https://s3.us-west-002.backblazeb2.com`
  - Credentials: `commitgraph-b2-workers` ExternalSecret
  - Bucket: `commitgraph-ops` (private, allPrivate)
  - Current TXID: 0x27ffc

- ✅ **Identified if any valid backups exist in the old ARMOR location**
  - Location exists: `s3://devimprint/state/litestream/queue.db`
  - Status: UNUSABLE (credential + endpoint gated)
  - Data value: NONE (database was empty before wipe)

- ✅ **Confirmed the backup generation to target for restore**
  - Target: Current B2 generation (post-July 21, 2026)
  - Location: `s3://commitgraph-ops/queue-api/queue.db`
  - Valid: Yes (actively replicating)

- ✅ **Documented any credential requirements for accessing the backup**
  - ARMOR: `armor-writer` secret (INACCESSIBLE via proxy)
  - B2: `commitgraph-b2-workers` ExternalSecret (AVAILABLE via OpenBao)
  - OpenBao path: `rs-manager/ord-devimprint/b2-workers`
  - Keys: `key-id`, `application-key`, `bucket`, `prefix`, `region`

## References

- Commits:
  - `c164a36b` (July 20): One-time queue.db wipe
  - `0b5c9c55` (July 20): Initial B2 configuration (commitgraph-corpus)
  - `07d75333` (July 21): Move to commitgraph-ops (private bucket)
  - `a2070281` (July 20): queue-api 2.0.0 bump
  - `73953915`: Devimprint pipeline decommission

- Beads:
  - `bf-34xw9`: Litestream restore (CREDENTIAL + ENDPOINT GATED)
  - `bf-24hrg`: S3 credential retrieval (OPEN)
  - `bf-5aqh0`: Queue-api restore (PREMISE OBSOLETE)
  - `bf-36zo2`: Litestream fresh snapshot (executed)

- Documentation:
  - `docs/disaster-recovery.md`: No litestream procedure
  - `docs/litestream-restore-procedure-and-verification.md`: Lists restore as FAILED/blocked
  - ADR-004: NEEDLE retry-storm anti-pattern

## Conclusion

The correct backup generation for queue-api restore is the **current B2-backed generation** at `s3://commitgraph-ops/queue-api/queue.db`. This is the only valid, accessible, and maintained backup location.

The old ARMOR location (`s3://devimprint/state/litestream/queue.db`) is unusable due to credential and endpoint blockers, and contains no valuable data (empty database before wipe).

**Recommendation:** Use current B2 generation for any disaster recovery scenarios. Do NOT pursue ARMOR restore without explicit business justification (effort exceeds value, multiple blockers, no data loss risk).
