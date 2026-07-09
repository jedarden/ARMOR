#!/run/current-system/sw/bin/bash
# Monitor log files and provide rotation recommendations

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="/home/coding/ARMOR/logs/pluck-debug"
MAX_SIZE_MB=100

echo "=== ARMOR Log Rotation Monitor ==="
echo "Timestamp: $(date)"
echo ""

# Analyze log directory
echo "Log Directory Analysis:"
echo "  Location: $LOG_DIR"
echo ""

total_size=0
file_count=0
declare -a large_files
declare -a backup_files

for log_file in "$LOG_DIR"/*.log; do
    if [[ -f "$log_file" ]]; then
        file_size=$(du -m "$log_file" | cut -f1)
        total_size=$((total_size + file_size))
        ((file_count++))

        if [[ $file_size -gt $MAX_SIZE_MB ]]; then
            large_files+=("$log_file (${file_size}MB)")
        elif [[ $file_size -gt 10 ]]; then
            large_files+=("$log_file (${file_size}MB) - approaching limit")
        fi
    fi
done

# Check backup files
for backup_file in "$LOG_DIR"/*.log.*; do
    if [[ -f "$backup_file" ]]; then
        backup_files+=("$backup_file")
    fi
done

echo "Statistics:"
echo "  Total log files: $file_count"
echo "  Total size: ${total_size}MB"
echo "  Backup files: ${#backup_files[@]}"
echo ""

if [[ ${#large_files[@]} -gt 0 ]]; then
    echo -e "\033[1;33mLarge files (need rotation):\033[0m"
    for file in "${large_files[@]}"; do
        echo "  • $file"
    done
    echo ""
    echo "Recommendation: Run $WORKSPACE/scripts/auto-rotate-logs.sh"
else
    echo -e "\033[0;32m✅ All log files are within acceptable size limits\033[0m"
fi

echo ""
echo "Available disk space:"
available_mb=$(df -BM "$WORKSPACE" | tail -1 | awk '{print $4}' | sed 's/M//')
echo "  ${available_mb}MB available"

if [[ $available_mb -lt 500 ]]; then
    echo -e "\033[0;31m⚠️  Low disk space warning!\033[0m"
    echo "Recommendation: Run $WORKSPACE/scripts/auto-rotate-logs.sh immediately"
fi
