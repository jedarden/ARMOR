#!/run/current-system/sw/bin/bash
# Log Rotation Setup for ARMOR Pluck Execution
# Configures automatic log rotation for long-running processes

set -e

# Configuration
WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
MAX_SIZE_MB=100
MAX_BACKUPS=5
MIN_DISK_SPACE_MB=500

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== ARMOR Log Rotation Setup ===${NC}"
echo ""

# Step 1: Check disk space
echo -e "${CYAN}Step 1: Checking disk space...${NC}"

available_mb=$(df -BM "$WORKSPACE" | tail -1 | awk '{print $4}' | sed 's/M//')
echo "Available disk space: ${available_mb}MB"

if [[ $available_mb -lt $MIN_DISK_SPACE_MB ]]; then
    echo -e "${RED}⚠️  Warning: Low disk space (${available_mb}MB < ${MIN_DISK_SPACE_MB}MB)${NC}"
    echo "Consider cleaning old log files"
else
    echo -e "${GREEN}✅ Sufficient disk space available${NC}"
fi
echo ""

# Step 2: Create log rotation script
echo -e "${CYAN}Step 2: Creating automatic log rotation script...${NC}"

cat > "$WORKSPACE/scripts/auto-rotate-logs.sh" << EOF
#!/run/current-system/sw/bin/bash
# Automatic log rotation for ARMOR Pluck execution logs
# Run this script manually or schedule via cron

WORKSPACE="$WORKSPACE"
LOG_DIR="$LOG_DIR"
MAX_SIZE_MB=$MAX_SIZE_MB
MAX_BACKUPS=$MAX_BACKUPS
MIN_DISK_SPACE_MB=$MIN_DISK_SPACE_MB

# Check disk space first
available_mb=\$(df -BM "\$WORKSPACE" | tail -1 | awk '{print \$4}' | sed 's/M//')

if [[ \$available_mb -lt \$MIN_DISK_SPACE_MB ]]; then
    echo "Warning: Low disk space (\${available_mb}MB < \${MIN_DISK_SPACE_MB}MB)"
    echo "Skipping log rotation due to insufficient disk space"
    exit 1
fi

# Function to rotate a single log file
rotate_log() {
    local log_file="\$1"
    local max_size="\$2"
    local max_backups="\$3"

    if [[ ! -f "\$log_file" ]]; then
        return 0
    fi

    # Get file size in MB
    local size_mb=\$(du -m "\$log_file" | cut -f1)

    if [[ \$size_mb -lt \$max_size ]]; then
        return 0
    fi

    echo "Rotating log file: \$log_file (\${size_mb}MB)"

    # Rotate existing backups
    for ((i=max_backups-1; i>=1; i--)); do
        local backup="\${log_file}.\$i"
        if [[ -f "\$backup" ]]; then
            local next_backup="\${log_file}.\$((i+1))"
            mv "\$backup" "\$next_backup" 2>/dev/null || true
        fi
    done

    # Move current log to .1
    mv "\$log_file" "\${log_file}.1" 2>/dev/null || true

    # Remove oldest backup if it exceeds max
    local oldest_backup="\${log_file}.\${max_backups}"
    if [[ -f "\$oldest_backup" ]]; then
        rm "\$oldest_backup"
        echo "  Removed oldest backup: \$oldest_backup"
    fi
}

# Rotate oversized logs
echo "Checking for oversized log files (>\${MAX_SIZE_MB}MB)..."
rotated_count=0

