#!/bin/bash
# reset-restore-env.sh - Complete restore environment reset

RESTORE_ENV="/home/coding/ARMOR/scratch/litestream-restore"

echo "=== Litestream Restore Environment Reset ==="
echo "Started at: $(date)"

# Backup existing logs if present
if [ -d "$RESTORE_ENV/logs" ] && [ "$(ls -A $RESTORE_ENV/logs 2>/dev/null)" ]; then
    BACKUP_NAME="$RESTORE_ENV/logs-$(date +%Y%m%d-%H%M%S)"
    echo "Backing up existing logs to: $BACKUP_NAME"
    mv "$RESTORE_ENV/logs" "$BACKUP_NAME"
fi

# Recreate directory structure
echo "Recreating directory structure..."
rm -rf "$RESTORE_ENV"
mkdir -p "$RESTORE_ENV"/{databases,logs,restored,temp}

# Set permissions
chmod 755 "$RESTORE_ENV"
chmod 755 "$RESTORE_ENV"/*

echo "=== Reset Complete ==="
echo "Directory structure:"
ls -la "$RESTORE_ENV"
echo ""
echo "Restore environment reset at $(date)"
