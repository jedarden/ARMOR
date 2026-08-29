#!/usr/bin/env python3
"""
Enumerates objects from an S3 bucket and filters for objects over 5MiB.

This script uses boto3 to list objects from a specified S3-compatible bucket
and outputs structured data for objects exceeding the size threshold.

Usage:
    python enumerate-rs-manager-bucket.py <bucket-name>

Output format:
    [{"bucket": "<bucket-name>", "key": "...", "size_bytes": N, "created_at": "ISO8601"}, ...]
"""

import sys
import json
import argparse
from datetime import datetime, timezone
from typing import List, Dict, Any, Optional

try:
    import boto3
    from botocore.exceptions import ClientError, NoCredentialsError, PartialCredentialsError
except ImportError:
    print("Error: boto3 is required. Install it with: pip install boto3", file=sys.stderr)
    sys.exit(1)


SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes (5,242,880 bytes)


def enumerate_large_objects(
    bucket_name: str,
    endpoint_url: Optional[str] = None,
    aws_access_key_id: Optional[str] = None,
    aws_secret_access_key: Optional[str] = None,
) -> List[Dict[str, Any]]:
    """
    Enumerate objects from the specified S3 bucket and filter for objects >5MiB.

    Args:
        bucket_name: Name of the S3 bucket to enumerate
        endpoint_url: Optional custom endpoint URL for S3-compatible services
        aws_access_key_id: Optional AWS access key ID
        aws_secret_access_key: Optional AWS secret access key

    Returns:
        List of dictionaries containing bucket, key, size_bytes, and created_at
    """
    objects: List[Dict[str, Any]] = []

    # Create S3 client with optional custom configuration
    s3_config = {}
    if endpoint_url:
        s3_config['endpoint_url'] = endpoint_url

    session = boto3.Session(
        aws_access_key_id=aws_access_key_id,
        aws_secret_access_key=aws_secret_access_key,
    )

    try:
        s3_client = session.client('s3', **s3_config)
    except Exception as e:
        print(f"Error creating S3 client: {e}", file=sys.stderr)
        return []

    print(f"Enumerating bucket '{bucket_name}' for objects >5MiB...", file=sys.stderr)

    try:
        # List all objects in the bucket
        paginator = s3_client.get_paginator('list_objects_v2')
        pages = paginator.paginate(Bucket=bucket_name)

        total_objects = 0
        large_objects = 0

        for page in pages:
            if 'Contents' not in page:
                continue

            for obj in page['Contents']:
                total_objects += 1
                key = obj.get('Key', '')
                size = obj.get('Size', 0)

                # Filter for objects larger than 5MiB
                if size > SIZE_THRESHOLD:
                    large_objects += 1

                    # Extract last modified timestamp
                    last_modified = obj.get('LastModified')
                    if last_modified:
                        # Convert to ISO8601 format
                        if last_modified.tzinfo is None:
                            # Assume UTC if no timezone info
                            last_modified = last_modified.replace(tzinfo=timezone.utc)
                        created_at = last_modified.isoformat()
                    else:
                        created_at = None

                    objects.append({
                        "bucket": bucket_name,
                        "key": key,
                        "size_bytes": size,
                        "created_at": created_at,
                    })

        print(f"Total objects scanned: {total_objects}", file=sys.stderr)
        print(f"Large objects found (>5MiB): {large_objects}", file=sys.stderr)

    except NoCredentialsError:
        print("Error: AWS credentials not found. Please configure credentials.", file=sys.stderr)
        return []
    except PartialCredentialsError:
        print("Error: Incomplete AWS credentials. Please check your configuration.", file=sys.stderr)
        return []
    except ClientError as e:
        error_code = e.response.get('Error', {}).get('Code', 'Unknown')
        error_message = e.response.get('Error', {}).get('Message', str(e))
        print(f"Error accessing bucket '{bucket_name}': [{error_code}] {error_message}", file=sys.stderr)
        return []
    except Exception as e:
        print(f"Unexpected error enumerating bucket: {e}", file=sys.stderr)
        return []

    return objects


def main() -> None:
    """Main entry point for the script."""
    parser = argparse.ArgumentParser(
        description='Enumerate objects from an S3 bucket and filter for objects over 5MiB',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Enumerate a standard S3 bucket
  python enumerate-rs-manager-bucket.py my-bucket

  # Enumerate an S3-compatible bucket with custom endpoint
  python enumerate-rs-manager-bucket.py my-bucket --endpoint-url https://s3.example.com

  # Use custom AWS credentials
  python enumerate-rs-manager-bucket.py my-bucket --access-key AKIAIOSFODNN7EXAMPLE --secret-key wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
        """
    )

    parser.add_argument(
        'bucket_name',
        help='Name of the S3 bucket to enumerate'
    )

    parser.add_argument(
        '--endpoint-url',
        help='Custom endpoint URL for S3-compatible services',
        default=None
    )

    parser.add_argument(
        '--access-key',
        help='AWS access key ID (overrides environment/config)',
        default=None
    )

    parser.add_argument(
        '--secret-key',
        help='AWS secret access key (overrides environment/config)',
        default=None
    )

    parser.add_argument(
        '--output-file',
        help='Path to save the JSON output file (default: <bucket-name>-large-objects.json)',
        default=None
    )

    args = parser.parse_args()

    print(f"Enumerating bucket '{args.bucket_name}' for objects > "
          f"{SIZE_THRESHOLD / (1024 * 1024)} MiB", file=sys.stderr)

    objects = enumerate_large_objects(
        bucket_name=args.bucket_name,
        endpoint_url=args.endpoint_url,
        aws_access_key_id=args.access_key,
        aws_secret_access_key=args.secret_key,
    )

    # Output to stdout
    print(json.dumps(objects, indent=2))

    # Save to file
    output_file = args.output_file or f"{args.bucket_name}-large-objects.json"
    try:
        with open(output_file, 'w') as f:
            json.dump(objects, f, indent=2)
        print(f"\nTotal large objects found: {len(objects)}", file=sys.stderr)
        print(f"Saved to: {output_file}", file=sys.stderr)
    except IOError as e:
        print(f"Warning: Could not write to file '{output_file}': {e}", file=sys.stderr)


if __name__ == "__main__":
    main()
