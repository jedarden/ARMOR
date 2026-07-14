# Litestream Restore Test Environment

## Scratch Location

**Primary scratch directory:** `/home/coding/ARMOR/scratch/restore-test/`

**Target database directory:** `/home/coding/ARMOR/scratch/restore-test/data/`

## Directory Structure

```
scratch/restore-test/
└── data/           # Target directory for restored database files
```

## Setup Verification

- ✅ Scratch directory created with permissions `755`
- ✅ Litestream CLI available at `/home/coding/.local/bin/litestream`
- ✅ Litestream version: (development build)
- ✅ Target database directory structure created

## Cleanup Procedure

To clean up the scratch environment after testing:

```bash
# Remove the entire scratch directory
rm -rf /home/coding/ARMOR/scratch/restore-test/

# Or remove just the restored database
rm -f /home/coding/ARMOR/scratch/restore-test/data/*.db
```

## Litestream Restore Commands

To restore a database from litestream backup:

```bash
# Basic restore command structure
litestream restore -o scratch/restore-test/data/<dbname>.db <backup-url>

# Example for S3 backup
litestream restore -o scratch/restore-test/data/armor.db s3://bucket/path/armor.db

# Example for file-based backup
litestream restore -o scratch/restore-test/data/armor.db file:///path/to/backup
```

## Notes

- The scratch directory is intentionally outside the main data directory to avoid conflicts
- All restored databases will be placed in the `data/` subdirectory
- This environment is safe for testing restore operations without affecting production data

## Created

2026-07-14 - Bead bf-jvsio
