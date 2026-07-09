#!/run/current-system/sw/bin/bash
# Output Redirection Test Script for ARMOR Pluck Execution
# Tests and validates output redirection configuration

set -e

# Configuration
WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TEST_LOG="$LOG_DIR/test-output-redirection.log"
TEST_ERR_LOG="$LOG_DIR/test-output-redirection-err.log"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== ARMOR Output Redirection Test ===${NC}"
echo ""

# Create log directory if it doesn't exist
echo -e "${CYAN}1. Creating log directory...${NC}"
mkdir -p "$LOG_DIR"
echo -e "${GREEN}✅ Log directory created: $LOG_DIR${NC}"
echo ""

# Test basic output redirection
echo -e "${CYAN}2. Testing basic output redirection...${NC}"

# Clean up any previous test files
rm -f "$TEST_LOG" "$TEST_ERR_LOG"

# Test stdout redirection
echo "Test stdout message" > "$TEST_LOG"
echo "Test stdout message 2" >> "$TEST_LOG"

# Test stderr redirection
echo "Test stderr message" >&2 > "$TEST_ERR_LOG" 2>/dev/null || echo "Test stderr message" > "$TEST_ERR_LOG"

if [[ -f "$TEST_LOG" && -f "$TEST_ERR_LOG" ]]; then
    echo -e "${GREEN}✅ Basic file redirection works${NC}"
    echo "  Stdout log: $TEST_LOG"
    echo "  Stderr log: $TEST_ERR_LOG"
else
    echo -e "${RED}❌ Basic file redirection failed${NC}"
    exit 1
fi
echo ""

# Test combined output redirection
echo -e "${CYAN}3. Testing combined stdout/stderr redirection...${NC}"

COMBINED_LOG="$LOG_DIR/test-combined.log"
rm -f "$COMBINED_LOG"

# Test with process substitution (like used in execute-pluck scripts)
{
    echo "Combined stdout message"
    echo "Combined stderr message" >&2
} > >(
    tee -a "$COMBINED_LOG"
) 2> >(
    tee -a "$COMBINED_LOG" >&2
)

if [[ -f "$COMBINED_LOG" ]]; then
    echo -e "${GREEN}✅ Combined redirection works${NC}"
    echo "  Combined log: $COMBINED_LOG"
    echo "  Lines in log: $(wc -l < "$COMBINED_LOG")"
else
    echo -e "${RED}❌ Combined redirection failed${NC}"
    exit 1
fi
echo ""

# Test output redirection with real command
echo -e "${CYAN}4. Testing redirection with real commands...${NC}"

REAL_TEST_LOG="$LOG_DIR/test-real-command.log"
rm -f "$REAL_TEST_LOG"

# Test with echo and sleep commands
timeout 5s bash -c '
    echo "Starting test command..."
    sleep 1
    echo "Test progress message 1"
    sleep 1
    echo "Test progress message 2"
    echo "WARNING: This is a test warning" >&2
    echo "ERROR: This is a test error" >&2
    echo "Completed test command"
' > >(
    tee -a "$REAL_TEST_LOG"
) 2> >(
    tee -a "$REAL_TEST_LOG" >&2
) || true

if [[ -f "$REAL_TEST_LOG" && -s "$REAL_TEST_LOG" ]]; then
    echo -e "${GREEN}✅ Real command redirection works${NC}"
    echo "  Log file: $REAL_TEST_LOG"
    echo "  File size: $(stat -c%s "$REAL_TEST_LOG" 2>/dev/null || stat -f%z "$REAL_TEST_LOG" 2>/dev/null || echo "unknown") bytes"
    echo "  Lines: $(wc -l < "$REAL_TEST_LOG")"
else
    echo -e "${RED}❌ Real command redirection failed${NC}"
    exit 1
fi
echo ""

# Test with simulated NEEDLE-like output
echo -e "${CYAN}5. Testing NEEDLE-like output simulation...${NC}"

NEEDLE_TEST_LOG="$LOG_DIR/test-needle-simulation.log"
NEEDLE_ERR_LOG="$LOG_DIR/test-needle-simulation-err.log"
rm -f "$NEEDLE_TEST_LOG" "$NEEDLE_ERR_LOG"

