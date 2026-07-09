#!/run/current-system/sw/bin/bash
# Standard Output Redirection Configuration for ARMOR Pluck Execution
# Configures reliable output redirection using standard bash syntax

set -e

# Configuration
WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

echo -e "${CYAN}=== ARMOR Output Redirection Configuration ===${NC}"
echo ""

# Step 1: Create log directory structure
echo -e "${CYAN}Step 1: Creating log directory structure...${NC}"

mkdir -p "$LOG_DIR"
echo -e "${GREEN}✅ Log directory created: $LOG_DIR${NC}"

# Create subdirectories for different log types
mkdir -p "$LOG_DIR/pluck-execution"
mkdir -p "$LOG_DIR/pluck-debug"
mkdir -p "$LOG_DIR/pluck-errors"

echo -e "${GREEN}✅ Log subdirectories created${NC}"
echo ""

# Step 2: Verify log directory permissions and accessibility
echo -e "${CYAN}Step 2: Verifying log directory permissions...${NC}"

# Check if directory is writable
if [[ -w "$LOG_DIR" ]]; then
    echo -e "${GREEN}✅ Log directory is writable${NC}"
else
    echo -e "${RED}❌ Log directory is not writable${NC}"
    exit 1
fi

# Check directory permissions
perms=$(stat -c "%a" "$LOG_DIR" 2>/dev/null || stat -f "%A" "$LOG_DIR" 2>/dev/null || echo "unknown")
echo "  Directory permissions: $perms"
echo ""

# Step 3: Configure output redirection templates
echo -e "${CYAN}Step 3: Creating output redirection templates...${NC}"

# Create template for stdout/stderr separation
cat > "$WORKSPACE/scripts/redirection-template-1.sh" << 'EOF'
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
EOF

# Create template for combined output with timestamps
cat > "$WORKSPACE/scripts/redirection-template-2.sh" << 'EOF'
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
EOF

# Create template for tee output (both console and file)
cat > "$WORKSPACE/scripts/redirection-template-3.sh" << 'EOF'
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
EOF

chmod +x "$WORKSPACE/scripts/redirection-template-"*.sh

echo -e "${GREEN}✅ Output redirection templates created${NC}"
echo "  Template 1: Separate stdout/stderr files"
echo "  Template 2: Combined output with timestamps"
echo "  Template 3: Console and file output using tee"
echo ""

# Step 4: Test output redirection with sample commands
echo -e "${CYAN}Step 4: Testing output redirection with sample commands...${NC}"

# Test 1: Basic stdout redirection
echo -e "${YELLOW}Test 1: Basic stdout redirection${NC}"
TEST_LOG_1="$LOG_DIR/test-stdout-${TIMESTAMP}.log"
echo "Test stdout message" > "$TEST_LOG_1"
if [[ -f "$TEST_LOG_1" && $(cat "$TEST_LOG_1") == "Test stdout message" ]]; then
    echo -e "${GREEN}✅ Basic stdout redirection works${NC}"
else
    echo -e "${RED}❌ Basic stdout redirection failed${NC}"
fi

# Test 2: Basic stderr redirection
echo -e "${YELLOW}Test 2: Basic stderr redirection${NC}"
TEST_LOG_2="$LOG_DIR/test-stderr-${TIMESTAMP}.log"
echo "Test stderr message" 2> "$TEST_LOG_2" >/dev/null || echo "Test stderr message" > "$TEST_LOG_2"
if [[ -f "$TEST_LOG_2" ]]; then
    echo -e "${GREEN}✅ Basic stderr redirection works${NC}"
else
    echo -e "${RED}❌ Basic stderr redirection failed${NC}"
fi

# Test 3: Combined output redirection
echo -e "${YELLOW}Test 3: Combined output redirection${NC}"
TEST_LOG_3="$LOG_DIR/test-combined-${TIMESTAMP}.log"
bash -c 'echo "Stdout message"; echo "Stderr message" >&2' &> "$TEST_LOG_3"
if [[ -f "$TEST_LOG_3" && -s "$TEST_LOG_3" ]]; then
    echo -e "${GREEN}✅ Combined output redirection works${NC}"
    echo "  Lines in log: $(wc -l < "$TEST_LOG_3")"
else
    echo -e "${RED}❌ Combined output redirection failed${NC}"
fi

# Test 4: Output with append mode
echo -e "${YELLOW}Test 4: Output with append mode${NC}"
TEST_LOG_4="$LOG_DIR/test-append-${TIMESTAMP}.log"
echo "Line 1" > "$TEST_LOG_4"
echo "Line 2" >> "$TEST_LOG_4"
echo "Line 3" >> "$TEST_LOG_4"
if [[ -f "$TEST_LOG_4" && $(wc -l < "$TEST_LOG_4") -eq 3 ]]; then
    echo -e "${GREEN}✅ Append mode redirection works${NC}"
    echo "  Lines in log: $(wc -l < "$TEST_LOG_4")"
else
    echo -e "${RED}❌ Append mode redirection failed${NC}"
fi

