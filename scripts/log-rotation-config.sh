#!/run/current-system/sw/bin/bash
# Log Rotation Configuration for ARMOR Pluck Execution
# Configures automatic log rotation for long-running processes

set -e

# Configuration
WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
CONFIG_DIR="$WORKSPACE/scripts"

# Log rotation settings
MAX_SIZE_MB=100
MAX_BACKUPS=5
MIN_DISK_SPACE_MB=500  # Minimum disk space required

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== ARMOR Log Rotation Configuration ===${NC}"
echo ""

# Function to check disk space
check_disk_space() {
    echo -e "${CYAN}Checking disk space...${NC}"
    local available_mb=$(df -BM "$WORKSPACE" | tail -1 | awk '{print $4}' | sed 's/M//')

    echo "Available disk space: ${available_mb}MB"

    if [[ $available_mb -lt $MIN_DISK_SPACE_MB ]]; then
        echo -e "${RED}⚠️  Warning: Low disk space (${available_mb}MB < ${MIN_DISK_SPACE_MB}MB)${NC}"
        echo "Consider cleaning old log files"
        return 1
    else
        echo -e "${GREEN}✅ Sufficient disk space available${NC}"
        return 0
    fi
}

# Function to rotate a single log file
rotate_log_file() {
    local log_file="$1"
    local max_size_mb="$2"
    local max_backups="$3"

    if [[ ! -f "$log_file" ]]; then
        return 0
    fi

    # Get file size in MB
    local size_mb=$(du -m "$log_file" | cut -f1)

    if [[ $size_mb -lt $max_size_mb ]]; then
        return 0
    fi

    echo -e "${YELLOW}Rotating log file: $log_file (${size_mb}MB)${NC}"

    # Rotate existing backups
    for ((i=max_backups-1; i>=1; i--)); do
        local backup="${log_file}.${i}"
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

    echo -e "${GREEN}✅ Log rotation completed${NC}"
}

# Function to clean up old log files
cleanup_old_logs() {
    local max_age_days=30
    echo -e "${CYAN}Cleaning up log files older than ${max_age_days} days...${NC}"

    local deleted_count=0
    while IFS= read -r -d '' file; do
        rm "$file"
        echo "  Deleted: $file"
        ((deleted_count++))
    done < <(find "$LOG_DIR" -name "*.log" -type f -mtime +$max_age_days -print0 2>/dev/null)

    if [[ $deleted_count -eq 0 ]]; then
        echo "  No old log files to delete"
    else
        echo -e "${GREEN}✅ Deleted ${deleted_count} old log file(s)${NC}"
    fi
}

# Function to analyze log directory
analyze_log_directory() {
    echo -e "${CYAN}Analyzing log directory...${NC}"

    local total_size=0
    local file_count=0
    declare -a large_files

    # Check if directory exists and has log files
    if [[ ! -d "$LOG_DIR" ]]; then
        echo "Log directory does not exist: $LOG_DIR"
        return 1
    fi

    for log_file in "$LOG_DIR"/*.log; do
        if [[ -f "$log_file" ]]; then
            local file_size=$(du -m "$log_file" | cut -f1)
            total_size=$((total_size + file_size))
            ((file_count++))

            if [[ $file_size -gt 10 ]]; then
                large_files+=("$log_file ($file_size MB)")
            fi
        fi
    done

    echo "Total log files: $file_count"
    echo "Total size: ${total_size}MB"

    if [[ ${#large_files[@]} -gt 0 ]]; then
        echo -e "${YELLOW}Large files (>10MB):${NC}"
        for file in "${large_files[@]}"; do
            echo "  - $file"
        done
    fi

    return 0
}

# Function to set up log rotation cron job
setup_cron_rotation() {
    echo -e "${CYAN}Setting up automatic log rotation...${NC}"

    # Create log rotation script
    cat > "$CONFIG_DIR/auto-rotate-logs.sh" << 'EOF'
#!/run/current-system/sw/bin/bash
# Automatic log rotation - runs via cron

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
MAX_SIZE_MB=100
MAX_BACKUPS=5

# Rotate oversized logs
for log_file in "$LOG_DIR"/*.log; do
    if [[ -f "$log_file" ]]; then
        size_mb=$(du -m "$log_file" | cut -f1)
        if [[ $size_mb -gt $MAX_SIZE_MB ]]; then
            # Rotate
            for ((i=MAX_BACKUPS-1; i>=1; i--)); do
                backup="${log_file}.${i}"
                if [[ -f "$backup" ]]; then
                    mv "$backup" "${log_file}.$((i+1))" 2>/dev/null || true
                fi
            done
            mv "$log_file" "${log_file}.1" 2>/dev/null || true
            rm "${log_file}.${MAX_BACKUPS}" 2>/dev/null || true
        fi
    fi
done

# Clean up logs older than 30 days
find "$LOG_DIR" -name "*.log" -type f -mtime +30 -delete
EOF

    chmod +x "$CONFIG_DIR/auto-rotate-logs.sh"

    # Check if cron entry exists
    if ! crontab -l 2>/dev/null | grep -q "auto-rotate-logs.sh"; then
        echo -e "${YELLOW}Adding cron job for daily log rotation...${NC}"
        (crontab -l 2>/dev/null; echo "0 2 * * * $CONFIG_DIR/auto-rotate-logs.sh >> $LOG_DIR/rotation.log 2>&1") | crontab -
        echo -e "${GREEN}✅ Cron job added (runs daily at 2AM)${NC}"
    else
        echo -e "${GREEN}✅ Cron job already exists${NC}"
    fi
}

# Main execution
main() {
    echo -e "${CYAN}=== Log Rotation Setup ===${NC}"
    echo ""

    # Check disk space
    check_disk_space
    echo ""

    # Analyze current log directory
    analyze_log_directory
    echo ""

    # Set up automatic rotation
    setup_cron_rotation
    echo ""

    echo -e "${CYAN}=== Testing Log Rotation ===${NC}"

    # Create a test log file to demonstrate rotation
    local test_log="$LOG_DIR/test-rotation.log"
    echo "Creating test log file for rotation demonstration..."

    # Create a 1MB test file
    dd if=/dev/zero of="$test_log" bs=1M count=1 2>/dev/null
    echo "Test log file created: $test_log"

    echo ""
    echo -e "${CYAN}Manual rotation commands:${NC}"
    echo "  $CONFIG_DIR/auto-rotate-logs.sh           # Run manual rotation"
    echo "  crontab -l                                # View cron schedule"
    echo ""

    echo -e "${GREEN}=== Log Rotation Configuration Complete ===${NC}"
    echo ""
    echo "Summary:"
    echo "  - Log directory: $LOG_DIR"
    echo "  - Maximum file size: ${MAX_SIZE_MB}MB"
    echo "  - Backup limit: ${MAX_BACKUPS} files"
    echo "  - Auto-rotation: Daily at 2AM"
    echo "  - Cleanup: Deletes logs older than 30 days"
}

# Run main function
main "$@"