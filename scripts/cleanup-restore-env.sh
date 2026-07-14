#!/bin/bash
# cleanup-restore-env.sh - Clean restore environment while preserving logs

RESTORE_ENV="/home/coding/ARMOR/scratch/litestream-restore"

echo "=== Litestream Restore Environment Cleanup ==="
echo "Started at: $(date)"

# Archive current logs if they exist
if [ -d "$RESTORE_ENV/logs" ]; then
    mkdir -p "$RESTORE_ENV/logs/archive"

    # Check if there are any log files to archive
    if [ "$(ls -A $RESTORE_ENV/logs/*.log 2>/dev/null)" ]; then
        echo "Archiving log files..."
        mv "$RESTORE_ENV/logs"/*.log "$RESTORE_ENV/logs/archive/" 2>/dev/null
        echo "Logs archived to: $RESTORE_ENV/logs/archive/"
    else
        echo "No log files to archive"
    fi
fi

# Clean working directories
echo "Cleaning databases directory..."
rm -rf "$RESTORE_ENV/databases"/*

echo "Cleaning temp directory..."
rm -rf "$RESTORE_ENV/temp"/*

# Log cleanup action
echo "Restore environment cleaned at $(date)" | tee -a "$RESTORE_ENV/logs/cleanup.log"

echo "=== Cleanup Complete ==="
echo "Disk space after cleanup:"
df -h "$RESTORE_ENV" | tail -1 | awk '{print "  Available: " $4 " of " $2}'
