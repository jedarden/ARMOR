# AWS CLI Compatibility Tests for ARMOR

This directory contains tests for verifying AWS CLI compatibility with ARMOR.

## Purpose

These tests verify that the standard AWS CLI commands work correctly against ARMOR. This is important because:

1. **SigV4 Signing Edge Cases**: AWS CLI uses the AWS SDK for Python (botocore), which implements SigV4 signing differently than boto3 or other SDKs
2. **XML Response Format**: AWS CLI parses XML responses differently and may have strict requirements
3. **Header Compatibility**: Different SDKs handle HTTP headers in various ways; these tests catch header-related issues

## Tests Covered

| Test | Command | Description |
|------|---------|-------------|
| `test_s3_cp_upload` | `aws s3 cp file s3://bucket/key` | Upload a file to ARMOR |
| `test_s3_cp_download` | `aws s3 cp s3://bucket/key file` | Download a file from ARMOR |
| `test_s3_ls` | `aws s3 ls s3://bucket/prefix/` | List objects in a bucket |
| `test_s3_cp_copy` | `aws s3 cp s3://src s3://dst` | Copy object within bucket |
| `test_s3_rm_single` | `aws s3 rm s3://bucket/key` | Delete a single object |
| `test_s3_rm_recursive` | `aws s3 rm s3://bucket/prefix/ --recursive` | Delete multiple objects |
| `test_s3api_head_object` | `aws s3api head-object` | Get object metadata |

## Prerequisites

1. **AWS CLI** installed: `pip install awscli` or from your package manager
2. **ARMOR server** running and accessible
3. **B2 bucket** configured with ARMOR
4. **ARMOR credentials** (access key and secret key)

## Running Tests

### Quick Start

```bash
# Set environment variables
export ARMOR_ENDPOINT="http://localhost:9000"
export ARMOR_ACCESS_KEY="your-access-key"
export ARMOR_SECRET_KEY="your-secret-key"
export ARMOR_BUCKET="your-bucket"

# Run the test
./tests/aws-cli-compatibility/test-aws-cli.sh
```

### Against Kubernetes Deployment

If ARMOR is deployed on Kubernetes with port-forward:

```bash
# Start port-forward (if not already running)
kubectl port-forward -n armor svc/armor 9000:9000 9001:9001

# Get credentials from secret
export ARMOR_ACCESS_KEY=$(kubectl get secret armor-secrets -n armor -o jsonpath='{.data.auth-access-key}' | base64 -d)
export ARMOR_SECRET_KEY=$(kubectl get secret armor-secrets -n armor -o jsonpath='{.data.auth-secret-key}' | base64 -d)
export ARMOR_BUCKET=$(kubectl get secret armor-secrets -n armor -o jsonpath='{.data.bucket}' | base64 -d)

# Run tests
./tests/aws-cli-compatibility/test-aws-cli.sh
```

### Output

The test script produces color-coded output:
- **GREEN**: Info and passed tests
- **RED**: Errors and failed tests
- **YELLOW**: Warnings and skipped tests

Final summary shows:
```
======================================
Test Summary
======================================
  Passed:  7
  Failed:  0
  Skipped: 0
======================================
```

## What Gets Tested

### 1. Upload Test
- Creates a local test file with known content
- Uploads it using `aws s3 cp`
- Verifies the upload succeeded

### 2. Download Test
- Downloads the uploaded file
- Compares content with original using `diff`
- Ensures encryption/decryption is transparent

### 3. List Test
- Lists objects with a test prefix
- Verifies the uploaded file appears in results
- Checks that list operation returns correct XML format

### 4. Copy Test
- Copies an object within the bucket using `aws s3 cp`
- Verifies ARMOR handles the copy operation correctly
- Tests DEK re-wrapping if keys are used

### 5. Delete Tests
- Tests single object delete: `aws s3 rm s3://bucket/key`
- Tests recursive delete: `aws s3 rm s3://bucket/prefix/ --recursive`
- Verifies objects are actually deleted

### 6. Metadata Test
- Uses `aws s3api head-object` to get metadata
- Verifies ContentLength is plaintext size (not encrypted size)
- Ensures ARMOR headers are present

## Known Issues

If tests fail, check:

1. **SigV4 Signing**: Verify ARMOR correctly handles the `X-Amz-Content-Sha256` header that AWS CLI sends
2. **XML Namespace**: AWS CLI is strict about XML namespaces in ListObjectsV2 responses
3. **Chunked Encoding**: For large uploads, AWS CLI uses chunked encoding; ensure ARMOR handles this
4. **Date Format**: AWS CLI requires RFC 1123 date format in Last-Modified headers

## Adding New Tests

To add a new AWS CLI operation test:

1. Add a new `test_*()` function to `test-aws-cli.sh`
2. Call the function in `main()`
3. Update this README with the test description

Example:
```bash
test_s3_sync() {
    log_test "Test: aws s3 sync"

    # Create local directory structure
    mkdir -p /tmp/sync-test/src
    echo "test" > /tmp/sync-test/src/file.txt

    # Sync to ARMOR
    if aws s3 sync \
        --endpoint-url="$ARMOR_ENDPOINT" \
        --region=us-east-1 \
        /tmp/sync-test/src \
        "s3://${ARMOR_BUCKET}/sync-test/" > /dev/null 2>&1; then
        pass "Successfully synced directory"
    else
        fail "Failed to sync directory"
    fi

    # Cleanup
    rm -rf /tmp/sync-test
    aws s3 rm --endpoint-url="$ARMOR_ENDPOINT" --region=us-east-1 \
        "s3://${ARMOR_BUCKET}/sync-test/" --recursive --quiet
}
```

## Cleanup

The test script automatically cleans up:
1. Local temporary files
2. Test objects uploaded to ARMOR (on exit via trap)

To manually cleanup if tests are interrupted:
```bash
aws s3 rm --endpoint-url=http://localhost:9000 --region=us-east-1 \
    s3://your-bucket/aws-cli-test-* --recursive
```
