#!/usr/bin/env python3
"""
ARMOR Multipart Upload Alignment Probe Script

Tests ARMOR's part-size validation behavior for multipart uploads.
All parts must be aligned to 65536-byte (64KB) block boundaries.

Environment variables:
    ARMOR_BUCKET           - Target bucket name (required)
    ARMOR_ENDPOINT_URL     - S3 endpoint URL (optional, for non-AWS)
    AWS_ACCESS_KEY_ID      - AWS access key (standard credential chain)
    AWS_SECRET_ACCESS_KEY  - AWS secret key (standard credential chain)
    AWS_SESSION_TOKEN      - AWS session token (standard credential chain)
"""

import os
import sys
import boto3
from botocore.exceptions import ClientError


# Configuration
BLOCK_SIZE = 65536  # 64KB alignment requirement


def create_s3_client():
    """Create an S3 client with standard credential chain."""
    endpoint_url = os.environ.get('ARMOR_ENDPOINT_URL')
    return boto3.client('s3', endpoint_url=endpoint_url if endpoint_url else None)


def get_test_bucket():
    """Get the target bucket name from environment."""
    bucket = os.environ.get('ARMOR_BUCKET')
    if not bucket:
        print("ERROR: ARMOR_BUCKET environment variable not set", file=sys.stderr)
        sys.exit(1)
    return bucket


def run_test(test_name, part_sizes, expected_result, s3, bucket):
    """
    Run a single multipart upload test.

    Args:
        test_name: Human-readable test name
        part_sizes: List of part sizes in bytes
        expected_result: 'OK' or 'FAIL'
        s3: boto3 S3 client
        bucket: Target bucket name

    Returns:
        Tuple: (passed, actual_result, error_message)
    """
    key = f"test-multipart-alignment-{test_name.replace(' ', '-').lower()}.bin"
    upload_id = None

    try:
        # Initiate multipart upload
        upload = s3.create_multipart_upload(Bucket=bucket, Key=key)
        upload_id = upload['UploadId']

        # Upload parts
        parts = []
        for i, part_size in enumerate(part_sizes, start=1):
            part_data = b'\x00' * part_size
            part = s3.upload_part(
                Bucket=bucket,
                Key=key,
                PartNumber=i,
                UploadId=upload_id,
                Body=part_data
            )
            parts.append({'PartNumber': i, 'ETag': part['ETag']})

        # Complete multipart upload
        s3.complete_multipart_upload(
            Bucket=bucket,
            Key=key,
            UploadId=upload_id,
            MultipartUpload={'Parts': parts}
        )

        # If we get here, upload succeeded
        actual_result = 'OK'
        error_message = None

        # Clean up
        s3.delete_object(Bucket=bucket, Key=key)

    except ClientError as e:
        error_code = e.response.get('Error', {}).get('Code', 'Unknown')
        error_message = f"{error_code}: {e.response.get('Error', {}).get('Message', 'No message')}"

        if error_code == 'InvalidPartSize':
            actual_result = 'FAIL'
        elif error_code == 'EntityTooSmall':
            actual_result = 'FAIL'
        else:
            actual_result = f'UNEXPECTED({error_code})'

    except Exception as e:
        actual_result = 'ERROR'
        error_message = str(type(e).__name__ + ': ' + str(e))

    # Clean up on failure
    if upload_id:
        try:
            s3.abort_multipart_upload(Bucket=bucket, Key=key, UploadId=upload_id)
        except:
            pass

    # Check if result matches expectation
    passed = (actual_result == expected_result)

    return passed, actual_result, error_message


def run_putobject_test(test_name, object_size, expected_result, s3, bucket):
    """
    Run a single PutObject test (non-multipart).

    Args:
        test_name: Human-readable test name
        object_size: Object size in bytes
        expected_result: 'OK' or 'FAIL'
        s3: boto3 S3 client
        bucket: Target bucket name

    Returns:
        Tuple: (passed, actual_result, error_message)
    """
    key = f"test-putobject-{test_name.replace(' ', '-').lower()}.bin"

    try:
        object_data = b'\x00' * object_size
        s3.put_object(Bucket=bucket, Key=key, Body=object_data)

        # If we get here, upload succeeded
        actual_result = 'OK'
        error_message = None

        # Clean up
        s3.delete_object(Bucket=bucket, Key=key)

    except ClientError as e:
        error_code = e.response.get('Error', {}).get('Code', 'Unknown')
        error_message = f"{error_code}: {e.response.get('Error', {}).get('Message', 'No message')}"

        if error_code in ('InvalidPartSize', 'EntityTooSmall'):
            actual_result = 'FAIL'
        else:
            actual_result = f'UNEXPECTED({error_code})'

    except Exception as e:
        actual_result = 'ERROR'
        error_message = str(type(e).__name__ + ': ' + str(e))

    # Check if result matches expectation
    passed = (actual_result == expected_result)

    return passed, actual_result, error_message


