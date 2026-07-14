#!/bin/bash
# ARMOR Endpoint Verification Test Script
# Tests all ARMOR endpoints for expected responses and status codes

set -euo pipefail

# Configuration
NAMESPACE="${NAMESPACE:-armor}"
SERVICE="${SERVICE:-armor}"
S3_PORT="${S3_PORT:-9000}"
ADMIN_PORT="${ADMIN_PORT:-9001}"
TIMEOUT="${TIMEOUT:-5}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Function to print test result
print_result() {
    local test_name="$1"
    local status="$2"
    local expected="$3"
    local actual="$4"

    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $test_name"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗${NC} $test_name"
        echo -e "  ${YELLOW}Expected:${NC} $expected"
        echo -e "  ${YELLOW}Actual:${NC} $actual"
        ((TESTS_FAILED++))
    fi
}

# Function to test HTTP endpoint
test_endpoint() {
    local test_name="$1"
    local url="$2"
    local expected_status="$3"
    local expected_body="${4:-}"

    echo "Testing: $test_name"
    echo "URL: $url"

    # Make request with timeout
    response=$(curl -s -w "\n%{http_code}" --max-time "$TIMEOUT" "$url" 2>&1) || {
        print_result "$test_name" "FAIL" "HTTP $expected_status" "Request failed: $response"
        echo ""
        return
    }

    # Split response body and status code
    body=$(echo "$response" | head -n -1)
    status_code=$(echo "$response" | tail -n 1)

    echo "Response status: $status_code"
    echo "Response body: $body"

    # Check status code
    if [ "$status_code" != "$expected_status" ]; then
        print_result "$test_name" "FAIL" "HTTP $expected_status" "HTTP $status_code"
        echo ""
        return
    fi

    # Check body if provided
    if [ -n "$expected_body" ]; then
        if [[ ! "$body" == *"$expected_body"* ]]; then
            print_result "$test_name" "FAIL" "Body contains '$expected_body'" "Body: '$body'"
            echo ""
            return
        fi
    fi

    print_result "$test_name" "PASS" "HTTP $expected_status" "HTTP $status_code"
    echo ""
}

# Function to test content type
test_content_type() {
    local test_name="$1"
    local url="$2"
    local expected_ct="$3"

    echo "Testing: $test_name"
    echo "URL: $url"

    content_type=$(curl -s -I --max-time "$TIMEOUT" "$url" 2>&1 | grep -i "content-type" | cut -d' ' -f2- | tr -d '\r')

    echo "Content-Type: $content_type"

    if [[ "$content_type" == *"$expected_ct"* ]]; then
        print_result "$test_name" "PASS" "Content-Type: $expected_ct" "$content_type"
    else
        print_result "$test_name" "FAIL" "Content-Type: $expected_ct" "$content_type"
    fi
    echo ""
}

# Main test suite
echo "=========================================="
echo "ARMOR Endpoint Verification Test Suite"
echo "=========================================="
echo ""
echo "Configuration:"
echo "  Namespace: $NAMESPACE"
echo "  Service: $SERVICE"
echo "  S3 Port: $S3_PORT"
echo "  Admin Port: $ADMIN_PORT"
echo "  Timeout: ${TIMEOUT}s"
echo ""
echo "=========================================="
echo ""

# Check if we can reach the endpoints
echo "Checking endpoint availability..."
if ! curl -s --max-time "$TIMEOUT" "http://localhost:${S3_PORT}/healthz" > /dev/null 2>&1; then
    echo -e "${YELLOW}WARNING:${NC} Cannot reach http://localhost:${S3_PORT}"
    echo "Ensure kubectl port-forward is running:"
    echo "  kubectl port-forward -n $NAMESPACE svc/$SERVICE ${S3_PORT}:${S3_PORT} ${ADMIN_PORT}:${ADMIN_PORT}"
    echo ""
    echo "Attempting to continue with tests anyway..."
fi
echo ""

echo "=========================================="
echo "S3 API Endpoint Tests (Port $S3_PORT)"
echo "=========================================="
echo ""

# Test S3 API healthz
test_endpoint \
    "S3 API /healthz endpoint" \
    "http://localhost:${S3_PORT}/healthz" \
    "200" \
    "OK"

# Test S3 API readyz (may be 200 or 503 depending on health state)
echo "Testing: S3 API /readyz endpoint"
echo "URL: http://localhost:${S3_PORT}/readyz"

response=$(curl -s -w "\n%{http_code}" --max-time "$TIMEOUT" "http://localhost:${S3_PORT}/readyz" 2>&1) || {
    print_result "S3 API /readyz endpoint" "FAIL" "HTTP 200 or 503" "Request failed: $response"
    echo ""
}
body=$(echo "$response" | head -n -1)
status_code=$(echo "$response" | tail -n 1)

