# Litestream Restore Environment

This directory is used for testing litestream restore operations for ARMOR database backups.

## Directory Structure

- `databases/` - Target location for restored database files
- `logs/` - Litestream operation logs
- `restored/` - Final verified restored databases  
- `temp/` - Temporary files during restore operations

## Scripts

- `../../scripts/cleanup-restore-env.sh` - Clean working directories
- `../../scripts/reset-restore-env.sh` - Complete environment reset

## Usage

Example restore command (when S3 credentials are available):
```bash
litestream restore -config /path/to/litestream.yml \
  replicas/* \
  databases/queue.db > logs/restore-$(date +%Y%m%d-%H%M%S).log 2>&1
```

## Documentation

See `/home/coding/ARMOR/notes/bf-jvsio-litestream-restore-environment.md` for complete documentation.
