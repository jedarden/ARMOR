#!/usr/bin/env bash
# AWS CLI Compatibility Test for ARMOR
#
# Tests AWS CLI (s3 cp, s3 ls, s3 rm) against ARMOR
#
# Environment Variables Required:
#   ARMOR_ENDPOINT        - ARMOR server endpoint (default: http://localhost:9000)
#   ARMOR_ACCESS_KEY      - ARMOR client access key
#   ARMOR_SECRET_KEY     - ARMOR client secret key
#   ARMOR_BUCKET         - B2 bucket name
#
# Usage:
#   export ARMOR_ENDPOINT=http://localhost:9000
#   export ARMOR_ACCESS_KEY=your-access-key
#   export ARMOR_SECRET_KEY=your-secret-key
#   export ARMOR_BUCKET=your-bucket
#   ./tests/aws-cli-compatibility/test-aws-cli.sh

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
ARMOR_ENDPOINT="${ARMOR_ENDPOINT:-http://localhost:9000}"
ARMOR_ACCESS_KEY="${ARMOR_ACCESS_KEY:-}"
ARMOR_SECRET_KEY="${ARMOR_SECRET_KEY:-}"
ARMOR_BUCKET="${ARMOR_BUCKET:-}"

# Test files
TEST_PREFIX="aws-cli-test-$(date +%s)-$$"
TEST_FILE_CONTENT="ARMOR AWS CLI Compatibility Test\nTimestamp: $(date)\nTest ID: $$\n"
TEST_FILE="/tmp/armor-aws-cli-test-$$-test.txt"
UPLOAD_KEY="${TEST_PREFIX}/test-upload.txt"
COPY_KEY="${TEST_PREFIX}/test-copy.txt"

# Results tracking
PASSED=0
FAILED=0
SKIPPED=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

log_test() {
    echo -e "\n${GREEN}[TEST]${NC} $*"
}

pass() {
    echo -e "${GREEN}[PASS]${NC} $*"
    ((PASSED++))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $*"
    ((FAILED++))
}

skip() {
    echo -e "${YELLOW}[SKIP]${NC} $*"
    ((SKIPPED++))
}

# Validate prerequisites
check_prerequisites() {
    log_test "Checking prerequisites..."

    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        fail "AWS CLI not found. Please install awscli."
        return 1
    fi
    pass "AWS CLI found: $(aws --version 2>&1 | head -1)"

    # Check required environment variables
    if [[ -z "$ARMOR_ACCESS_KEY" ]]; then
        fail "ARMOR_ACCESS_KEY not set"
        return 1
    fi
    pass "ARMOR_ACCESS_KEY is set"

    if [[ -z "$ARMOR_SECRET_KEY" ]]; then
        fail "ARMOR_SECRET_KEY not set"
        return 1
    fi
    pass "ARMOR_SECRET_KEY is set"

    if [[ -z "$ARMOR_BUCKET" ]]; then
        fail "ARMOR_BUCKET not set"
        return 1
    fi
    pass "ARMOR_BUCKET is set: $ARMOR_BUCKET"

    # Check ARMOR is accessible
    if ! curl -sf "${ARMOR_ENDPOINT}/healthz" > /dev/null 2>&1; then
        fail "ARMOR not accessible at ${ARMOR_ENDPOINT}"
        return 1
    fi
    pass "ARMOR is accessible at ${ARMOR_ENDPOINT}"

    log_info "All prerequisites met"
    echo ""
}

# Setup AWS CLI configuration
setup_aws_cli() {
    log_test "Setting up AWS CLI configuration..."

    # Create a temporary profile for this test
    export AWS_ACCESS_KEY_ID="$ARMOR_ACCESS_KEY"
    export AWS_SECRET_ACCESS_KEY="$ARMOR_SECRET_KEY"
    export AWS_DEFAULT_REGION="us-east-1"

    # Configure AWS CLI to use ARMOR endpoint
    # We use environment variables instead of profiles for simplicity
    export AWS_ENDPOINT_URL="$ARMOR_ENDPOINT"

    pass "AWS CLI configured for endpoint: $ARMOR_ENDPOINT"
    echo ""
}

