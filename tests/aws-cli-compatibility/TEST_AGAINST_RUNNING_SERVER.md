# Testing ARMOR Against a Running Server

This document describes how to use the `ARMOR_COMPAT_ENDPOINT` mode to test the AWS CLI compatibility suite against a running ARMOR server instead of the in-process mock.

## Background

Normally, the `aws-cli-compatibility` tests run against an in-process mock server (`mockBackend`). This is great for CI and quick development feedback, but doesn't test against a real running ARMOR server with actual storage.

## ARMOR_COMPAT_ENDPOINT Mode

When `ARMOR_COMPAT_ENDPOINT` is set, the suite targets an external server instead:

### Environment Variables

- `ARMOR_COMPAT_ENDPOINT` - The HTTP endpoint of the running ARMOR server (e.g., `http://127.0.0.1:9000`)
- `ARMOR_COMPAT_ACCESS_KEY` - ARMOR access key for authentication (e.g., `armor`)
- `ARMOR_COMPAT_SECRET_KEY` - ARMOR secret key for authentication (e.g., `armor-demo-secret`)
- `ARMOR_BUCKET` - The bucket name to use (e.g., `demo-bucket`)

### Differences from Mock Mode

1. **Missing binaries fail instead of skip** - In endpoint mode, missing `aws` or `rclone` binaries cause test failures rather than silent skips
2. **Uses real storage** - The server stores data using its configured backend (filesystem, B2, etc.)
3. **Share token tests skip** - Tests that require presigner access skip with a message (the external server's presigner secret is not available to the test harness)

## Example: Testing Against `armor demo`

### Terminal 1: Start the demo server

```bash
./armor demo
```

Output:
```
ARMOR demo mode
Filesystem backend: /tmp/armor-demo-*
S3 API: 127.0.0.1:9000
Admin API: 127.0.0.1:9001

Demo credentials:
  Access Key ID: armor
  Secret Access Key: armor-demo-secret

=== AWS CLI Configuration ===

export AWS_ACCESS_KEY_ID=armor
export AWS_SECRET_ACCESS_KEY=armor-demo-secret

# Test the connection:
aws --endpoint-url http://127.0.0.1:9000 s3 ls s3://demo-bucket

=============================
```

### Terminal 2: Run the tests

```bash
export ARMOR_COMPAT_ENDPOINT=http://127.0.0.1:9000
export ARMOR_COMPAT_ACCESS_KEY=armor
export ARMOR_COMPAT_SECRET_KEY=armor-demo-secret
export ARMOR_BUCKET=demo-bucket

go test ./tests/aws-cli-compatibility/
```

## What Gets Tested

When running in endpoint mode, all these test categories execute against the real server:

- **SDK smoke tests** (`TestVerify_*`) - Core S3 operations via aws-sdk-go-v2
- **AWS CLI tests** (`TestAWSCLI_*`) - Real `aws` CLI invocations (requires `aws` binary)
- **rclone tests** (`TestRclone_*`) - Real `rclone` invocations (requires `rclone` binary)

Tests that are skipped:
- **Share token tests** (`TestShareGET_*`) - Require access to the server's presigner secret

## Acceptance Criteria

```bash
# Terminal 1
./armor demo

# Terminal 2
ARMOR_COMPAT_ENDPOINT=http://127.0.0.1:9000 \
ARMOR_COMPAT_ACCESS_KEY=armor \
ARMOR_COMPAT_SECRET_KEY=armor-demo-secret \
ARMOR_BUCKET=demo-bucket \
go test ./tests/aws-cli-compatibility/
```

Result: All applicable tests pass ✅
