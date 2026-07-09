#!/run/current-system/sw/bin/bash
# Pluck execution with debug logging for bead bf-2ux9
# Executes Pluck command with comprehensive debug output capture

set -e

# Configuration
BEAD_ID="bf-2ux9"
WORKSPACE="/home/coding/ARMOR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="$WORKSPACE/logs/pluck-debug"
STDOUT_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-capture-${TIMESTAMP}.log"
STDERR_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-stderr-${TIMESTAMP}.log"
SUMMARY_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-summary-${TIMESTAMP}.log"
COMBINED_LOG="$LOG_DIR/pluck-combined-${BEAD_ID}-${TIMESTAMP}.log"

# Create log directory
mkdir -p "$LOG_DIR"

echo "=== Pluck Execution with Debug Logging for ${BEAD_ID} ==="
echo "Timestamp: $(date)"
echo "Log directory: $LOG_DIR"
echo "Stdout log: $STDOUT_LOG"
echo "Stderr log: $STDERR_LOG"
echo "Summary log: $SUMMARY_LOG"
echo "Combined log: $COMBINED_LOG"
echo ""

# Set comprehensive debug logging (full debug level)
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

echo "RUST_LOG configuration:"
echo "$RUST_LOG"
echo ""

# Execute NEEDLE with output capture
echo "=== Executing NEEDLE with Pluck debug logging ==="
echo "Starting: $(date)"
echo ""

# Run NEEDLE with stdout/stderr separation and timeout
timeout 180s needle run -w "$WORKSPACE" -c 1 \
  > >(tee -a "$STDOUT_LOG") \
  2> >(tee -a "$STDERR_LOG" >&2) || {
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 124 ]; then
        echo ""
        echo "⏰ Execution timed out after 180 seconds (expected for long-running agent execution)"
    else
        echo ""
        echo "⚠️  Execution completed with exit code: $EXIT_CODE"
    fi
}

echo ""
echo "=== Execution Completed ==="
echo "Finished: $(date)"
echo ""

# Combine stdout and stderr for analysis
echo "=== Combining logs ===" > "$COMBINED_LOG"
echo "Bead ID: $BEAD_ID" >> "$COMBINED_LOG"
echo "Timestamp: $TIMESTAMP" >> "$COMBINED_LOG"
echo "" >> "$COMBINED_LOG"

if [[ -f "$STDOUT_LOG" ]]; then
    echo "=== STDOUT ===" >> "$COMBINED_LOG"
    cat "$STDOUT_LOG" >> "$COMBINED_LOG"
fi

if [[ -f "$STDERR_LOG" ]]; then
    echo "" >> "$COMBINED_LOG"
    echo "=== STDERR ===" >> "$COMBINED_LOG"
    cat "$STDERR_LOG" >> "$COMBINED_LOG"
fi

# Generate summary
echo "=== Execution Summary ===" | tee -a "$SUMMARY_LOG"
echo "Bead ID: $BEAD_ID" | tee -a "$SUMMARY_LOG"
echo "Timestamp: $TIMESTAMP" | tee -a "$SUMMARY_LOG"
echo "Log directory: $LOG_DIR" | tee -a "$SUMMARY_LOG"
echo "" | tee -a "$SUMMARY_LOG"

# File statistics
echo "📊 File Statistics:" | tee -a "$SUMMARY_LOG"
if [[ -f "$STDOUT_LOG" ]]; then
    STDOUT_SIZE=$(stat -f%z "$STDOUT_LOG" 2>/dev/null || stat -c%s "$STDOUT_LOG" 2>/dev/null || echo "0")
    STDOUT_LINES=$(wc -l < "$STDOUT_LOG" 2>/dev/null || echo "0")
    echo "  Stdout: $STDOUT_LOG" | tee -a "$SUMMARY_LOG"
    echo "    Size: $STDOUT_SIZE bytes" | tee -a "$SUMMARY_LOG"
    echo "    Lines: $STDOUT_LINES" | tee -a "$SUMMARY_LOG"
