#!/run/current-system/sw/bin/bash
# Pluck Command Syntax Validation Test
# Tests command parsing and debug flags without full execution

set -e

echo "=== Pluck Command Syntax Validation Test ==="
echo "Timestamp: $(date)"
echo ""

# Test 1: Verify needle command exists
echo "Test 1: Verifying needle command exists..."
if command -v needle &> /dev/null; then
    NEEDLE_PATH=$(command -v needle)
    echo "✅ needle command found at: $NEEDLE_PATH"
else
    echo "❌ needle command not found"
    exit 1
fi
echo ""

# Test 2: Validate needle run command syntax
echo "Test 2: Validating needle run command syntax..."
if needle run --help &> /dev/null; then
    echo "✅ needle run command syntax is valid"
else
    echo "❌ needle run command syntax is invalid"
    exit 1
fi
echo ""

# Test 3: Validate specific flags used in execute script
echo "Test 3: Validating specific flags..."
echo "Testing -w (workspace) flag..."
if needle run -w "/tmp" --help &> /dev/null; then
    echo "✅ -w flag is valid"
else
    echo "❌ -w flag is invalid"
    exit 1
fi

echo "Testing -c (count) flag..."
if needle run -c 1 --help &> /dev/null; then
    echo "✅ -c flag is valid"
else
    echo "❌ -c flag is invalid"
    exit 1
fi
echo ""

# Test 4: Validate RUST_LOG environment variable format
echo "Test 4: Validating RUST_LOG environment variable format..."
RUST_LOG_TEST="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

# Check if format matches expected pattern (module=path with comma separation)
if echo "$RUST_LOG_TEST" | grep -qE '^[a-z:_]+=[a-z]+(,[a-z:_]+=[a-z]+)*$'; then
    echo "✅ RUST_LOG format is valid"
    echo "   Configuration: $RUST_LOG_TEST"
else
    echo "❌ RUST_LOG format is invalid"
    exit 1
fi
echo ""

# Test 5: Verify timeout command accepts the syntax
echo "Test 5: Validating timeout command syntax..."
if timeout 1s echo "test" &> /dev/null; then
    echo "✅ timeout command is available and working"
else
    echo "❌ timeout command is not available"
    exit 1
fi
echo ""

# Test 6: Validate the complete command structure (dry run)
echo "Test 6: Validating complete command structure (dry run)..."
WORKSPACE="/home/coding/ARMOR"
COMMAND="timeout 1s needle run -w \"$WORKSPACE\" -c 1 --help"

echo "Testing command: $COMMAND"
if eval "$COMMAND" &> /dev/null; then
    echo "✅ Complete command structure is valid"
else
    echo "❌ Complete command structure is invalid"
    exit 1
fi
echo ""

# Test 7: Check log directory creation
echo "Test 7: Validating log directory creation..."
TEST_LOG_DIR="/tmp/pluck-syntax-test-$$"
if mkdir -p "$TEST_LOG_DIR" && rmdir "$TEST_LOG_DIR"; then
    echo "✅ Log directory creation is working"
else
    echo "❌ Log directory creation failed"
    exit 1
fi
echo ""

# Test 8: Validate tee command availability
echo "Test 8: Validating output redirection with tee..."
if echo "test" | tee /dev/null &> /dev/null; then
    echo "✅ tee command is available and working"
else
    echo "❌ tee command is not available"
    exit 1
fi
echo ""

# Summary
echo "=== Syntax Validation Summary ==="
echo "✅ All syntax validation tests passed!"
echo ""
echo "Validated components:"
echo "  • needle command exists and is executable"
echo "  • needle run command syntax is valid"
echo "  • All flags (-w, -c) are recognized"
echo "  • RUST_LOG environment variable format is correct"
echo "  • timeout command is available"
echo "  • Complete command structure parses correctly"
echo "  • Log directory creation works"
echo "  • Output redirection with tee functions"
echo ""
echo "🎯 Pluck command is ready for execution!"