#!/bin/bash
# Test ARMOR endpoint authentication
# This script verifies that authentication works correctly with various scenarios

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR/.."

echo "=== ARMOR Authentication Test Suite ==="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to report test result
report_result() {
    local test_name="$1"
    local result="$2"
    local message="${3:-}"

    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓ PASS${NC}: $test_name"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC}: $test_name"
        if [ -n "$message" ]; then
            echo -e "  ${YELLOW}$message${NC}"
        fi
        ((TESTS_FAILED++))
    fi
}

# Get ARMOR endpoint from environment or use default
ARMOR_ENDPOINT="${ARMOR_ENDPOINT:-http://localhost:9000}"
ARMOR_ACCESS_KEY="${ARMOR_ACCESS_KEY:-}"
ARMOR_SECRET_KEY="${ARMOR_SECRET_KEY:-}"
ARMOR_BUCKET="${ARMOR_BUCKET:-test-bucket}"

echo "Configuration:"
echo "  Endpoint: $ARMOR_ENDPOINT"
echo "  Bucket: $ARMOR_BUCKET"
echo ""

# Check if server is running
echo -n "Checking if ARMOR server is running... "
if ! curl -s -f "$ARMOR_ENDPOINT/healthz" > /dev/null 2>&1; then
    echo -e "${RED}FAILED${NC}"
    echo "ARMOR server is not responding at $ARMOR_ENDPOINT"
    echo "Please start the server first or set ARMOR_ENDPOINT"
    exit 1
fi
echo -e "${GREEN}OK${NC}"
echo ""

# Check if credentials are set
if [ -z "$ARMOR_ACCESS_KEY" ] || [ -z "$ARMOR_SECRET_KEY" ]; then
    echo -e "${YELLOW}Warning: ARMOR_ACCESS_KEY and ARMOR_SECRET_KEY not set${NC}"
    echo "These tests require credentials. Please set them:"
    echo "  export ARMOR_ACCESS_KEY=your-access-key"
    echo "  export ARMOR_SECRET_KEY=your-secret-key"
    echo ""
    echo "Attempting to retrieve credentials from server logs or config..."
    # Try to get credentials from the running server (if possible)
    # This would typically require admin access
fi

echo "Running authentication tests..."
echo ""

# Test 1: Missing authentication header
echo "Test 1: Request without authentication header"
TEST_NAME="No authentication header"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ARMOR_ENDPOINT/$ARMOR_BUCKET/test-key")
if [ "$HTTP_CODE" = "403" ] || [ "$HTTP_CODE" = "401" ]; then
    report_result "$TEST_NAME" "PASS"
else
    report_result "$TEST_NAME" "FAIL" "Expected 403/401, got $HTTP_CODE"
fi
echo ""

# Test 2: Invalid access key
echo "Test 2: Invalid access key"
if [ -n "$ARMOR_ACCESS_KEY" ] && [ -n "$ARMOR_SECRET_KEY" ]; then
    TEST_NAME="Invalid access key"
    CURRENT_DATE=$(date -u +"%Y%m%dT%H%M%SZ")
    # Create a signature with wrong access key
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: AWS4-HMAC-SHA256 Credential=WRONGKEY/$CURRENT_DATE/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123" \
        -H "X-Amz-Date: $CURRENT_DATE" \
        "$ARMOR_ENDPOINT/$ARMOR_BUCKET/test-key")
    if [ "$HTTP_CODE" = "403" ]; then
        report_result "$TEST_NAME" "PASS"
    else
        report_result "$TEST_NAME" "FAIL" "Expected 403, got $HTTP_CODE"
    fi
else
    echo -e "${YELLOW}Skipping - credentials not set${NC}"
fi
echo ""

