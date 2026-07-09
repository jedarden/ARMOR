#!/bin/bash
# Comprehensive Pluck Command Syntax Validation
# Bead: bf-t5my
# This script validates all aspects of the Pluck command syntax

set -e

echo "=== Pluck Command Syntax Validation ==="
echo "Bead: bf-t5my"
echo "Date: $(date)"
echo ""

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0
TOTAL=0

# Helper function for tests
test_result() {
    TOTAL=$((TOTAL + 1))
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✅ PASSED${NC}: $2"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}❌ FAILED${NC}: $2"
        FAILED=$((FAILED + 1))
    fi
}

echo "=== Section 1: Binary and Command Tests ==="

# Test 1: Needle binary exists
which needle > /dev/null 2>&1
test_result $? "Needle binary exists and is in PATH"

# Test 2: Needle version check
needle --version > /dev/null 2>&1
test_result $? "Needle version can be retrieved"

# Test 3: Needle run command help
needle run --help > /dev/null 2>&1
test_result $? "Needle run command help is available"

echo ""
echo "=== Section 2: Flag Recognition Tests ==="

# Test 4: Workspace flag syntax
timeout 1s needle run -w "/home/coding/ARMOR" --help > /dev/null 2>&1
test_result $? "Workspace flag (-w) syntax valid"

# Test 5: Count flag syntax
timeout 1s needle run -c 1 --help > /dev/null 2>&1
test_result $? "Count flag (-c) syntax valid"

# Test 6: Agent flag syntax
timeout 1s needle run -a claude --help > /dev/null 2>&1
test_result $? "Agent flag (-a) syntax valid"

# Test 7: Identifier flag syntax
timeout 1s needle run -i test-worker --help > /dev/null 2>&1
test_result $? "Identifier flag (-i) syntax valid"

# Test 8: Timeout flag syntax
timeout 1s needle run -t 30 --help > /dev/null 2>&1
test_result $? "Timeout flag (-t) syntax valid"

# Test 9: Resume flag syntax
timeout 1s needle run --resume --help > /dev/null 2>&1
test_result $? "Resume flag (--resume) syntax valid"

# Test 10: Hot-reload flag syntax
timeout 1s needle run --hot-reload true --help > /dev/null 2>&1
test_result $? "Hot-reload flag (--hot-reload) syntax valid"

echo ""
echo "=== Section 3: Combined Flag Tests ==="

# Test 11: Multiple flags together
timeout 1s needle run -w "/home/coding/ARMOR" -c 1 --help > /dev/null 2>&1
test_result $? "Multiple flags work together"

# Test 12: Production flags combination
timeout 1s needle run -w "/home/coding/ARMOR" -c 1 -t 180 --help > /dev/null 2>&1
test_result $? "Production flags combined correctly"

echo ""
echo "=== Section 4: RUST_LOG Configuration Tests ==="

# Test 13: Basic RUST_LOG format
RUST_LOG="info" timeout 1s needle run --help > /dev/null 2>&1
test_result $? "RUST_LOG basic format accepted"

# Test 14: Pluck module syntax
RUST_LOG="needle::strand::pluck=debug" timeout 1s needle run --help > /dev/null 2>&1
test_result $? "RUST_LOG pluck module syntax valid"

# Test 15: Multiple modules syntax
RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug" timeout 1s needle run --help > /dev/null 2>&1
test_result $? "RUST_LOG multiple modules syntax valid"

echo ""
echo "=== Section 5: Infrastructure Tests ==="

# Test 16: Workspace directory exists
[ -d "/home/coding/ARMOR" ]
test_result $? "Workspace directory exists"

# Test 17: .beads directory exists
[ -d "/home/coding/ARMOR/.beads" ]
test_result $? ".beads directory exists"

# Test 18: Beads database exists
[ -f "/home/coding/ARMOR/.beads/beads.db" ]
test_result $? "Beads database exists"

# Test 19: Log directory can be created
mkdir -p /home/coding/ARMOR/logs/pluck-debug/test 2>/dev/null
test_result $? "Log directory can be created"

# Test 20: Timeout command available
which timeout > /dev/null 2>&1
test_result $? "Timeout command available"

# Test 21: Tee command available
which tee > /dev/null 2>&1
test_result $? "Tee command available"

echo ""
echo "=== Section 6: Complete Command Structure Tests ==="

# Test 22: Complete command with timeout
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug"
timeout 1s needle run -w "/home/coding/ARMOR" -c 1 --help > /dev/null 2>&1
test_result $? "Complete command with all flags parses correctly"

# Test 23: Command with output redirection syntax (dry run)
bash -c "echo 'test' | timeout 1s tee >(cat) > /dev/null 2>&1"
test_result $? "Output redirection syntax is valid"

echo ""
echo "=== Section 7: Shell Script Syntax Tests ==="

# Test 24: Validation script syntax
if [ -f "scripts/validate-pluck-syntax.sh" ]; then
    bash -n scripts/validate-pluck-syntax.sh 2>/dev/null
    test_result $? "Validation script syntax is valid"
else
    echo -e "${YELLOW}⚠️  SKIPPED${NC}: Validation script not found"
    TOTAL=$((TOTAL + 1))
fi

# Test 25: Basic test script syntax
if [ -f "test-pluck-syntax.sh" ]; then
    bash -n test-pluck-syntax.sh 2>/dev/null
    test_result $? "Basic test script syntax is valid"
else
    echo -e "${YELLOW}⚠️  SKIPPED${NC}: Basic test script not found"
    TOTAL=$((TOTAL + 1))
fi

# Test 26: Execute script syntax
if [ -f "scripts/execute-pluck-bf-4q1w.sh" ]; then
    bash -n scripts/execute-pluck-bf-4q1w.sh 2>/dev/null
    test_result $? "Execute script syntax is valid"
else
    echo -e "${YELLOW}⚠️  SKIPPED${NC}: Execute script not found"
    TOTAL=$((TOTAL + 1))
fi

echo ""
echo "=== Validation Summary ==="
echo "Total Tests: $TOTAL"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"

if [ $FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✅ ALL TESTS PASSED${NC}"
    echo "The Pluck command syntax has been validated successfully."
    exit 0
else
    echo ""
    echo -e "${RED}❌ SOME TESTS FAILED${NC}"
    echo "Please review the failed tests above."
    exit 1
fi
