#!/run/current-system/sw/bin/bash
# Automatic log rotation for ARMOR Pluck execution logs
# Run this script manually or schedule via cron

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="/home/coding/ARMOR/logs/pluck-debug"
MAX_SIZE_MB=100
MAX_BACKUPS=5
MIN_DISK_SPACE_MB=500

# Check disk space first
available_mb=$(df -BM "$WORKSPACE" | tail -1 | awk '{print $4}' | sed 's/M//')

if [[ $available_mb -lt $MIN_DISK_SPACE_MB ]]; then
    echo "Warning: Low disk space (${available_mb}MB < ${MIN_DISK_SPACE_MB}MB)"
    echo "Skipping log rotation due to insufficient disk space"
    exit 1
fi

# Function to rotate a single log file
rotate_log() {
    local log_file="$1"
    local max_size="$2"
    local max_backups="$3"

    if [[ ! -f "$log_file" ]]; then
        return 0
    fi

    # Get file size in MB
    local size_mb=$(du -m "$log_file" | cut -f1)

    if [[ $size_mb -lt $max_size ]]; then
        return 0
    fi

    echo "Rotating log file: $log_file (${size_mb}MB)"

    # Rotate existing backups
    for ((i=max_backups-1; i>=1; i--)); do
        local backup="${log_file}.$i"
        if [[ -f "$backup" ]]; then
            local next_backup="${log_file}.$((i+1))"
            mv "$backup" "$next_backup" 2>/dev/null || true
        fi
    done

    # Move current log to .1
    mv "$log_file" "${log_file}.1" 2>/dev/null || true

    # Remove oldest backup if it exceeds max
    local oldest_backup="${log_file}.${max_backups}"
    if [[ -f "$oldest_backup" ]]; then
        rm "$oldest_backup"
        echo "  Removed oldest backup: $oldest_backup"
    fi
}

# Rotate oversized logs
echo "Checking for oversized log files (>${MAX_SIZE_MB}MB)..."
rotated_count=0

for log_file in "$LOG_DIR"/*.log; do
    if [[ -f "$log_file" ]]; then
        rotate_log "$log_file" "$MAX_SIZE_MB" "$MAX_BACKUPS"
        ((rotated_count++))
    fi
done

if [[ $rotated_count -eq 0 ]]; then
    echo "No oversized log files found"
else
    echo "Checked $rotated_count log file(s)"
fi

# Clean up logs older than 30 days
echo ""
echo "Cleaning up log files older than 30 days..."
deleted_count=0

for log_file in "$LOG_DIR"/*.log; do
    if [[ -f "$log_file" ]]; then
        # Check if file is older than 30 days
        if find "$log_file" -mtime +30 -type f 2>/dev/null | grep -q .; then
            rm "$log_file"
            echo "  Deleted old log: $log_file"
            ((deleted_count++))
        fi
    fi
done

# Also check backup files
for backup_file in "$LOG_DIR"/*.log.*; do
    if [[ -f "$backup_file" ]]; then
        if find "$backup_file" -mtime +30 -type f 2>/dev/null | grep -q .; then
            rm "$backup_file"
            echo "  Deleted old backup: $backup_file"
            ((deleted_count++))
        fi
    fi
done

if [[ $deleted_count -eq 0 ]]; then
    echo "No old log files to delete"
else
    echo "Deleted $deleted_count old log file(s)"
fi

echo ""
echo "Log rotation completed"
