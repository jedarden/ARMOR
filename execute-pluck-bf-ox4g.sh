#!/run/current-system/sw/bin/bash
# Script to execute Pluck with debug logging for bead bf-ox4g
set -e

OUTPUT_FILE="logs/pluck-debug/pluck-debug-bf-ox4g-capture-$(date +%Y%m%d-%H%M%S).log"

echo "=== Pluck Debug Execution Capture for bf-ox4g ==="
echo "Output file: $OUTPUT_FILE"
echo "Timestamp: $(date)"
echo ""

# Set comprehensive debug logging for Pluck
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

echo "RUST_LOG configuration:"
echo "$RUST_LOG"
echo ""

# Execute NEEDLE with timeout and capture output
echo "Executing NEEDLE..."
timeout 180s needle run -w /home/coding/ARMOR -c 1 2>&1 | tee "$OUTPUT_FILE" || {
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 124 ]; then
        echo ""
        echo "Execution timed out after 180 seconds (expected for long-running agent execution)"
        echo "Output captured to: $OUTPUT_FILE"
    else
        echo ""
        echo "Execution completed with exit code: $EXIT_CODE"
        echo "Output captured to: $OUTPUT_FILE"
    fi
}

echo ""
echo "=== Capture Summary ==="
echo "Log file: $OUTPUT_FILE"
echo "File size: $(wc -c < "$OUTPUT_FILE") bytes"
echo "Line count: $(wc -l < "$OUTPUT_FILE") lines"
echo ""

# Search for Pluck-specific output
echo "=== Pluck Output Analysis ==="
echo "Lines containing 'pluck': $(grep -ci 'pluck' "$OUTPUT_FILE" || echo '0')"
echo "Lines containing 'filter': $(grep -ci 'filter' "$OUTPUT_FILE" || echo '0')"
echo "Lines containing 'candidate': $(grep -ci 'candidate' "$OUTPUT_FILE" || echo '0')"
echo "Lines containing 'strand': $(grep -ci 'strand' "$OUTPUT_FILE" || echo '0')"
echo ""

echo "Capture complete!"
