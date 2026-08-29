#!/bin/bash
# Test script to verify filesystem backend works with aws-cli
#
# Acceptance: ARMOR_BACKEND=filesystem ARMOR_FS_PATH=/tmp/x ARMOR_MEK=...
# ARMOR_AUTH_ACCESS_KEY=... ARMOR_AUTH_SECRET_KEY=... armor serve
# starts and passes a PUT/GET/range/multipart/DELETE cycle with aws-cli

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_DIR="/tmp/armor-fs-test-$$"
FS_PATH="${TEST_DIR}/backend"
BUCKET="test-bucket"
TEST_FILE="${TEST_DIR}/test-file.txt"
TEST_KEY="test-object.txt"
MEK="0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20"
ACCESS_KEY="test-access-key"
SECRET_KEY="test-secret-key"

# ARMOR server configuration
ARMOR_BINARY="${ARMOR_BINARY:-./armor}"
ARMOR_PORT="19000"
ARMOR_ENDPOINT="http://localhost:${ARMOR_PORT}"

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ -n "$ARMOR_PID" ]; then
        kill $ARMOR_PID 2>/dev/null || true
        wait $ARMOR_PID 2>/dev/null || true
    fi
    rm -rf "$TEST_DIR"
}

# Set cleanup trap
trap cleanup EXIT INT TERM

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    if ! command -v aws &> /dev/null; then
        log_error "aws-cli is required but not installed"
        exit 1
    fi

    if [ ! -f "$ARMOR_BINARY" ]; then
        log_error "ARMOR binary not found at $ARMOR_BINARY"
        log_info "Build ARMOR first with: go build ./cmd/armor"
        exit 1
    fi

    log_info "Prerequisites check passed"
}

# Setup test environment
setup_environment() {
    log_info "Setting up test environment..."

    # Create test directory
    mkdir -p "$TEST_DIR"
    mkdir -p "$FS_PATH"

    # Create test file
    echo "Hello, ARMOR filesystem backend!" > "$TEST_FILE"

    log_info "Test environment created at $TEST_DIR"
}

# Start ARMOR server
start_armor() {
    log_info "Starting ARMOR server with filesystem backend..."

    # Set environment variables
    export ARMOR_BACKEND=filesystem
    export ARMOR_FS_PATH="$FS_PATH"
    export ARMOR_BUCKET="$BUCKET"
    export ARMOR_MEK="$MEK"
    export ARMOR_AUTH_ACCESS_KEY="$ACCESS_KEY"
    export ARMOR_AUTH_SECRET_KEY="$SECRET_KEY"
    export ARMOR_LISTEN="127.0.0.1:$ARMOR_PORT"
    export ARMOR_ADMIN_LISTEN="127.0.0.1:$((ARMOR_PORT + 1))"

    # Start ARMOR in background
    $ARMOR_BINARY serve &
    ARMOR_PID=$!

    # Wait for server to be ready
    log_info "Waiting for ARMOR server to start..."
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "$ARMOR_ENDPOINT/healthz" > /dev/null 2>&1; then
            log_info "ARMOR server is ready"
            return 0
        fi
        sleep 1
        attempt=$((attempt + 1))
    done

    log_error "ARMOR server failed to start"
    return 1
}

# Configure aws-cli
configure_aws() {
    log_info "Configuring aws-cli..."

    export AWS_ACCESS_KEY_ID="$ACCESS_KEY"
    export AWS_SECRET_ACCESS_KEY="$SECRET_KEY"
    export AWS_ENDPOINT_URL="$ARMOR_ENDPOINT"

    log_info "aws-cli configured"
}

# Test 1: PUT object
test_put() {
    log_test "Test 1: PUT object"

    if aws s3api put-object \
        --bucket "$BUCKET" \
        --key "$TEST_KEY" \
        --body "$TEST_FILE" \
        --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
        log_info "✓ PUT successful"
        return 0
    else
        log_error "✗ PUT failed"
        return 1
    fi
}

# Test 2: GET object
test_get() {
    log_test "Test 2: GET object"

    local output_file="${TEST_DIR}/get-output.txt"
    if aws s3api get-object \
        --bucket "$BUCKET" \
        --key "$TEST_KEY" \
        "$output_file" \
        --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
        if diff -q "$TEST_FILE" "$output_file" > /dev/null; then
            log_info "✓ GET successful and content matches"
            return 0
        else
            log_error "✗ GET content mismatch"
            return 1
        fi
    else
        log_error "✗ GET failed"
        return 1
    fi
}

# Test 3: Range GET
test_range() {
    log_test "Test 3: Range GET"

    local range_output="${TEST_DIR}/range-output.txt"
    if aws s3api get-object \
        --bucket "$BUCKET" \
        --key "$TEST_KEY" \
        --range "bytes=0-10" \
        "$range_output" \
        --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
        local expected=$(head -c 11 "$TEST_FILE")
        local actual=$(cat "$range_output")
        if [ "$expected" = "$actual" ]; then
            log_info "✓ Range GET successful"
            return 0
        else
            log_error "✗ Range GET content mismatch"
            return 1
        fi
    else
        log_error "✗ Range GET failed"
        return 1
    fi
}