fi

if [[ -f "$STDERR_LOG" ]]; then
    STDERR_SIZE=$(stat -f%z "$STDERR_LOG" 2>/dev/null || stat -c%s "$STDERR_LOG" 2>/dev/null || echo "0")
    STDERR_LINES=$(wc -l < "$STDERR_LOG" 2>/dev/null || echo "0")
    echo "  Stderr: $STDERR_LOG" | tee -a "$SUMMARY_LOG"
    echo "    Size: $STDERR_SIZE bytes" | tee -a "$SUMMARY_LOG"
    echo "    Lines: $STDERR_LINES" | tee -a "$SUMMARY_LOG"
fi

if [[ -f "$COMBINED_LOG" ]]; then
    COMBINED_SIZE=$(stat -f%z "$COMBINED_LOG" 2>/dev/null || stat -c%s "$COMBINED_LOG" 2>/dev/null || echo "0")
    COMBINED_LINES=$(wc -l < "$COMBINED_LOG" 2>/dev/null || echo "0")
    echo "  Combined: $COMBINED_LOG" | tee -a "$SUMMARY_LOG"
    echo "    Size: $COMBINED_SIZE bytes" | tee -a "$SUMMARY_LOG"
    echo "    Lines: $COMBINED_LINES" | tee -a "$SUMMARY_LOG"
fi

echo "" | tee -a "$SUMMARY_LOG"

# Error analysis
echo "🚨 Error Analysis:" | tee -a "$SUMMARY_LOG"
if [[ -f "$STDERR_LOG" && -s "$STDERR_LOG" ]]; then
    ERROR_COUNT=$(grep -ci "error" "$STDERR_LOG" 2>/dev/null || echo "0")
    WARN_COUNT=$(grep -ci "warn" "$STDERR_LOG" 2>/dev/null || echo "0")

    echo "  Errors: $ERROR_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Warnings: $WARN_COUNT" | tee -a "$SUMMARY_LOG"
else
    echo "  No stderr output - clean execution!" | tee -a "$SUMMARY_LOG"
fi

echo "" | tee -a "$SUMMARY_LOG"

# Progress indicator analysis
echo "🔄 Progress Indicators Found:" | tee -a "$SUMMARY_LOG"
if [[ -f "$STDOUT_LOG" && -s "$STDOUT_LOG" ]]; then
    PLUCK_COUNT=$(grep -ci "pluck" "$STDOUT_LOG" 2>/dev/null || echo "0")
    FILTER_COUNT=$(grep -ci "filter" "$STDOUT_LOG" 2>/dev/null || echo "0")
    CANDIDATE_COUNT=$(grep -ci "candidate" "$STDOUT_LOG" 2>/dev/null || echo "0")
    STRAND_COUNT=$(grep -ci "strand" "$STDOUT_LOG" 2>/dev/null || echo "0")
    BEAD_COUNT=$(grep -ci "bead" "$STDOUT_LOG" 2>/dev/null || echo "0")

    echo "  Pluck mentions: $PLUCK_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Filter mentions: $FILTER_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Candidate mentions: $CANDIDATE_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Strand mentions: $STRAND_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Bead mentions: $BEAD_COUNT" | tee -a "$SUMMARY_LOG"
else
    echo "  No stdout output available" | tee -a "$SUMMARY_LOG"
fi

echo "" | tee -a "$SUMMARY_LOG"
echo "=== Execution Complete ===" | tee -a "$SUMMARY_LOG"

echo ""
echo "✅ Pluck debug execution completed!"
echo "📊 Check $SUMMARY_LOG for comprehensive analysis"
echo "📋 Full logs:"
echo "   - Stdout: $STDOUT_LOG"
echo "   - Stderr: $STDERR_LOG"
echo "   - Combined: $COMBINED_LOG"
