# Scratch Database Location Preparation - bf-1vqi4k

**Date:** 2026-08-02  
**Task:** Prepare scratch database location for litestream restore  
**Location:** `/home/coding/ARMOR/scratch/litestream-restore/`

## Acceptance Criteria Status

### ✅ 1. Scratch directory exists and is writable
- Directory: `/home/coding/ARMOR/scratch/litestream-restore/`
- Permissions: `755` (rwxr-xr-x)
- Ownership: `coding:users`
- Write test: **PASSED**

### ⚠️ 2. Restore configuration file present and valid
- File: `litestream-restore.yml` exists
- Status: **Present but incomplete**
- Issue: `secret-access-key` field is empty
- Configuration contents:
  ```yaml
  dbs:
    - path: databases/queue.db
      replica:
        type: s3
        bucket: devimprint
        path: state/litestream/queue.db
        endpoint: http://100.80.255.8:9000
        force-path-style: true
        access-key-id: lcs18qaArvWltpK/3oSfFrqiZ/oD7bcGMNYVkW2buD0=
        secret-access-key: # EMPTY
  ```

**Note:** This is a known credential gate documented in bead `bf-34xw9`. The endpoint `http://100.80.255.8:9000` (armor:9000) is ClusterIP-only and unreachable from this host without port-forward access.

### ✅ 3. Required binaries available
- **litestream CLI:** `/home/coding/.local/bin/litestream`
- Version: `(development build)`
- Status: **Available and executable**

### ✅ 4. Target database path defined
- Path: `databases/queue.db`
- Directory exists and is empty (ready for restore)
- Status: **Defined and ready**

### ✅ 5. Sufficient space for restore
- Available space: **70GB** on filesystem
- Status: **Sufficient** (typical queue.db is < 1GB)

## Directory Structure

```
/home/coding/ARMOR/scratch/litestream-restore/
├── databases/          # Target location for restored database files
├── logs/               # Litestream operation logs (5 historical logs present)
├── litestream-restore.yml  # Restore configuration (credential-gated)
└── README.md           # Documentation
```

## Known Issues

### 1. Credential Gate (bf-34xw9)
The configuration references ARMOR (`http://100.80.255.8:9000`) which is:
- ClusterIP-only service (not routable from host)
- No write-access kubeconfig for ord-devimprint
- Empty `secret-access-key` in config

### 2. Obsolete Premise (bf-34xw9 notes)
- `queue-api` moved from `devimprint` namespace to `commitgraph` namespace (July 2026)
- Queue-api now backs up to **B2 directly** (`https://s3.us-west-002.backblazeb2.com`)
- Uses `commitgraph-b2-workers` secret for credentials
- No longer uses ARMOR for backups

## Conclusion

The scratch location **is properly prepared structurally** for restore operations:
- All required directories exist with correct permissions
- Target database path is defined and ready
- Sufficient disk space available
- Litestream CLI installed and available

**However, the restore operation itself is blocked by:**
1. Empty `secret-access-key` in configuration
2. Unreachable ARMOR endpoint from host
3. Obsolete backup location (queue-api now uses B2 directly)

## Recommendation

The scratch environment is ready for restore operations once credentials are available. Consider updating the configuration to point to the current B2 backup location used by the `commitgraph/queue-api` workload.