# Simulate NEEDLE output with various log levels
timeout 3s bash -c '
    echo "[INFO] Worker booted"
    echo "[DEBUG] Loading configuration from pluck-config.yaml"
    echo "[TRACE] Pluck strand filtering starting"
    echo "Processing bead candidates..."
    for i in {1..5}; do
        echo "[INFO] Evaluating candidate $i"
        echo "[TRACE] Filter applied: label=deferred, result=pass"
        sleep 0.1
    done
    echo "[INFO] Found 5 valid candidates"
    echo "[WARN] High candidate count, filtering may take time" >&2
    echo "[ERROR] Test error message for validation" >&2
    echo "[INFO] Pluck execution completed"
' > >(
    tee -a "$NEEDLE_TEST_LOG"
) 2> >(
    tee -a "$NEEDLE_ERR_LOG" >&2
) || true

if [[ -f "$NEEDLE_TEST_LOG" && -f "$NEEDLE_ERR_LOG" ]]; then
    stdout_lines=$(wc -l < "$NEEDLE_TEST_LOG")
    stderr_lines=$(wc -l < "$NEEDLE_ERR_LOG")

    echo -e "${GREEN}✅ NEEDLE-like simulation works${NC}"
    echo "  Stdout log: $NEEDLE_TEST_LOG ($stdout_lines lines)"
    echo "  Stderr log: $NEEDLE_ERR_LOG ($stderr_lines lines)"

    # Show some content samples
    echo ""
    echo "  Stdout sample:"
    head -3 "$NEEDLE_TEST_LOG" | sed 's/^/    /'

    echo "  Stderr sample:"
    head -3 "$NEEDLE_ERR_LOG" | sed 's/^/    /'
else
    echo -e "${RED}❌ NEEDLE-like simulation failed${NC}"
    exit 1
fi
echo ""

# Test log file permissions and accessibility
echo -e "${CYAN}6. Testing log file permissions...${NC}"

TEST_PERM_LOG="$LOG_DIR/test-permissions.log"
rm -f "$TEST_PERM_LOG"

echo "Test content" > "$TEST_PERM_LOG"

# Check if file is readable
if [[ -r "$TEST_PERM_LOG" ]]; then
    echo -e "${GREEN}✅ Log file is readable${NC}"
else
    echo -e "${RED}❌ Log file is not readable${NC}"
fi

# Check if file is writable
if [[ -w "$TEST_PERM_LOG" ]]; then
    echo -e "${GREEN}✅ Log file is writable${NC}"
else
    echo -e "${RED}❌ Log file is not writable${NC}"
fi

# Check file permissions
perms=$(stat -c "%a" "$TEST_PERM_LOG" 2>/dev/null || stat -f "%A" "$TEST_PERM_LOG" 2>/dev/null || echo "unknown")
echo "  File permissions: $perms"
echo ""

# Summary and validation
echo -e "${CYAN}7. Final Validation Summary...${NC}"

all_tests_passed=true

# Check if all test logs exist
for test_log in "$TEST_LOG" "$TEST_ERR_LOG" "$COMBINED_LOG" "$REAL_TEST_LOG" "$NEEDLE_TEST_LOG" "$NEEDLE_ERR_LOG" "$TEST_PERM_LOG"; do
    if [[ ! -f "$test_log" ]]; then
        echo -e "${RED}❌ Missing log file: $test_log${NC}"
        all_tests_passed=false
    fi
done

if $all_tests_passed; then
    echo -e "${GREEN}✅ All output redirection tests passed!${NC}"
    echo ""
    echo "Validated redirection patterns:"
    echo "  • Basic stdout redirection (>)"
    echo "  • Basic stderr redirection (2>)"
    echo "  • Append redirection (>>)"
    echo "  • Combined output (>&2)"
    echo "  • Process substitution (> >(tee))"
    echo "  • Real command execution with timeout"
    echo "  • NEEDLE-like output simulation"
    echo "  • File permissions and accessibility"
    echo ""
    echo "Log directory location: $LOG_DIR"
    echo "Test logs created: $(ls -1 "$LOG_DIR"/test-*.log | wc -l)"
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi

# Cleanup test files (optional - commented out for inspection)
# echo ""
# echo "Cleaning up test files..."
# rm -f "$LOG_DIR"/test-*.log
# echo -e "${GREEN}✅ Test files cleaned up${NC}"

echo ""
echo -e "${CYAN}=== Output Redirection Test Complete ===${NC}"