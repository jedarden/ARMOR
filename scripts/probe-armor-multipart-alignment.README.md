# ARMOR Multipart Upload Alignment Probe

A reproducible test script that validates ARMOR's part-size alignment requirements for multipart uploads.

## Background

ARMOR uses a block-aligned uniform-part-size contract for parts that have a
following part. This script verifies the ADR-005 exemptions for a lone part and
the final part, including exact GET/HEAD readback without zero-padding.

## Prerequisites

- Python 3 with `boto3` installed: `pip install boto3`
- AWS credentials configured (standard boto3 credential chain)
- Access to an ARMOR endpoint (local or remote)

## Installation

The script lives at `scripts/probe-armor-multipart-alignment.py` in the ARMOR repo.

```bash
cd ~/ARMOR
# No installation needed - it's a standalone script
```

## Usage

Set environment variables for your ARMOR instance:

```bash
# Required: Target bucket
export ARMOR_BUCKET=devimprint

# Optional: ARMOR endpoint (defaults to AWS S3 if not set)
export ARMOR_ENDPOINT_URL=http://127.0.0.1:9000

# Standard AWS credentials (optional if using instance profiles or ~/.aws/credentials)
export AWS_ACCESS_KEY_ID=your-key
export AWS_SECRET_ACCESS_KEY=your-secret
```

Run the probe:

```bash
cd ~/ARMOR/scripts
python3 probe-armor-multipart-alignment.py
```

## Test Cases

The script tests the following scenarios:

1. **Single misaligned part** - A >5 MiB part not a multiple of 65536 (OK expected)
2. **Single aligned part** - Part size is a multiple of 65536 (OK expected)
3. **Aligned first part + short final part** - Final part not aligned (OK expected)
4. **Misaligned first part + short final part** - Upload is single-part-only (FAIL expected)
5. **Exactly-one-block part** - Minimal aligned part (OK expected)
6. **Zero-byte final part** - Empty final part, no padding (OK expected)
7. **PutObject with misaligned length** - Non-multipart upload (OK expected)

## Expected Behavior

Under ADR-005:
- A lone part may be any size, including non-aligned.
- A short final part may be any size, including zero bytes.
- Regular parts that have a following part must remain uniform and block-aligned.
- Successful cases must return exactly the uploaded bytes on GET and report the
  same length on HEAD; ARMOR does not zero-pad the final part.

## Interpretation of Results

- **All tests pass**: ARMOR validates alignment as expected
- **All multipart tests fail with same error**: ARMOR has a broader multipart upload bug (not alignment-specific)
- **PutObject passes but multipart fails**: Alignment only affects multipart, not single-part uploads

## Running in iad-ci

To test against ARMOR in the iad-ci cluster:

```bash
# Set up kubectl port-forward to ARMOR
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  port-forward -n armor svc/armor 9000:9000

# In another terminal, run the probe
export ARMOR_ENDPOINT_URL=http://localhost:9000
export ARMOR_BUCKET=your-test-bucket
python3 ~/ARMOR/scripts/probe-armor-multipart-alignment.py
```

## History

- Created: 2026-08-07
- Initial run revealed that ARMOR multipart uploads are completely broken, not just alignment-validated (all multipart tests fail with InternalError/InvalidPart, not InvalidPartSize)
- See bead bf-3i9nr for full results and analysis