def main():
    """Run all test cases and report results."""
    print("=" * 80)
    print("ARMOR Multipart Upload Alignment Probe")
    print(f"Block size: {BLOCK_SIZE} bytes ({BLOCK_SIZE // 1024}KB)")
    print("=" * 80)
    print()

    # Verify bucket is set
    bucket = get_test_bucket()

    # Create S3 client
    try:
        s3 = create_s3_client()
        print(f"Connected to S3, target bucket: {bucket}")
        print()
    except Exception as e:
        print(f"ERROR: Failed to create S3 client: {e}", file=sys.stderr)
        sys.exit(1)

    # Define test cases
    test_cases = [
        {
            'name': 'Single misaligned part (4837376 B = 65536*73 + 32768)',
            'parts': [4837376],  # 73.8 blocks - NOT aligned
            'expected': 'FAIL',
            'reason': 'Part size not a multiple of block size'
        },
        {
            'name': 'Single aligned part (4849664 B = 65536*74)',
            'parts': [4849664],  # Exactly 74 blocks - aligned
            'expected': 'OK',
            'reason': 'Part size is a multiple of block size'
        },
        {
            'name': 'Aligned first part + short final part (1000 B)',
            'parts': [BLOCK_SIZE * 10, 1000],  # 10 blocks + 1000-byte tail
            'expected': 'FAIL',
            'reason': 'Final part size not a multiple of block size'
        },
        {
            'name': 'Misaligned first part + short final part',
            'parts': [4837376, 1000],  # Misaligned first + short final
            'expected': 'FAIL',
            'reason': 'First part misaligned, final part also invalid'
        },
        {
            'name': 'Exactly-one-block part (65536 B)',
            'parts': [BLOCK_SIZE],  # Exactly 1 block
            'expected': 'OK',
            'reason': 'Single minimal aligned part'
        },
        {
            'name': 'Zero-byte final part',
            'parts': [BLOCK_SIZE, 0],  # Aligned first + zero-byte final
            'expected': 'FAIL',
            'reason': 'Zero-byte part is invalid'
        },
    ]

    # Add PutObject test case
    test_cases.append({
        'name': 'PutObject with misaligned length (non-multipart)',
        'object_size': 4837376,  # Same misaligned size, but via PutObject
        'expected': 'OK',
        'reason': 'WAL archiving uses PutObject, alignment not enforced',
        'is_putobject': True
    })

    # Run tests
    results = []
    for i, test in enumerate(test_cases, start=1):
        print(f"Test {i}: {test['name']}")
        print(f"  Reason: {test['reason']}")
        print(f"  Expected: {test['expected']}")

        if test.get('is_putobject'):
            passed, actual, error = run_putobject_test(
                test['name'],
                test['object_size'],
                test['expected'],
                s3,
                bucket
            )
        else:
            passed, actual, error = run_test(
                test['name'],
                test['parts'],
                test['expected'],
                s3,
                bucket
            )

        results.append((test['name'], passed, actual, error))

        # Print result
        status = "✓ PASS" if passed else "✗ FAIL"
        print(f"  Actual: {actual} - {status}")

        if error:
            print(f"  Error: {error}")

        if not passed:
            print(f"  ⚠ MISMATCH: Expected {test['expected']}, got {actual}")

        print()

    # Summary
    print("=" * 80)
    print("SUMMARY")
    print("=" * 80)

    passed_count = sum(1 for r in results if r[1])
    total_count = len(results)

    print(f"Tests passed: {passed_count}/{total_count}")
    print()

    if passed_count == total_count:
        print("✓ All tests passed - ARMOR alignment validation works as expected")
        return 0
    else:
        print("✗ Some tests failed - ARMOR behavior differs from expectations")
        print()
        print("Failed tests:")
        for name, passed, actual, error in results:
            if not passed:
                print(f"  - {name} (got: {actual})")
        return 1


if __name__ == '__main__':
    sys.exit(main())