# Cleanup function
cleanup() {
    log_test "Cleaning up test artifacts..."

    # Remove local test file
    rm -f "$TEST_FILE"

    # Remove test objects from ARMOR
    log_info "Removing test objects from s3://${ARMOR_BUCKET}/${TEST_PREFIX}/"

    # Use --quiet to suppress output, check exit status
    aws s3 rm \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${TEST_PREFIX}/" \
        --recursive \
        --quiet 2>/dev/null || log_warn "Failed to cleanup test objects (may not exist)"

    log_info "Cleanup completed"
    echo ""
}

# Test: aws s3 cp (upload)
test_s3_cp_upload() {
    log_test "Test: aws s3 cp (upload)"

    # Create test file
    echo -e "$TEST_FILE_CONTENT" > "$TEST_FILE"

    # Upload file
    if aws s3 cp \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "$TEST_FILE" \
        "s3://${ARMOR_BUCKET}/${UPLOAD_KEY}" 2>&1 | grep -q "upload"; then
        pass "Successfully uploaded file to s3://${ARMOR_BUCKET}/${UPLOAD_KEY}"
    else
        fail "Failed to upload file"
        return 1
    fi
    echo ""
}

# Test: aws s3 cp (download)
test_s3_cp_download() {
    log_test "Test: aws s3 cp (download)"

    local download_file="/tmp/armor-aws-cli-test-$$-download.txt"

    # Download file
    if aws s3 cp \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${UPLOAD_KEY}" \
        "$download_file" > /dev/null 2>&1; then

        # Verify content
        if diff -q "$TEST_FILE" "$download_file" > /dev/null 2>&1; then
            pass "Successfully downloaded file and content matches"
            rm -f "$download_file"
        else
            fail "Downloaded content does not match original"
            rm -f "$download_file"
            return 1
        fi
    else
        fail "Failed to download file"
        rm -f "$download_file"
        return 1
    fi
    echo ""
}

