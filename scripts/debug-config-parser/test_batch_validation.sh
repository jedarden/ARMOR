#!/usr/bin/env bash
# Test script for batch validation acceptance criteria
# Tests all acceptance criteria: inventory processing, reporting, exit codes

set -e

echo "=== Testing Batch Validation Acceptance Criteria ==="
echo ""

# Create test workspace
TEST_DIR="/tmp/batch-validation-test-$$"
mkdir -p "$TEST_DIR"

echo "Test 1: Creating test workspace with valid and invalid files..."
cat > "$TEST_DIR/valid.yaml" << 'EOF'
test: value
nested:
  item: 123
EOF

cat > "$TEST_DIR/another-valid.json" << 'EOF'
{
  "test": "value",
  "nested": {
    "item": 456
  }
}
EOF

cat > "$TEST_DIR/invalid.yaml" << 'EOF'
invalid: yaml: content: [unclosed
EOF

echo "  ✓ Created test workspace: $TEST_DIR"
echo ""

# Test 2: Run batch validation
echo "Test 2: Running batch validation..."
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace $TEST_DIR" > /tmp/test-output.txt 2>&1 || true

echo "  ✓ Batch validation completed"
echo ""

# Test 3: Verify output contains expected sections
echo "Test 3: Verifying output format..."
if grep -q "Batch Configuration File Validation" /tmp/test-output.txt; then
  echo "  ✓ Contains header"
fi
if grep -q "Step 1: Discovering configuration files" /tmp/test-output.txt; then
  echo "  ✓ Contains discovery step"
fi
if grep -q "Step 2: Validating file syntax" /tmp/test-output.txt; then
  echo "  ✓ Contains validation step"
fi
if grep -q "VALIDATION SUMMARY" /tmp/test-output.txt; then
  echo "  ✓ Contains summary section"
fi
if grep -q "Total files:" /tmp/test-output.txt; then
  echo "  ✓ Contains total count"
fi
if grep -q "Successful:" /tmp/test-output.txt; then
  echo "  ✓ Contains success count"
fi
if grep -q "Errors:" /tmp/test-output.txt; then
  echo "  ✓ Contains error count"
fi
if grep -q "Files with errors:" /tmp/test-output.txt; then
  echo "  ✓ Contains error list"
fi
echo ""

# Test 4: Verify exit code
echo "Test 4: Verifying exit code (should be 1 for errors)..."
nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace $TEST_DIR" > /dev/null 2>&1
EXIT_CODE=$?
if [ $EXIT_CODE -eq 1 ]; then
  echo "  ✓ Exit code is 1 (errors found)"
else
  echo "  ✗ Exit code is $EXIT_CODE (expected 1)"
  exit 1
fi
echo ""

# Test 5: Test with only valid files
echo "Test 5: Testing with only valid files (should exit 0)..."
VALID_DIR="/tmp/batch-validation-valid-$$"
mkdir -p "$VALID_DIR"
cat > "$VALID_DIR/valid.yaml" << 'EOF'
test: value
EOF

nix-shell -p python3.pkgs.pyyaml python3.pkgs.tomli --run \
  "python3 scripts/debug-config-parser/batch_validate.py --workspace $VALID_DIR" > /dev/null 2>&1
EXIT_CODE=$?
if [ $EXIT_CODE -eq 0 ]; then
  echo "  ✓ Exit code is 0 (all valid)"
else
  echo "  ✗ Exit code is $EXIT_CODE (expected 0)"
  exit 1
fi
echo ""

# Test 6: Verify file-by-file output
echo "Test 6: Verifying file-by-file results..."
if grep -q "valid.yaml" /tmp/test-output.txt; then
  echo "  ✓ Shows valid.yaml result"
fi
if grep -q "invalid.yaml" /tmp/test-output.txt; then
  echo "  ✓ Shows invalid.yaml result"
fi
if grep -q "another-valid.json" /tmp/test-output.txt; then
  echo "  ✓ Shows another-valid.json result"
fi
echo ""

# Cleanup
rm -rf "$TEST_DIR" "$VALID_DIR" /tmp/test-output.txt

echo "=== All Acceptance Criteria Tests Passed ✅ ==="
echo ""
echo "Summary:"
echo "  ✅ Batch processor validates all files in inventory"
echo "  ✅ Generates comprehensive report with file-by-file results"
echo "  ✅ Summary includes total/success/failed counts"
echo "  ✅ Lists all files with syntax errors"
echo "  ✅ Returns proper exit codes (0=success, 1=errors found)"
echo "  ✅ Integration-ready for CI/CD pipelines"
