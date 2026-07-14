# Litestream Restore Environment Setup

**Bead ID:** bf-jvsio  
**Created:** 2026-07-14  
**Purpose:** Scratch database location and restore environment for litestream testing

## Environment Overview

This document describes the scratch restore environment created for testing litestream database restores. The environment provides an isolated location for restoring ARMOR-encrypted SQLite backups without affecting production data.

## Directory Structure

```
/home/coding/ARMOR/scratch/litestream-restore/
├── databases/      # Target location for restored databases
├── logs/          # Litestream restore operation logs
├── restored/      # Final restored database files
└── temp/          # Temporary files during restore operations
```

### Directory Purposes

- **`databases/`**: Target directory where litestream will place restored database files
- **`logs/`**: Storage for litestream operation logs and restore output
- **`restored/`**: Final location for successfully restored and verified databases
- **`temp/`**: Temporary workspace for intermediate files during restore operations

## Environment Verification

### Disk Space Status (as of 2026-07-14)
- **Total Disk Space:** 444G
- **Used Space:** 382G (91%)
- **Available Space:** 40G
- **Scratch Directory Usage:** 32K (empty, ready for use)

**Note:** 40G available is sufficient for litestream restore testing. Typical ARMOR database restores are expected to be under 5G.

### Litestream CLI Status

**Binary Location:** `/home/coding/.local/bin/litestream`  
**Version:** (development build)  
**Status:** ✅ Functional and available

**Available Commands:**
- `restore` - Recover database backup from replica
- `replicate` - Run replication server
- `databases` - List databases in config
- `status` - Display replication status
- `ltx` - List available LTX files

### Directory Permissions

All directories created with `755` permissions (rwxr-xr-x):
- Owner: `coding` user
- Group: `users` group
- Access: Read/write/execute for owner, read/execute for group/others

## Usage Example

### Basic Restore Command

```bash
cd /home/coding/ARMOR/scratch/litestream-restore/

# Execute restore (when credentials are available)
litestream restore -config /path/to/litestream.yml \
  replicas/* \
  databases/queue.db > logs/restore-$(date +%Y%m%d-%H%M%S).log 2>&1
```

### Restore with Monitoring

```bash
# Start restore with timestamped logging
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
litestream restore -config /path/to/litestream.yml \
  replicas/* \
  databases/queue.db 2>&1 | tee logs/restore-$TIMESTAMP.log

# Monitor progress
tail -f logs/restore-$TIMESTAMP.log
```

### Post-Restore Verification

```bash
# Move restored database to final location
mv databases/queue.db restored/queue-$TIMESTAMP.db

# Verify database integrity
sqlite3 restored/queue-$TIMESTAMP.db 'PRAGMA integrity_check;'

# Check database size
ls -lh restored/queue-$TIMESTAMP.db
```

## Cleanup Procedures

### Manual Cleanup

```bash
# Remove specific restore artifacts
rm -rf /home/coding/ARMOR/scratch/litestream-restore/databases/*
rm -rf /home/coding/ARMOR/scratch/litestream-restore/temp/*

# Archive logs before cleanup
mkdir -p /home/coding/ARMOR/scratch/litestream-restore/logs/archive
mv /home/coding/ARMOR/scratch/litestream-restore/logs/*.log \
   /home/coding/ARMOR/scratch/litestream-restore/logs/archive/

# Complete reset (remove all restore data)
rm -rf /home/coding/ARMOR/scratch/litestream-restore/*
```

### Automated Cleanup Script

```bash
#!/bin/bash
# cleanup-restore-env.sh - Clean restore environment while preserving logs

RESTORE_ENV="/home/coding/ARMOR/scratch/litestream-restore"

# Archive current logs
if [ -d "$RESTORE_ENV/logs" ]; then
    mkdir -p "$RESTORE_ENV/logs/archive"
    mv "$RESTORE_ENV/logs"/*.log "$RESTORE_ENV/logs/archive/" 2>/dev/null
fi

# Clean working directories
rm -rf "$RESTORE_ENV/databases"/*
rm -rf "$RESTORE_ENV/temp"/*

echo "Restore environment cleaned at $(date)" | tee -a "$RESTORE_ENV/logs/cleanup.log"
```

