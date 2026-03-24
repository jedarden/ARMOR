# ARMOR Integration Tests

These tests verify ARMOR's functionality against a real B2 bucket and Cloudflare CDN.

## Prerequisites

1. A B2 bucket configured for ARMOR (see main README)
2. Cloudflare domain CNAME'd to the B2 bucket
3. ARMOR server running locally or accessible via network

## Environment Variables

All of these must be set to run the integration tests:

| Variable | Description | Example |
|----------|-------------|---------|
| `ARMOR_INTEGRATION_TEST` | Must be `1` to enable tests | `1` |
| `ARMOR_B2_ACCESS_KEY_ID` | B2 application key ID | `00abcd...` |
| `ARMOR_B2_SECRET_ACCESS_KEY` | B2 application key secret | `K00xyz...` |
| `ARMOR_B2_REGION` | B2 region | `us-east-005` |
| `ARMOR_BUCKET` | B2 bucket name | `my-armor-bucket` |
| `ARMOR_CF_DOMAIN` | Cloudflare domain | `armor-b2.example.com` |
| `ARMOR_MEK` | Master encryption key (hex, 32 bytes) | `a1b2c3...` (64 hex chars) |
| `ARMOR_AUTH_ACCESS_KEY` | ARMOR client access key | `my-access-key` |
| `ARMOR_AUTH_SECRET_KEY` | ARMOR client secret key | `my-secret-key` |
| `ARMOR_ENDPOINT` | ARMOR server endpoint (optional) | `http://localhost:9000` |
| `ARMOR_ADMIN_ENDPOINT` | ARMOR admin endpoint (optional) | `http://localhost:9001` |

## Running Tests

### Quick Start

```bash
# Set environment variables
export ARMOR_INTEGRATION_TEST=1
export ARMOR_B2_ACCESS_KEY_ID=your-key-id
export ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret
export ARMOR_B2_REGION=us-east-005
export ARMOR_BUCKET=your-bucket
export ARMOR_CF_DOMAIN=your-cf-domain.example.com
export ARMOR_MEK=$(openssl rand -hex 32)
export ARMOR_AUTH_ACCESS_KEY=test-access-key
export ARMOR_AUTH_SECRET_KEY=test-secret-key

# Start ARMOR (in another terminal)
go run ./cmd/armor

# Run integration tests
go test -tags=integration ./tests/integration/...
```

### Running Specific Tests

```bash
# Run only PutGetRoundtrip test
go test -tags=integration ./tests/integration/... -run TestPutGetRoundtrip

# Run with verbose output
go test -tags=integration ./tests/integration/... -v

# Skip long-running tests
go test -tags=integration ./tests/integration/... -short
```

## Test Coverage

| Test | Description |
|------|-------------|
| `TestPutGetRoundtrip` | Upload and download through ARMOR |
| `TestRangeRead` | Range request handling |
| `TestHeadObject` | HeadObject returns correct plaintext size |
| `TestListObjectsV2` | Listing with size correction |
| `TestDeleteObject` | Delete through ARMOR |
| `TestCopyObject` | Copy with DEK re-wrapping |
| `TestMultipartUpload` | Full multipart upload flow |
| `TestLargeFile` | Files above streaming threshold |
| `TestConditionalRequests` | ETag-based conditionals |
| `TestPresignedURL` | Pre-signed URL sharing |
| `TestHealthEndpoints` | /healthz and /readyz |
| `TestCanaryEndpoint` | Canary integrity check |
| `TestDirectB2Download` | Confirms encryption is working |

## CI Integration

For CI pipelines, use a test bucket dedicated to integration testing:

```yaml
# Example GitHub Actions
- name: Run Integration Tests
  env:
    ARMOR_INTEGRATION_TEST: 1
    ARMOR_B2_ACCESS_KEY_ID: ${{ secrets.B2_ACCESS_KEY_ID }}
    ARMOR_B2_SECRET_ACCESS_KEY: ${{ secrets.B2_SECRET_ACCESS_KEY }}
    ARMOR_B2_REGION: us-east-005
    ARMOR_BUCKET: armor-test-bucket
    ARMOR_CF_DOMAIN: ${{ secrets.CF_DOMAIN }}
    ARMOR_MEK: ${{ secrets.MEK }}
    ARMOR_AUTH_ACCESS_KEY: test-key
    ARMOR_AUTH_SECRET_KEY: test-secret
  run: |
    go run ./cmd/armor &
    sleep 5
    go test -tags=integration ./tests/integration/... -v
```

## Troubleshooting

### "Skipping integration tests: ARMOR_INTEGRATION_TEST not set"

Set the environment variable: `export ARMOR_INTEGRATION_TEST=1`

### "Skipping integration tests: missing environment variables"

Ensure all required environment variables are set.

### Connection refused errors

Make sure ARMOR is running and accessible at the endpoint specified by `ARMOR_ENDPOINT`.

### Cloudflare download failures

1. Verify the Cloudflare CNAME is configured correctly
2. Check that the bucket is set to public
3. Ensure SSL mode is "Full (strict)"

### Test cleanup failures

Integration tests attempt to clean up created objects, but may fail if:
- The object was never created
- Network issues during deletion
- B2 rate limiting

Orphaned test objects can be manually removed from the bucket.
