#!/bin/bash
set -euo pipefail

# ARMOR Deployment Validation Script
# Performs 50MB multipart round-trip validation against deployed ARMOR instances
# Based on ord-devimprint validation from 2026-07-18

CLUSTER="${1:-}"
ENDPOINT="${2:-}"
BUCKET="${3:-}"
SCRATCH_PREFIX="${4:-armor-validation-scratch}"

if [ -z "$CLUSTER" ] || [ -z "$ENDPOINT" ] || [ -z "$BUCKET" ]; then
    echo "Usage: $0 <cluster> <endpoint> <bucket> [scratch_prefix]"
    echo "Example: $0 rs-manager http://localhost:9000 rs-manager"
    exit 1
fi

echo "=== ARMOR Deployment Validation ==="
echo "Cluster: $CLUSTER"
echo "Endpoint: $ENDPOINT"
echo "Bucket: $BUCKET"
echo "Scratch Prefix: $SCRATCH_PREFIX"
echo

# Configuration
TEST_SIZE_MB=50
TEST_FILE="/tmp/armor-validation-$CLUSTER-$$"
TEST_OBJECT="${SCRATCH_PREFIX}/test-${CLUSTER}-$(date +%s)-50MB.bin"
PART_SIZE_MB=8  # Must be block-aligned (16 MiB for 65536-byte blocks)

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check AWS CLI is configured
if ! command -v aws &> /dev/null; then
    echo -e "${RED}ERROR: aws CLI not found${NC}"
    exit 1
fi

# Check endpoint connectivity
echo -n "Testing endpoint connectivity... "
if ! curl -s --connect-timeout 5 "$ENDPOINT/readyz" > /dev/null 2>&1; then
    echo -e "${RED}FAILED${NC}"
    echo "Cannot reach $ENDPOINT/readyz"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Generate test data
echo "Generating ${TEST_SIZE_MB}MB test file..."
dd if=/dev/urandom of="$TEST_FILE" bs=1M count=$TEST_SIZE_MB 2>/dev/null
ORIGINAL_SHA=$(sha256sum "$TEST_FILE" | awk '{print $1}')
echo "Original SHA-256: $ORIGINAL_SHA"

# Configure AWS CLI for ARMOR
export AWS_ACCESS_KEY_ID="${ARMOR_AUTH_ACCESS_KEY:-}"
export AWS_SECRET_ACCESS_KEY="${ARMOR_AUTH_SECRET_KEY:-}"
export AWS_ENDPOINT_URL="$ENDPOINT"

if [ -z "$AWS_ACCESS_KEY_ID" ] || [ -z "$AWS_SECRET_ACCESS_KEY" ]; then
    echo -e "${RED}ERROR: ARMOR_AUTH_ACCESS_KEY and ARMOR_AUTH_SECRET_KEY must be set${NC}"
    rm -f "$TEST_FILE"
    exit 1
fi

# Test 1: Sequential multipart upload (ARMOR requirement)
echo
echo "=== Test 1: Sequential Multipart Upload ==="
echo "Note: ARMOR requires sequential part upload (max_concurrent_requests=1)"

if aws s3 cp "$TEST_FILE" "s3://${BUCKET}/${TEST_OBJECT}" \
    --endpoint-url="$ENDPOINT" \
    --max-concurrent-requests 1 \
    --part-size "$((PART_SIZE_MB * 1024 * 1024))" \
    2>&1 | tee /tmp/armor-upload-$$.log; then

    echo -e "${GREEN}Upload successful${NC}"
else
    echo -e "${RED}Upload FAILED${NC}"
    cat /tmp/armor-upload-$$.log
    rm -f "$TEST_FILE" /tmp/armor-*-$$*.log
    exit 1
fi

# Test 2: Download and verify
echo
echo "=== Test 2: Download and SHA-256 Verification ==="

