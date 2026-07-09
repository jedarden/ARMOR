#!/run/current-system/sw/bin/bash
# Output redirection template 1: Separate stdout and stderr files

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
STDOUT_LOG="$LOG_DIR/pluck-execution-stdout-${TIMESTAMP}.log"
STDERR_LOG="$LOG_DIR/pluck-execution-stderr-${TIMESTAMP}.log"

# Execute command with separated output
your_command_here > "$STDOUT_LOG" 2> "$STDERR_LOG"

# Check exit status
EXIT_CODE=$?
if [ $EXIT_CODE -eq 0 ]; then
    echo "✅ Command completed successfully"
else
    echo "❌ Command failed with exit code: $EXIT_CODE"
fi
