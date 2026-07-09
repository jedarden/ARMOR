#!/bin/bash
# Integration test for Pluck output redirection configuration
# Demonstrates the complete logging setup with real Pluck execution

set -e

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BEAD_ID="integration-test"
TEST_LOG="$LOG_DIR/pluck-combined-${BEAD_ID}-${TIMESTAMP}.log"
TEST_SUMMARY="$LOG_DIR/pluck-test-summary-${TIMESTAMP}.txt"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[TEST]${NC} $*"
}

log_section() {
    echo -e "${BLUE}=== $* ===${NC}"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test files..."
    rm -f "$TEST_LOG" "$TEST_SUMMARY"
}

# Set trap for cleanup
trap cleanup EXIT

log_section "Pluck Output Redirection Integration Test"

log_info "Testing complete log redirection setup"
log_info "Workspace: $WORKSPACE"
log_info "Log Directory: $LOG_DIR"
echo ""

# Test 1: Configuration Script
log_section "Test 1: Configuration Script"
log_info "Running pluck-log-redirection.sh..."

if bash "$WORKSPACE/pluck-log-redirection.sh" -b "$BEAD_ID" -p minimal > /dev/null 2>&1; then
    log_info "✓ Configuration script executed successfully"
else
    echo "✗ Configuration script failed"
    exit 1
fi

# Test 2: Log Rotation
log_section "Test 2: Log Rotation"
log_info "Running log rotation..."

if bash "$LOG_DIR/log-rotation-config.sh" > /dev/null 2>&1; then
    log_info "✓ Log rotation executed successfully"
else
    echo "✗ Log rotation failed"
    exit 1
fi

# Test 3: Real Pluck Execution with Logging
log_section "Test 3: Real Pluck Execution with Logging"
log_info "Running Pluck command with output redirection to log file..."

export RUST_LOG="needle::strand::pluck=info"

timeout 3s needle run -w "$WORKSPACE" -c 1 > "$TEST_LOG" 2>&1 || true

if [[ -f "$TEST_LOG" && -s "$TEST_LOG" ]]; then
    local log_lines=$(wc -l < "$TEST_LOG")
    local log_size=$(stat -c%s "$TEST_LOG" 2>/dev/null || stat -f%z "$TEST_LOG" 2>/dev/null)
    log_info "✓ Pluck execution logged successfully"
    log_info "  Log file: $TEST_LOG"
    log_info "  Lines: $log_lines"
    log_info "  Size: $log_size bytes"
else
    echo "✗ Pluck execution logging failed"
    exit 1
fi

# Test 4: Log Content Verification
log_section "Test 4: Log Content Verification"

local tests_passed=0
local tests_total=4

# Check for NEEDLE worker boot messages
if grep -q "NEEDLE worker boot" "$TEST_LOG"; then
    log_info "✓ Found NEEDLE worker boot messages"
    ((tests_passed++))
else
    echo "✗ Missing NEEDLE worker boot messages"
fi

# Check for DEBUG/INFO log levels
if grep -qE "DEBUG|INFO" "$TEST_LOG"; then
    log_info "✓ Found log level messages"
    ((tests_passed++))
else
    echo "✗ Missing log level messages"
fi

# Check for telemetry events
if grep -q "telemetry" "$TEST_LOG"; then
    log_info "✓ Found telemetry event messages"
    ((tests_passed++))
else
    echo "✗ Missing telemetry event messages"
fi

# Check for timestamp format
if grep -qE "^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}" "$TEST_LOG"; then
    log_info "✓ Found ISO timestamp format"
    ((tests_passed++))
else
    echo "✗ Missing ISO timestamp format"
fi

# Generate test summary
log_section "Test Summary"

{
    echo "Pluck Output Redirection Integration Test Results"
    echo "=================================================="
    echo "Date: $(date)"
    echo "Workspace: $WORKSPACE"
    echo "Log Directory: $LOG_DIR"
    echo ""
    echo "Test Results:"
    echo "  Test 1 (Configuration Script): PASSED"
    echo "  Test 2 (Log Rotation): PASSED"
    echo "  Test 3 (Pluck Execution Logging): PASSED"
    echo "  Test 4 (Content Verification): $tests_passed/$tests_total checks passed"
    echo ""
    echo "Log File Analysis:"
    echo "  File: $TEST_LOG"
    echo "  Lines: $(wc -l < "$TEST_LOG")"
    echo "  Size: $(stat -c%s "$TEST_LOG" 2>/dev/null || stat -f%z "$TEST_LOG" 2>/dev/null) bytes"
    echo ""
    echo "Sample Log Content:"
    echo "----------------------------------------"
    head -5 "$TEST_LOG"
    echo "----------------------------------------"
} | tee "$TEST_SUMMARY"

echo ""

if [[ $tests_passed -eq $tests_total ]]; then
    log_section "Integration Test: PASSED"
    log_info "All acceptance criteria met:"
    log_info "✓ Log file location created and verified"
    log_info "✓ Output redirection syntax validated"
    log_info "✓ Sample command successfully wrote to log file"
    log_info "✓ Log rotation configured for long-running processes"
    exit 0
else
    log_section "Integration Test: FAILED"
    log_info "Some content verification checks failed"
    exit 1
fi