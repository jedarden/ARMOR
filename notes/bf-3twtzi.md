# Bead bf-3twtzi: Litestream restore.sh Script

## Completed

Created the litestream restore script at `/home/coding/scratch/fresh-restore/restore.sh` with the following features:

### Script Capabilities
- **Location**: `/home/coding/scratch/fresh-restore/restore.sh`
- **Executable**: Yes (`chmod +x` applied)
- **Shebang**: `#!/usr/bin/env bash`

### Features Implemented
1. **Error handling for missing arguments**
   - Validates exactly 2 arguments are provided
   - Checks that neither argument is empty
   - Displays usage information on errors

2. **Usage documentation**
   - Comprehensive comments explaining script purpose
   - Usage examples for both S3 and file path backups
   - Clear argument descriptions

3. **Litestream integration**
   - Checks for litestream binary availability
   - Uses `litestream restore` command with `-o` output flag
   - Handles both S3 and local file path backups

4. **Additional safeguards**
   - Creates output directory if it doesn't exist
   - Proper exit codes (0=success, 1=usage error, 2=missing binary)
   - `set -euo pipefail` for strict error handling

### Usage Examples
```bash
# Restore from S3 backup
./restore.sh s3://my-bucket/backups/Armor.db /tmp/Armor.db

# Restore from file backup
./restore.sh /backup/path /tmp/restored.db
```