# Test 4: Multipart upload
test_multipart() {
    log_test "Test 4: Multipart upload"

    local multipart_file="${TEST_DIR}/multipart-file.bin"
    local multipart_key="multipart-test.bin"

    # Create a larger file for multipart upload (> 10MB to ensure multiple parts)
    dd if=/dev/urandom of="$multipart_file" bs=1M count=15 2>/dev/null

    # Create multipart upload
    local upload_id
    upload_id=$(aws s3api create-multipart-upload \
        --bucket "$BUCKET" \
        --key "$multipart_key" \
        --endpoint-url "$ARMOR_ENDPOINT" \
        --query 'UploadId' \
        --output text 2>/dev/null)

    if [ -z "$upload_id" ]; then
        log_error "✗ Failed to create multipart upload"
        return 1
    fi

    # Upload parts (5MB parts)
    local part1_etag
    local part2_etag
    local part3_etag

    part1_etag=$(aws s3api upload-part \
        --bucket "$BUCKET" \
        --key "$multipart_key" \
        --upload-id "$upload_id" \
        --part-number 1 \
        --body "$multipart_file" \
        --endpoint-url "$ARMOR_ENDPOINT" \
        --query 'ETag' \
        --output text 2>/dev/null)

    # Split file for remaining parts
    local part2_file="${TEST_DIR}/part2.bin"
    local part3_file="${TEST_DIR}/part3.bin"
    tail -c 10M "$multipart_file" > "$part2_file"

    part2_etag=$(aws s3api upload-part \
        --bucket "$BUCKET" \
        --key "$multipart_key" \
        --upload-id "$upload_id" \
        --part-number 2 \
        --body "$part2_file" \
        --endpoint-url "$ARMOR_ENDPOINT" \
        --query 'ETag' \
        --output text 2>/dev/null)

    # Complete multipart upload
    if aws s3api complete-multipart-upload \
        --bucket "$BUCKET" \
        --key "$multipart_key" \
        --upload-id "$upload_id" \
        --endpoint-url "$ARMOR_ENDPOINT" \
        --multipart-upload "Parts=[{PartNumber=1,ETag=$part1_etag},{PartNumber=2,ETag=$part2_etag}]" > /dev/null 2>&1; then
        log_info "✓ Multipart upload successful"

        # Verify the uploaded object
        local downloaded_file="${TEST_DIR}/downloaded-multipart.bin"
        if aws s3api get-object \
            --bucket "$BUCKET" \
            --key "$multipart_key" \
            "$downloaded_file" \
            --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
            # Compare first 10MB (what we uploaded)
            local original_hash=$(head -c 10M "$multipart_file" | md5sum | cut -d' ' -f1)
            local downloaded_hash=$(head -c 10M "$downloaded_file" | md5sum | cut -d' ' -f1)
            if [ "$original_hash" = "$downloaded_hash" ]; then
                log_info "✓ Multipart content verified"
                return 0
            else
                log_error "✗ Multipart content mismatch"
                return 1
            fi
        else
            log_error "✗ Failed to download multipart object"
            return 1
        fi
    else
        log_error "✗ Failed to complete multipart upload"
        # Abort on failure
        aws s3api abort-multipart-upload \
            --bucket "$BUCKET" \
            --key "$multipart_key" \
            --upload-id "$upload_id" \
            --endpoint-url "$ARMOR_ENDPOINT" 2>/dev/null || true
        return 1
    fi
}

# Test 5: DELETE object
test_delete() {
    log_test "Test 5: DELETE object"

    if aws s3api delete-object \
        --bucket "$BUCKET" \
        --key "$TEST_KEY" \
        --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
        # Verify object is deleted
        if aws s3api head-object \
            --bucket "$BUCKET" \
            --key "$TEST_KEY" \
            --endpoint-url "$ARMOR_ENDPOINT" > /dev/null 2>&1; then
            log_error "✗ Object still exists after DELETE"
            return 1
        else
            log_info "✓ DELETE successful and object removed"
            return 0
        fi
    else
        log_error "✗ DELETE failed"
        return 1
    fi
}

# Verify filesystem storage
verify_filesystem_storage() {
    log_test "Verifying filesystem storage"

    # Check that bucket directory exists
    if [ ! -d "$FS_PATH/$BUCKET" ]; then
        log_error "✗ Bucket directory not created in filesystem"
        return 1
    fi

    log_info "✓ Filesystem storage verified"
    return 0
}

# Main test execution
main() {
    log_info "Starting ARMOR filesystem backend tests..."
    log_info "Test directory: $TEST_DIR"
    log_info "Filesystem path: $FS_PATH"

    check_prerequisites
    setup_environment
    start_armor
    configure_aws

    # Run tests
    local failures=0

    test_put || failures=$((failures + 1))
    test_get || failures=$((failures + 1))
    test_range || failures=$((failures + 1))
    test_multipart || failures=$((failures + 1))
    test_delete || failures=$((failures + 1))
    verify_filesystem_storage || failures=$((failures + 1))

    # Summary
    echo ""
    log_info "Test Summary"
    log_info "============"

    if [ $failures -eq 0 ]; then
        log_info "✓ All tests passed!"
        return 0
    else
        log_error "✗ $failures test(s) failed"
        return 1
    fi
}

# Run main
main