### Complete Environment Reset

```bash
#!/bin/bash
# reset-restore-env.sh - Complete restore environment reset

RESTORE_ENV="/home/coding/ARMOR/scratch/litestream-restore"

# Backup existing logs if present
if [ -d "$RESTORE_ENV/logs" ] && [ "$(ls -A $RESTORE_ENV/logs)" ]; then
    mv "$RESTORE_ENV/logs" "$RESTORE_ENV/logs-$(date +%Y%m%d-%H%M%S)"
fi

# Recreate directory structure
rm -rf "$RESTORE_ENV"
mkdir -p "$RESTORE_ENV"/{databases,logs,restored,temp}
chmod 755 "$RESTORE_ENV"
chmod 755 "$RESTORE_ENV"/*

echo "Restore environment reset at $(date)"
```

## Prerequisites for Restore Operations

Before executing restore operations, ensure the following are available:

### 1. Litestream Configuration File
- Path to `litestream.yml` configuration
- Contains replica definitions and S3/B2 credentials
- Validated with `litestream validate -config <path>`

### 2. S3/B2 Credentials
- Valid `AWS_ACCESS_KEY_ID`
- Valid `AWS_SECRET_ACCESS_KEY`
- Proper bucket permissions (read access for restore)
- Endpoint configuration if using B2/other S3-compatible storage

### 3. Target Database Information
- Original database name and expected schema
- Expected approximate size (for validation)
- Last known good backup timestamp

## Troubleshooting

### Permission Issues

If permission errors occur:
```bash
# Reset permissions
chmod 755 /home/coding/ARMOR/scratch/litestream-restore
chmod 755 /home/coding/ARMOR/scratch/litestream-restore/*
```

### Disk Space Issues

If disk space becomes insufficient:
```bash
# Check current usage
df -h /home/coding/ARMOR/scratch/litestream-restore/

# Clean old logs
find /home/coding/ARMOR/scratch/litestream-restore/logs/ -name "*.log" -mtime +7 -delete

# Remove old restored databases
find /home/coding/ARMOR/scratch/litestream-restore/restored/ -name "*.db" -mtime +30 -delete
```

### Litestream Command Issues

If litestream commands fail:
```bash
# Verify litestream installation
which litestream
litestream version

# Check for conflicting processes
ps aux | grep litestream

# Test with simple command
litestream databases -config /path/to/litestream.yml
```

## Safety Considerations

1. **Isolated Environment**: Scratch directory is completely isolated from production data
2. **No Auto-Cleanup**: Manual cleanup required - prevents accidental data loss
3. **Log Preservation**: All operations create timestamped logs for audit trail
4. **Permission Safety**: Directory structure prevents accidental overwrites
5. **Disk Space Monitoring**: 40G available provides buffer for multiple restore attempts

## Integration with ARMOR Workflow

This restore environment supports the ARMOR v0.1.x maintenance workflow:

- **Testing**: Validate restore procedures without production impact
- **Verification**: Confirm backup integrity through actual restore operations
- **Documentation**: Build disaster recovery runbook through practical testing
- **Development**: Test ARMOR encryption/decryption with real backup data

## Related Documentation

- **Litestream Restore Procedure**: `/home/coding/ARMOR/docs/litestream-restore-procedure-and-verification.md`
- **ARMOR v0.1.x Maintenance Plan**: `docs/plan/plan.md`
- **ARMOR Deployment**: `k8s/ord-devimprint/devimprint/armor-deployment.yml`

## Next Steps

Once this environment is set up, the following tasks can proceed:

1. **Obtain S3 Credentials** (bead: bf-24hrg) - Required for restore access
2. **Execute Restore** (bead: bf-2ke2y) - Perform actual restore to scratch location
3. **Verify Integrity** (bead: bf-69ix4) - Validate restored database
4. **Document Results** (bead: bf-2b38h) - Update restore procedure documentation

---

**Environment Status:** ✅ READY  
**Disk Space:** 40G available (sufficient)  
**Litestream CLI:** ✅ Functional  
**Directory Structure:** ✅ Complete  
**Permissions:** ✅ Correct  
**Documentation:** ✅ Complete  

**Ready for restore operations pending S3 credential availability.**