# Test 3: Invalid signature
echo "Test 3: Invalid signature"
if [ -n "$ARMOR_ACCESS_KEY" ] && [ -n "$ARMOR_SECRET_KEY" ]; then
    TEST_NAME="Invalid signature"
    CURRENT_DATE=$(date -u +"%Y%m%dT%H%M%SZ")
    CREDENTIAL_DATE=$(date -u +"%Y%m%d")
    # Use correct access key but wrong signature
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: AWS4-HMAC-SHA256 Credential=$ARMOR_ACCESS_KEY/$CREDENTIAL_DATE/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=0000000000000000000000000000000000000000000000000000000000000000" \
        -H "X-Amz-Date: $CURRENT_DATE" \
        "$ARMOR_ENDPOINT/$ARMOR_BUCKET/test-key")
    if [ "$HTTP_CODE" = "403" ]; then
        report_result "$TEST_NAME" "PASS"
    else
        report_result "$TEST_NAME" "FAIL" "Expected 403, got $HTTP_CODE"
    fi
else
    echo -e "${YELLOW}Skipping - credentials not set${NC}"
fi
echo ""

# Test 4: Expired request
echo "Test 4: Expired request timestamp"
if [ -n "$ARMOR_ACCESS_KEY" ] && [ -n "$ARMOR_SECRET_KEY" ]; then
    TEST_NAME="Expired request"
    # Use a timestamp 20 minutes in the past
    OLD_DATE=$(date -u -d "20 minutes ago" +"%Y%m%dT%H%M%SZ" 2>/dev/null || date -u -v-20M +"%Y%m%dT%H%M%SZ")
    CREDENTIAL_DATE=$(date -u -d "20 minutes ago" +"%Y%m%d" 2>/dev/null || date -u -v-20M +"%Y%m%d")
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: AWS4-HMAC-SHA256 Credential=$ARMOR_ACCESS_KEY/$CREDENTIAL_DATE/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123" \
        -H "X-Amz-Date: $OLD_DATE" \
        "$ARMOR_ENDPOINT/$ARMOR_BUCKET/test-key")
    if [ "$HTTP_CODE" = "403" ]; then
        report_result "$TEST_NAME" "PASS"
    else
        report_result "$TEST_NAME" "FAIL" "Expected 403, got $HTTP_CODE"
    fi
else
    echo -e "${YELLOW}Skipping - credentials not set${NC}"
fi
echo ""

# Test 5: Missing date header
echo "Test 5: Missing X-Amz-Date header"
if [ -n "$ARMOR_ACCESS_KEY" ] && [ -n "$ARMOR_SECRET_KEY" ]; then
    TEST_NAME="Missing date header"
    CREDENTIAL_DATE=$(date -u +"%Y%m%d")
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" \
        -H "Authorization: AWS4-HMAC-SHA256 Credential=$ARMOR_ACCESS_KEY/$CREDENTIAL_DATE/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123" \
        "$ARMOR_ENDPOINT/$ARMOR_BUCKET/test-key")
    if [ "$HTTP_CODE" = "403" ]; then
        report_result "$TEST_NAME" "PASS"
    else
        report_result "$TEST_NAME" "FAIL" "Expected 403, got $HTTP_CODE"
    fi
else
    echo -e "${YELLOW}Skipping - credentials not set${NC}"
fi
echo ""

# Test 6: Health endpoint should be public (no auth required)
echo "Test 6: Public endpoint (health check) should not require auth"
TEST_NAME="Public health endpoint"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ARMOR_ENDPOINT/healthz")
if [ "$HTTP_CODE" = "200" ]; then
    report_result "$TEST_NAME" "PASS"
else
    report_result "$TEST_NAME" "FAIL" "Expected 200, got $HTTP_CODE"
fi
echo ""

# Test 7: Readiness endpoint should be public
echo "Test 7: Public endpoint (ready check) should not require auth"
TEST_NAME="Public ready endpoint"
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" "$ARMOR_ENDPOINT/readyz")
if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "503" ]; then
    report_result "$TEST_NAME" "PASS"
else
    report_result "$TEST_NAME" "FAIL" "Expected 200 or 503, got $HTTP_CODE"
fi
echo ""

# Summary
echo "=== Test Summary ==="
echo -e "Tests Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All authentication tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some authentication tests failed!${NC}"
    exit 1
fi
