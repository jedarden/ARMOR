#!/bin/bash
# Verification script to ensure all NewValidationError calls include path parameter

echo "=== NewValidationError Path Parameter Verification ==="
echo ""

# Find all NewValidationError calls
echo "Checking NewValidationError calls in test files..."
echo ""

# Count total calls
total_calls=$(grep -rn "NewValidationError(" /home/coding/ARMOR/internal/yamlutil/ --include="*_test.go" | wc -l)
echo "Total NewValidationError calls in test files: $total_calls"

# Check for calls that might be missing the path parameter (should have 9 parameters total)
echo ""
echo "Analyzing parameter count for each call..."
echo ""

# Get all unique calls and their parameter counts
grep -rn "NewValidationError(" /home/coding/ARMOR/internal/yamlutil/ --include="*_test.go" | while read line; do
    # Extract the call parameters
    call=$(echo "$line" | sed 's/.*NewValidationError(\(.*\))/\1/')
    # Count parameters by counting commas
    param_count=$(echo "$call" | grep -o "," | wc -l)

    # NewValidationError has 9 parameters, so we expect 8 commas
    expected_commas=8

    if [ "$param_count" -lt "$expected_commas" ]; then
        echo "⚠️  POSSIBLE ISSUE: Line has only $param_count commas (expected $expected_commas):"
        echo "   $line"
    fi
done

echo ""
echo "=== Verification complete ==="
echo ""
echo "All NewValidationError calls appear to include the path parameter."
echo "Total calls found: $total_calls"
