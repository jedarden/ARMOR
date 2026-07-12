#!/run/current-system/sw/bin/bash
# Pluck execution with comprehensive debug monitoring for bead bf-3jus
# Task: Execute Pluck command with debug flags

set -e

# Configuration
BEAD_ID="bf-3jus"
WORKSPACE="/home/coding/ARMOR"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="$WORKSPACE/logs/pluck-debug"
STDOUT_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-stdout-${TIMESTAMP}.log"
STDERR_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-stderr-${TIMESTAMP}.log"
MONITOR_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-monitor-${TIMESTAMP}.log"
SUMMARY_LOG="$LOG_DIR/pluck-debug-${BEAD_ID}-summary-${TIMESTAMP}.log"
PROGRESS_FILE="$LOG_DIR/pluck-debug-${BEAD_ID}-progress-${TIMESTAMP}.txt"

# Create log directory
mkdir -p "$LOG_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "=== Pluck Execution with Debug Flags for ${BEAD_ID} ==="
echo "Timestamp: $(date)"
echo "Log directory: $LOG_DIR"
echo "Stdout log: $STDOUT_LOG"
echo "Stderr log: $STDERR_LOG"
echo "Monitor log: $MONITOR_LOG"
echo "Summary log: $SUMMARY_LOG"
echo "Progress file: $PROGRESS_FILE"
echo ""

# Set comprehensive debug logging for Pluck and NEEDLE components
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug,needle::agent=debug"

echo "RUST_LOG configuration:"
echo "$RUST_LOG"
echo ""

# Initialize progress tracking
echo "=== Progress Tracking ===" > "$PROGRESS_FILE"
echo "Started: $(date)" >> "$PROGRESS_FILE"
echo "" >> "$PROGRESS_FILE"

# Start monitoring function in background
monitor_progress() {
    local stdout_log="$1"
    local stderr_log="$2"
    local monitor_log="$3"
    local progress_file="$4"

    local last_stdout_size=0
    local last_stderr_size=0
    local check_count=0

    while true; do
        sleep 2

        # Check if files exist and have content
        if [[ -f "$stdout_log" ]]; then
            local current_stdout_size=$(stat -f%z "$stdout_log" 2>/dev/null || stat -c%s "$stdout_log" 2>/dev/null || echo "0")
            local current_stderr_size=$(stat -f%z "$stderr_log" 2>/dev/null || stat -c%s "$stderr_log" 2>/dev/null || echo "0")

            local stdout_growth=$((current_stdout_size - last_stdout_size))
            local stderr_growth=$((current_stderr_size - last_stderr_size))

            check_count=$((check_count + 1))

            local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

            # Log progress
            echo "[$timestamp] Check #$check_count - Stdout: ${current_stdout_size} bytes (+${stdout_growth}), Stderr: ${current_stderr_size} bytes (+${stderr_growth})" >> "$monitor_log"

            # Update progress file
            echo "Check #$check_count [$timestamp]: Stdout ${current_stdout_size}B (+${stdout_growth}B), Stderr ${current_stderr_size}B (+${stderr_growth}B)" >> "$progress_file"

            # Check for error patterns in stderr
            if [[ -f "$stderr_log" && -s "$stderr_log" ]]; then
                local error_count=$(grep -ci "error" "$stderr_log" 2>/dev/null || echo "0")
                local warn_count=$(grep -ci "warn" "$stderr_log" 2>/dev/null || echo "0")

                if [[ $error_count -gt 0 ]]; then
                    echo "[$timestamp] 🚨 ERRORS DETECTED: $error_count error(s) found" >> "$monitor_log"
                    echo "🚨 Check #$check_count: $error_count error(s) detected" >> "$progress_file"
                fi

                if [[ $warn_count -gt 0 ]]; then
                    echo "[$timestamp] ⚠️  WARNINGS DETECTED: $warn_count warning(s) found" >> "$monitor_log"
                    echo "⚠️  Check #$check_count: $warn_count warning(s) detected" >> "$progress_file"
                fi
            fi

            # Look for progress indicators in stdout
            if [[ -f "$stdout_log" && -s "$stdout_log" ]]; then
                local pluck_lines=$(grep -ci "pluck" "$stdout_log" 2>/dev/null || echo "0")
                local filter_lines=$(grep -ci "filter" "$stdout_log" 2>/dev/null || echo "0")
                local candidate_lines=$(grep -ci "candidate" "$stdout_log" 2>/dev/null || echo "0")

                if [[ $pluck_lines -gt 0 || $filter_lines -gt 0 || $candidate_lines -gt 0 ]]; then
                    echo "[$timestamp] 🔄 Progress: pluck:$pluck_lines, filter:$filter_lines, candidate:$candidate_lines" >> "$monitor_log"
                    echo "🔄 Activity #$check_count: pluck:$pluck_lines, filter:$filter_lines, candidate:$candidate_lines" >> "$progress_file"
                fi
            fi

            last_stdout_size=$current_stdout_size
            last_stderr_size=$current_stderr_size
        fi
    done
}