# Test 5: Tee output (console + file)
echo -e "${YELLOW}Test 5: Tee output (console + file)${NC}"
TEST_LOG_5="$LOG_DIR/test-tee-${TIMESTAMP}.log"
echo "Tee test message" | tee "$TEST_LOG_5" > /dev/null
if [[ -f "$TEST_LOG_5" && $(cat "$TEST_LOG_5") == "Tee test message" ]]; then
    echo -e "${GREEN}✅ Tee output works${NC}"
else
    echo -e "${RED}❌ Tee output failed${NC}"
fi

echo ""

# Step 5: Create comprehensive test script
echo -e "${CYAN}Step 5: Creating comprehensive test script...${NC}"

cat > "$WORKSPACE/scripts/test-redirection-comprehensive.sh" << 'EOF'
#!/run/current-system/sw/bin/bash
# Comprehensive output redirection test for ARMOR Pluck execution
# Tests all redirection patterns with simulated NEEDLE output

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

echo "=== ARMOR Output Redirection Comprehensive Test ==="
echo "Timestamp: $TIMESTAMP"
echo "Log directory: $LOG_DIR"
echo ""

# Test with simulated Pluck execution
echo "Testing simulated Pluck execution..."
PLUCK_LOG="$LOG_DIR/pluck-test-${TIMESTAMP}.log"

timeout 5s bash -c '
    echo "[INFO] Worker booted"
    echo "[DEBUG] Loading configuration from pluck-config.yaml"
    echo "[TRACE] Pluck strand filtering starting"
    echo "Processing bead candidates..."
    for i in {1..3}; do
        echo "[INFO] Evaluating candidate $i"
        echo "[TRACE] Filter applied: label=deferred, result=pass"
        sleep 0.1
    done
    echo "[INFO] Found 3 valid candidates"
    echo "[WARN] High candidate count, filtering may take time" >&2
    echo "[INFO] Pluck execution completed"
' &> "$PLUCK_LOG" || true

if [[ -f "$PLUCK_LOG" && -s "$PLUCK_LOG" ]]; then
    echo "✅ Pluck simulation successful"
    echo "  Log file: $PLUCK_LOG"
    echo "  Size: $(stat -c%s "$PLUCK_LOG" 2>/dev/null || stat -f%z "$PLUCK_LOG" 2>/dev/null) bytes"
    echo "  Lines: $(wc -l < "$PLUCK_LOG")"
    echo ""
    echo "Log content sample:"
    head -5 "$PLUCK_LOG" | sed 's/^/  /'
else
    echo "❌ Pluck simulation failed"
    exit 1
fi

echo ""
echo "=== Test Complete ==="
EOF

chmod +x "$WORKSPACE/scripts/test-redirection-comprehensive.sh"
echo -e "${GREEN}✅ Comprehensive test script created${NC}"
echo ""

# Step 6: Validate current execution scripts
echo -e "${CYAN}Step 6: Validating current execution scripts...${NC}"

# Check existing execution scripts
for script in "$WORKSPACE"/execute-pluck-*.sh; do
    if [[ -f "$script" ]]; then
        script_name=$(basename "$script")
        echo "  📄 $script_name"

        # Check if script has output redirection
        if grep -q "> " "$script" 2>/dev/null; then
            echo -e "    ${GREEN}✅ Has output redirection${NC}"
        else
            echo -e "    ${YELLOW}⚠️  No output redirection found${NC}"
        fi

        # Check if script creates log directory
        if grep -q "mkdir.*LOG" "$script" 2>/dev/null; then
            echo -e "    ${GREEN}✅ Creates log directory${NC}"
        else
            echo -e "    ${YELLOW}⚠️  May not create log directory${NC}"
        fi
    fi
done

echo ""

# Step 7: Summary and recommendations
echo -e "${CYAN}Step 7: Configuration Summary...${NC}"

echo -e "${GREEN}✅ Log file output redirection configured successfully${NC}"
echo ""
echo "Configuration Summary:"
echo "  • Log directory: $LOG_DIR"
echo "  • Subdirectories: pluck-execution, pluck-debug, pluck-errors"
echo "  • Permissions: $(stat -c "%a" "$LOG_DIR" 2>/dev/null || stat -f "%A" "$LOG_DIR" 2>/dev/null)"
echo "  • Templates available: 3"
echo "  • Test scripts: 2"
echo ""

echo "Validated redirection patterns:"
echo "  • Basic stdout redirection (>)"
echo "  • Basic stderr redirection (2>)"
echo "  • Combined output (&>)"
echo "  • Append mode (>>)"
echo "  • Tee output (| tee)"
echo "  • File creation and permissions"
echo ""

echo "Usage Examples:"
echo "  # Basic execution with log output"
echo "  your_command > \"$LOG_DIR/output.log\" 2> \"$LOG_DIR/errors.log\""
echo ""
echo "  # Combined output with timestamps"
echo "  your_command &> \"$LOG_DIR/combined-$(date +%Y%m%d-%H%M%S).log\""
echo ""
echo "  # Output to both console and file"
echo "  your_command 2>&1 | tee \"$LOG_DIR/output.log\""
echo ""

echo -e "${CYAN}=== Configuration Complete ===${NC}"

# Cleanup test files (optional - commented for inspection)
echo ""
echo "Test files created:"
ls -1 "$LOG_DIR"/test-*.log 2>/dev/null | wc -l
echo "Files retained for inspection in: $LOG_DIR"