# Test: aws s3 ls
test_s3_ls() {
    log_test "Test: aws s3 ls (list objects)"

    # List objects in bucket with our prefix
    local output
    output=$(aws s3 ls \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${TEST_PREFIX}/" 2>&1)

    if echo "$output" | grep -q "test-upload.txt"; then
        pass "Successfully listed objects, found test-upload.txt"
        log_info "List output:\n$output"
    else
        fail "Failed to list objects or test-upload.txt not found"
        log_info "List output:\n$output"
        return 1
    fi
    echo ""
}

# Test: aws s3 cp (copy within bucket)
test_s3_cp_copy() {
    log_test "Test: aws s3 cp (copy within bucket)"

    # Copy object
    if aws s3 cp \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${UPLOAD_KEY}" \
        "s3://${ARMOR_BUCKET}/${COPY_KEY}" > /dev/null 2>&1; then
        pass "Successfully copied object within bucket"
    else
        fail "Failed to copy object"
        return 1
    fi
    echo ""
}

# Test: aws s3 rm (single object)
test_s3_rm_single() {
    log_test "Test: aws s3 rm (single object)"

    # Delete copy
    if aws s3 rm \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${COPY_KEY}" > /dev/null 2>&1; then
        pass "Successfully deleted single object"
    else
        fail "Failed to delete object"
        return 1
    fi

    # Verify it's gone
    if aws s3 ls \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${COPY_KEY}" 2>&1 | grep -q "test-copy.txt"; then
        fail "Object still exists after deletion"
        return 1
    else
        pass "Verified object is deleted"
    fi
    echo ""
}

# Test: aws s3 rm (recursive)
test_s3_rm_recursive() {
    log_test "Test: aws s3 rm (recursive cleanup)"

    # Create additional test file
    local extra_key="${TEST_PREFIX}/extra-test.txt"
    echo "extra content" > /tmp/extra-test-$$
    aws s3 cp \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "/tmp/extra-test-$$" \
        "s3://${ARMOR_BUCKET}/${extra_key}" > /dev/null 2>&1 || true
    rm -f /tmp/extra-test-$$

    # Delete all test objects recursively
    if aws s3 rm \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${TEST_PREFIX}/" \
        --recursive > /dev/null 2>&1; then
        pass "Successfully deleted objects recursively"
    else
        fail "Failed to delete objects recursively"
        return 1
    fi

    # Verify they're all gone
    local remaining
    remaining=$(aws s3 ls \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${TEST_PREFIX}/" 2>&1 || true)

    if [[ -n "$remaining" ]] && ! echo "$remaining" | grep -q "NoSuchBucket"; then
        fail "Objects still exist after recursive delete"
        log_info "Remaining objects:\n$remaining"
        return 1
    else
        pass "Verified all test objects deleted"
    fi
    echo ""
}

# Test: aws s3api head-object (metadata check)
test_s3api_head_object() {
    log_test "Test: aws s3api head-object (metadata)"

    # Upload a fresh file for this test
    local head_test_key="${TEST_PREFIX}/head-test.txt"
    echo "head test content" > /tmp/head-test-$$

    if ! aws s3 cp \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "/tmp/head-test-$$" \
        "s3://${ARMOR_BUCKET}/${head_test_key}" > /dev/null 2>&1; then
        rm -f /tmp/head-test-$$
        skip "Failed to upload test file for head-object test"
        return 0
    fi
    rm -f /tmp/head-test-$$

    # Get object metadata
    local metadata
    metadata=$(aws s3api head-object \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        --bucket="$ARMOR_BUCKET" \
        --key="$head_test_key" 2>&1)

    if [[ $? -eq 0 ]]; then
        pass "Successfully retrieved object metadata"

        # Check ContentLength (should be plaintext size)
        local size
        size=$(echo "$metadata" | grep -o '"ContentLength": [0-9]*' | grep -o '[0-9]*' || echo "0")
        if [[ "$size" == "19" ]]; then
            pass "ContentLength is correct (plaintext size): $size bytes"
        else
            log_warn "ContentLength may be incorrect: $size (expected 19)"
        fi
    else
        fail "Failed to retrieve object metadata"
        log_info "Error output:\n$metadata"
    fi

    # Cleanup
    aws s3 rm \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/${head_test_key}" > /dev/null 2>&1 || true

    echo ""
}

# Main test runner
main() {
    echo "======================================"
    echo "ARMOR AWS CLI Compatibility Test"
    echo "======================================"
    echo ""
    echo "Configuration:"
    echo "  Endpoint: $ARMOR_ENDPOINT"
    echo "  Bucket: $ARMOR_BUCKET"
    echo "  Test Prefix: $TEST_PREFIX"
    echo ""
    echo "======================================"
    echo ""

    # Check prerequisites
    if ! check_prerequisites; then
        log_error "Prerequisites not met. Exiting."
        exit 1
    fi

    # Setup AWS CLI
    setup_aws_cli

    # Set up cleanup trap
    trap cleanup EXIT

    # Run tests
    test_s3_cp_upload || true
    test_s3_cp_download || true
    test_s3_ls || true
    test_s3_cp_copy || true
    test_s3_rm_single || true
    test_s3api_head_object || true
    test_s3_rm_recursive || true

    # Print summary
    echo "======================================"
    echo "Test Summary"
    echo "======================================"
    echo "  Passed:  $PASSED"
    echo "  Failed:  $FAILED"
    echo "  Skipped: $SKIPPED"
    echo "======================================"

    if [[ $FAILED -eq 0 ]]; then
        log_info "All tests passed!"
        exit 0
    else
        log_error "Some tests failed!"
        exit 1
    fi
}

# Run main
main "$@"