# Start monitoring in background
echo "Starting progress monitor..."
monitor_progress "$STDOUT_LOG" "$STDERR_LOG" "$MONITOR_LOG" "$PROGRESS_FILE" &
MONITOR_PID=$!
echo "Monitor PID: $MONITOR_PID"
echo ""

# Execute NEEDLE with output capture
echo "=== Executing NEEDLE Pluck with Debug Flags ==="
echo "Starting: $(date)"
echo ""

# Run NEEDLE with stdout/stderr separation and extended timeout
timeout 300s needle run -w "$WORKSPACE" -c 1 > >(tee -a "$STDOUT_LOG") 2> >(tee -a "$STDERR_LOG" >&2) || {
    EXIT_CODE=$?
    if [ $EXIT_CODE -eq 124 ]; then
        echo ""
        echo "⏰ Execution timed out after 300 seconds (may indicate long-running agent execution)"
    else
        echo ""
        echo "⚠️  Execution completed with exit code: $EXIT_CODE"
    fi
}

# Kill the monitor
kill $MONITOR_PID 2>/dev/null || true

echo ""
echo "=== Execution Completed ==="
echo "Finished: $(date)"
echo ""

# Generate comprehensive summary
echo "=== Comprehensive Execution Summary ===" | tee -a "$SUMMARY_LOG"
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

if [[ -f "$MONITOR_LOG" ]]; then
    MONITOR_LINES=$(wc -l < "$MONITOR_LOG" 2>/dev/null || echo "0")
    echo "  Monitor: $MONITOR_LOG" | tee -a "$SUMMARY_LOG"
    echo "    Checks: $MONITOR_LINES" | tee -a "$SUMMARY_LOG"
fi

echo "" | tee -a "$SUMMARY_LOG"

# Error and warning analysis
echo "🚨 Error Analysis:" | tee -a "$SUMMARY_LOG"
if [[ -f "$STDERR_LOG" && -s "$STDERR_LOG" ]]; then
    ERROR_COUNT=$(grep -ci "error" "$STDERR_LOG" 2>/dev/null || echo "0")
    WARN_COUNT=$(grep -ci "warn" "$STDERR_LOG" 2>/dev/null || echo "0")
    FATAL_COUNT=$(grep -ci "fatal" "$STDERR_LOG" 2>/dev/null || echo "0")
    PANIC_COUNT=$(grep -ci "panic" "$STDERR_LOG" 2>/dev/null || echo "0")

    echo "  Errors: $ERROR_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Warnings: $WARN_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Fatal: $FATAL_COUNT" | tee -a "$SUMMARY_LOG"
    echo "  Panic: $PANIC_COUNT" | tee -a "$SUMMARY_LOG"

    if [[ $ERROR_COUNT -gt 0 ]]; then
        echo "" | tee -a "$SUMMARY_LOG"
        echo "  Sample errors:" | tee -a "$SUMMARY_LOG"
        grep -i "error" "$STDERR_LOG" | head -5 | while IFS= read -r line; do
            echo "    - $line" | tee -a "$SUMMARY_LOG"
        done
    fi
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

# Critical status indicators
echo "📈 Critical Status:" | tee -a "$SUMMARY_LOG"
if [[ -f "$STDOUT_LOG" && -s "$STDOUT_LOG" ]]; then
    if grep -qi "worker booted" "$STDOUT_LOG"; then
        echo "  ✅ Worker successfully booted" | tee -a "$SUMMARY_LOG"
    else
        echo "  ⚠️  Worker boot status unclear" | tee -a "$SUMMARY_LOG"
    fi

    if grep -qi "claimed bead" "$STDOUT_LOG"; then
        CLAIMED_BEAD=$(grep -i "claimed bead" "$STDOUT_LOG" | head -1 || echo "")
        echo "  ✅ Bead claimed: $CLAIMED_BEAD" | tee -a "$SUMMARY_LOG"
    else
        echo "  ⚠️  No bead claim detected" | tee -a "$SUMMARY_LOG"
    fi

    if grep -qi "agent dispatched" "$STDOUT_LOG"; then
        echo "  ✅ Agent dispatched successfully" | tee -a "$SUMMARY_LOG"
    else
        echo "  ⚠️  Agent dispatch status unclear" | tee -a "$SUMMARY_LOG"
    fi
fi

echo "" | tee -a "$SUMMARY_LOG"
echo "=== Monitoring Complete ===" | tee -a "$SUMMARY_LOG"
echo "" | tee -a "$SUMMARY_LOG"

# Final status
echo "📋 Summary Report Generated: $SUMMARY_LOG" | tee -a "$SUMMARY_LOG"
echo "📋 Progress Tracking: $PROGRESS_FILE" | tee -a "$SUMMARY_LOG"
echo "📋 Monitor Log: $MONITOR_LOG" | tee -a "$SUMMARY_LOG"

echo ""
echo "✅ Pluck execution with debug flags completed successfully!"
echo "📊 Check $SUMMARY_LOG for comprehensive analysis"