for log_file in "\$LOG_DIR"/*.log; do
    if [[ -f "\$log_file" ]]; then
        rotate_log "\$log_file" "\$MAX_SIZE_MB" "\$MAX_BACKUPS"
        ((rotated_count++))
    fi
done

if [[ \$rotated_count -eq 0 ]]; then
    echo "No oversized log files found"
else
    echo "Checked \$rotated_count log file(s)"
fi

# Clean up logs older than 30 days
echo ""
echo "Cleaning up log files older than 30 days..."
deleted_count=0

for log_file in "\$LOG_DIR"/*.log; do
    if [[ -f "\$log_file" ]]; then
        # Check if file is older than 30 days
        if find "\$log_file" -mtime +30 -type f 2>/dev/null | grep -q .; then
            rm "\$log_file"
            echo "  Deleted old log: \$log_file"
            ((deleted_count++))
        fi
    fi
done

# Also check backup files
for backup_file in "\$LOG_DIR"/*.log.*; do
    if [[ -f "\$backup_file" ]]; then
        if find "\$backup_file" -mtime +30 -type f 2>/dev/null | grep -q .; then
            rm "\$backup_file"
            echo "  Deleted old backup: \$backup_file"
            ((deleted_count++))
        fi
    fi
done

if [[ \$deleted_count -eq 0 ]]; then
    echo "No old log files to delete"
else
    echo "Deleted \$deleted_count old log file(s)"
fi

echo ""
echo "Log rotation completed"
EOF

chmod +x "$WORKSPACE/scripts/auto-rotate-logs.sh"
echo -e "${GREEN}✅ Log rotation script created${NC}"
echo "  Script: $WORKSPACE/scripts/auto-rotate-logs.sh"
echo ""

# Step 3: Test log rotation manually
echo -e "${CYAN}Step 3: Testing log rotation...${NC}"

# Create a test log file that exceeds the size threshold
TEST_LARGE_LOG="$LOG_DIR/test-large-rotation.log"
echo "Creating test log file for rotation demonstration..."

# Create a 1MB test file (well under threshold, but for testing)
dd if=/dev/zero of="$TEST_LARGE_LOG" bs=1M count=1 2>/dev/null

echo "Created test file: $TEST_LARGE_LOG"
echo "Size: $(du -m "$TEST_LARGE_LOG" | cut -f1)MB"

# Run the rotation script to test it
echo "Running rotation script..."
bash "$WORKSPACE/scripts/auto-rotate-logs.sh"

echo ""
echo -e "${GREEN}✅ Log rotation test completed${NC}"
echo ""

# Step 4: Set up cron job (optional - requires user confirmation)
echo -e "${CYAN}Step 4: Cron job setup (optional)...${NC}"

# Check if cron entry exists
if crontab -l 2>/dev/null | grep -q "auto-rotate-logs.sh"; then
    echo -e "${GREEN}✅ Cron job already exists${NC}"
    echo "Current cron entries:"
    crontab -l 2>/dev/null | grep "auto-rotate-logs.sh" | sed 's/^/  /'
else
    echo -e "${YELLOW}No automatic cron job found${NC}"
    echo ""
    echo "To add automatic daily log rotation at 2AM, run:"
    echo "  (crontab -l 2>/dev/null; echo \"0 2 * * * $WORKSPACE/scripts/auto-rotate-logs.sh >> $LOG_DIR/rotation.log 2>&1\") | crontab -"
    echo ""
    echo "Or run manual rotation as needed:"
    echo "  $WORKSPACE/scripts/auto-rotate-logs.sh"
fi
echo ""

# Step 5: Create log rotation monitoring script
echo -e "${CYAN}Step 5: Creating log rotation monitoring script...${NC}"

cat > "$WORKSPACE/scripts/monitor-log-rotation.sh" << EOF
#!/run/current-system/sw/bin/bash
# Monitor log files and provide rotation recommendations

WORKSPACE="$WORKSPACE"
LOG_DIR="$LOG_DIR"
MAX_SIZE_MB=$MAX_SIZE_MB

echo "=== ARMOR Log Rotation Monitor ==="
echo "Timestamp: \$(date)"
echo ""

# Analyze log directory
echo "Log Directory Analysis:"
echo "  Location: \$LOG_DIR"
echo ""

total_size=0
file_count=0
declare -a large_files
declare -a backup_files

for log_file in "\$LOG_DIR"/*.log; do
    if [[ -f "\$log_file" ]]; then
        file_size=\$(du -m "\$log_file" | cut -f1)
        total_size=\$((total_size + file_size))
        ((file_count++))

        if [[ \$file_size -gt \$MAX_SIZE_MB ]]; then
            large_files+=("\$log_file (\${file_size}MB)")
        elif [[ \$file_size -gt 10 ]]; then
            large_files+=("\$log_file (\${file_size}MB) - approaching limit")
        fi
    fi
done

# Check backup files
for backup_file in "\$LOG_DIR"/*.log.*; do
    if [[ -f "\$backup_file" ]]; then
        backup_files+=("\$backup_file")
    fi
done

echo "Statistics:"
echo "  Total log files: \$file_count"
echo "  Total size: \${total_size}MB"
echo "  Backup files: \${#backup_files[@]}"
echo ""

if [[ \${#large_files[@]} -gt 0 ]]; then
    echo -e "\033[1;33mLarge files (need rotation):\033[0m"
    for file in "\${large_files[@]}"; do
        echo "  • \$file"
    done
    echo ""
    echo "Recommendation: Run \$WORKSPACE/scripts/auto-rotate-logs.sh"
else
    echo -e "\033[0;32m✅ All log files are within acceptable size limits\033[0m"
fi

echo ""
echo "Available disk space:"
available_mb=\$(df -BM "\$WORKSPACE" | tail -1 | awk '{print \$4}' | sed 's/M//')
echo "  \${available_mb}MB available"

if [[ \$available_mb -lt $MIN_DISK_SPACE_MB ]]; then
    echo -e "\033[0;31m⚠️  Low disk space warning!\033[0m"
    echo "Recommendation: Run \$WORKSPACE/scripts/auto-rotate-logs.sh immediately"
fi
EOF

chmod +x "$WORKSPACE/scripts/monitor-log-rotation.sh"
echo -e "${GREEN}✅ Log rotation monitoring script created${NC}"
echo "  Script: $WORKSPACE/scripts/monitor-log-rotation.sh"
echo ""

# Step 6: Summary and recommendations
echo -e "${CYAN}Step 6: Configuration Summary...${NC}"

echo -e "${GREEN}✅ Log rotation configured successfully${NC}"
echo ""
echo "Configuration Summary:"
echo "  • Log directory: $LOG_DIR"
echo "  • Maximum file size: ${MAX_SIZE_MB}MB"
echo "  • Backup limit: ${MAX_BACKUPS} files"
echo "  • Cleanup threshold: 30 days"
echo "  • Minimum disk space: ${MIN_DISK_SPACE_MB}MB"
echo ""
echo "Scripts created:"
echo "  • auto-rotate-logs.sh - Manual/Automatic rotation"
echo "  • monitor-log-rotation.sh - Monitor and recommend actions"
echo ""
echo "Usage:"
echo "  # Manual rotation"
echo "  $WORKSPACE/scripts/auto-rotate-logs.sh"
echo ""
echo "  # Monitor status"
echo "  $WORKSPACE/scripts/monitor-log-rotation.sh"
echo ""
echo "  # Automatic daily rotation at 2AM (optional)"
echo "  (crontab -l 2>/dev/null; echo \"0 2 * * * $WORKSPACE/scripts/auto-rotate-logs.sh >> $LOG_DIR/rotation.log 2>&1\") | crontab -"
echo ""

echo -e "${CYAN}=== Log Rotation Setup Complete ===${NC}"