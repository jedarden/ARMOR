#!/run/current-system/sw/bin/bash
# Output redirection template 3: Output to both console and file using tee

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
COMBINED_LOG="$LOG_DIR/pluck-execution-tee-${TIMESTAMP}.log"

# Execute command with tee for simultaneous console and file output
{
    echo "=== Execution started at $(date) ==="
    your_command_here 2>&1
    EXIT_CODE=$?
    echo "=== Execution completed at $(date) with exit code: $EXIT_CODE ==="
} | tee "$COMBINED_LOG"

# Exit with same code as command
exit $EXIT_CODE