DOWNLOAD_FILE="${TEST_FILE}.downloaded"
if aws s3 cp "s3://${BUCKET}/${TEST_OBJECT}" "$DOWNLOAD_FILE" \
    --endpoint-url="$ENDPOINT" \
    2>&1 | tee /tmp/armor-download-$$.log; then

    echo -e "${GREEN}Download successful${NC}"
else
    echo -e "${RED}Download FAILED${NC}"
    cat /tmp/armor-download-$$.log
    rm -f "$TEST_FILE" "$DOWNLOAD_FILE" /tmp/armor-*-$$*.log
    exit 1
fi

DOWNLOADED_SHA=$(sha256sum "$DOWNLOAD_FILE" | awk '{print $1}')
echo "Downloaded SHA-256: $DOWNLOADED_SHA"

if [ "$ORIGINAL_SHA" = "$DOWNLOADED_SHA" ]; then
    echo -e "${GREEN}SHA-256 MATCH - Round-trip successful${NC}"
    ROUNDTRIP_OK=true
else
    echo -e "${RED}SHA-256 MISMATCH - Data corruption detected${NC}"
    ROUNDTRIP_OK=false
fi

# Test 3: Check object metadata
echo
echo "=== Test 3: Object Metadata Check ==="

METADATA_JSON="/tmp/armor-metadata-$$.json"
if aws s3api head-object \
    --bucket "$BUCKET" \
    --key "$TEST_OBJECT" \
    --endpoint-url="$ENDPOINT" \
    --query 'Metadata' \
    --output json > "$METADATA_JSON" 2>/dev/null; then

    echo "Object metadata retrieved:"
    cat "$METADATA_JSON" | jq '.'

    # Check for ARMOR-specific metadata
    if jq -e '.["x-amz-meta-armor-multipart"]' "$METADATA_JSON" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ ARMOR multipart marker found${NC}"
    else
        echo -e "${YELLOW}⚠ ARMOR multipart marker not found (may be single-PUT)${NC}"
    fi

    if jq -e '.["x-amz-meta-armor-plaintext-sha256"]' "$METADATA_JSON" > /dev/null 2>&1; then
        STORED_SHA=$(jq -r '.["x-amz-meta-armor-plaintext-sha256"]' "$METADATA_JSON")
        echo "Stored plaintext SHA-256: $STORED_SHA"
        if [ "$STORED_SHA" = "$ORIGINAL_SHA" ]; then
            echo -e "${GREEN}✓ Stored SHA matches original${NC}"
        else
            echo -e "${RED}✗ Stored SHA mismatch${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ No plaintext SHA-256 in metadata (multipart placeholder expected)${NC}"
    fi
else
    echo -e "${RED}Failed to retrieve object metadata${NC}"
fi

# Test 4: Cleanup
echo
echo "=== Test 4: Cleanup ==="

if aws s3 rm "s3://${BUCKET}/${TEST_OBJECT}" \
    --endpoint-url="$ENDPOINT" \
    2>&1 | tee /tmp/armor-delete-$$.log; then

    echo -e "${GREEN}Test object deleted successfully${NC}"
else
    echo -e "${YELLOW}WARNING: Failed to delete test object${NC}"
    echo "Manual cleanup required: s3://${BUCKET}/${TEST_OBJECT}"
fi

# Summary
echo
echo "=== Validation Summary ==="
echo "Cluster: $CLUSTER"
echo "Endpoint: $ENDPOINT"
echo "Test Size: ${TEST_SIZE_MB}MB"
echo "Original SHA-256: $ORIGINAL_SHA"
echo "Downloaded SHA-256: $DOWNLOADED_SHA"
echo

if [ "$ROUNDTRIP_OK" = true ]; then
    echo -e "${GREEN}✓ ROUND-TRIP VALIDATION PASSED${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}✗ ROUND-TRIP VALIDATION FAILED${NC}"
    EXIT_CODE=1
fi

# Cleanup temp files
rm -f "$TEST_FILE" "$DOWNLOAD_FILE" "$METADATA_JSON" /tmp/armor-*-$$*.log

exit $EXIT_CODE
