#!/run/current-system/sw/bin/bash
# Output redirection template 2: Combined stdout/stderr with timestamps

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
COMBINED_LOG="$LOG_DIR/pluck-execution-combined-${TIMESTAMP}.log"

# Execute command with combined output and timestamps
{
    echo "=== Execution started at $(date) ==="
    your_command_here 2>&1
    EXIT_CODE=$?
    echo "=== Execution completed at $(date) with exit code: $EXIT_CODE ==="
} > "$COMBINED_LOG"

# Display result
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Command completed successfully"
    cat "$COMBINED_LOG"
else
    echo "❌ Command failed with exit code: $EXIT_CODE"
    cat "$COMBINED_LOG"
fi