echo "Response status: $status_code"
echo "Response body: $body"

if [ "$status_code" = "200" ]; then
    print_result "S3 API /readyz endpoint" "PASS" "HTTP 200" "HTTP $status_code"
elif [ "$status_code" = "503" ]; then
    print_result "S3 API /readyz endpoint" "PASS" "HTTP 503 (valid for unhealthy state)" "HTTP $status_code"
else
    print_result "S3 API /readyz endpoint" "FAIL" "HTTP 200 or 503" "HTTP $status_code"
fi
echo ""

echo "=========================================="
echo "Admin API Endpoint Tests (Port $ADMIN_PORT)"
echo "=========================================="
echo ""

# Test Admin API healthz
test_endpoint \
    "Admin API /healthz endpoint" \
    "http://localhost:${ADMIN_PORT}/healthz" \
    "200" \
    "OK"

# Test metrics endpoint
echo "Testing: Admin API /metrics endpoint"
echo "URL: http://localhost:${ADMIN_PORT}/metrics"

response=$(curl -s -w "\n%{http_code}" --max-time "$TIMEOUT" "http://localhost:${ADMIN_PORT}/metrics" 2>&1) || {
    print_result "Admin API /metrics endpoint" "FAIL" "HTTP 200" "Request failed: $response"
    echo ""
}
body=$(echo "$response" | head -n -1)
status_code=$(echo "$response" | tail -n 1)

echo "Response status: $status_code"
echo "Response body (first 100 chars): ${body:0:100}"

if [ "$status_code" = "200" ]; then
    print_result "Admin API /metrics endpoint" "PASS" "HTTP 200" "HTTP $status_code"
else
    print_result "Admin API /metrics endpoint" "FAIL" "HTTP 200" "HTTP $status_code"
fi
echo ""

# Test metrics content type
test_content_type \
    "Admin API /metrics Content-Type" \
    "http://localhost:${ADMIN_PORT}/metrics" \
    "text/plain"

# Test canary endpoint
echo "Testing: Admin API /armor/canary endpoint"
echo "URL: http://localhost:${ADMIN_PORT}/armor/canary"

response=$(curl -s -w "\n%{http_code}" --max-time "$TIMEOUT" "http://localhost:${ADMIN_PORT}/armor/canary" 2>&1) || {
    print_result "Admin API /armor/canary endpoint" "FAIL" "HTTP 200" "Request failed: $response"
    echo ""
}
body=$(echo "$response" | head -n -1)
status_code=$(echo "$response" | tail -n 1)

echo "Response status: $status_code"
echo "Response body: $body"

if [ "$status_code" = "200" ]; then
    # Check if response is valid JSON
    if echo "$body" | jq empty 2>/dev/null; then
        print_result "Admin API /armor/canary endpoint" "PASS" "HTTP 200 with JSON" "HTTP $status_code"
    else
        print_result "Admin API /armor/canary endpoint" "FAIL" "HTTP 200 with JSON" "HTTP $status_code (invalid JSON)"
    fi
else
    print_result "Admin API /armor/canary endpoint" "FAIL" "HTTP 200" "HTTP $status_code"
fi
echo ""

# Test dashboard endpoint (may require auth)
echo "Testing: Admin API /dashboard endpoint"
echo "URL: http://localhost:${ADMIN_PORT}/dashboard"

response=$(curl -s -w "\n%{http_code}" --max-time "$TIMEOUT" "http://localhost:${ADMIN_PORT}/dashboard" 2>&1) || {
    print_result "Admin API /dashboard endpoint" "FAIL" "HTTP 200 or 401" "Request failed: $response"
    echo ""
}
body=$(echo "$response" | head -n -1)
status_code=$(echo "$response" | tail -n 1)

echo "Response status: $status_code"
echo "Response body (first 50 chars): ${body:0:50}"

if [ "$status_code" = "200" ]; then
    print_result "Admin API /dashboard endpoint" "PASS" "HTTP 200 (no auth)" "HTTP $status_code"
elif [ "$status_code" = "401" ]; then
    print_result "Admin API /dashboard endpoint" "PASS" "HTTP 401 (auth required)" "HTTP $status_code"
elif [ "$status_code" = "404" ]; then
    print_result "Admin API /dashboard endpoint" "PASS" "HTTP 404 (not configured)" "HTTP $status_code"
else
    print_result "Admin API /dashboard endpoint" "FAIL" "HTTP 200, 401, or 404" "HTTP $status_code"
fi
echo ""

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""
echo "Tests Passed: $TESTS_PASSED"
echo "Tests Failed: $TESTS_FAILED"
echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
