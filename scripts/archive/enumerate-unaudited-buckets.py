#!/usr/bin/env python3
"""
Enumerates objects from unaudited ARMOR buckets during affected deployment periods.

This script enumerates objects from the 4 unaudited ARMOR buckets (iad-ci, iad-kalshi,
rs-manager, armor-apexalgo) and filters for objects over 5MiB written during the
multipart upload corruption bug window (2026-03-24 to 2026-07-16).

Usage:
    python enumerate-unaudited-buckets.py --b2-key-id <KEY> --b2-key <SECRET>

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


# Configuration
SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes (5,242,880 bytes)
B2_ENDPOINT = "https://s3.us-east-005.backblazeb2.com"  # Backblaze B2 S3 endpoint

# Bug window from deployment windows analysis
BUG_WINDOW_START = "2026-03-24T00:00:00Z"
BUG_WINDOW_END = "2026-07-16T23:59:59Z"

# Target buckets to enumerate
TARGET_BUCKETS = [
    "iad-ci",
    "iad-kalshi",
    "rs-manager",
    "armor-apexalgo"
]


def parse_timestamp(ts: str) -> datetime:
    """Parse ISO8601 timestamp to datetime object."""
    if ts.endswith('Z'):
        ts = ts[:-1] + '+00:00'
    return datetime.fromisoformat(ts)


def filter_by_bug_window(objects: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
    """Filter objects to only those created within the bug window."""
    bug_start = parse_timestamp(BUG_WINDOW_START)
    bug_end = parse_timestamp(BUG_WINDOW_END)

    filtered = []
    for obj in objects:
        created_at = obj.get('created_at')
        if not created_at:
            continue

        try:
            obj_time = parse_timestamp(created_at)
            if bug_start <= obj_time <= bug_end:
                filtered.append(obj)
        except Exception as e:
            print(f"Warning: Could not parse timestamp '{created_at}': {e}", file=sys.stderr)
            continue

    return filtered


def enumerate_bucket(
    bucket_name: str,
    b2_key_id: str,
    b2_key: str,
    endpoint_url: str = B2_ENDPOINT,
) -> List[Dict[str, Any]]:
    """
    Enumerate objects from a specified B2 bucket and filter for objects >5MiB.

    Args:
        bucket_name: Name of the B2 bucket to enumerate
        b2_key_id: Backblaze B2 key ID
        b2_key: Backblaze B2 application key
        endpoint_url: B2 S3-compatible endpoint URL

    Returns:
        List of dictionaries containing bucket, key, size_bytes, and created_at
    """
    objects: List[Dict[str, Any]] = []

    try:
        # Create B2 S3 client
        s3_client = boto3.client(
            's3',
            endpoint_url=endpoint_url,
            aws_access_key_id=b2_key_id,
            aws_secret_access_key=b2_key
        )
    except Exception as e:
        print(f"Error creating B2 client for bucket '{bucket_name}': {e}", file=sys.stderr)
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

        print(f"  Total objects scanned: {total_objects}", file=sys.stderr)
        print(f"  Large objects found (>5MiB): {large_objects}", file=sys.stderr)

    except NoCredentialsError:
        print(f"Error: B2 credentials not found for bucket '{bucket_name}'.", file=sys.stderr)
        print(f"Please check your B2 key ID and application key.", file=sys.stderr)
        return []
    except PartialCredentialsError:
        print(f"Error: Incomplete B2 credentials for bucket '{bucket_name}'.", file=sys.stderr)
        return []
    except ClientError as e:
        error_code = e.response.get('Error', {}).get('Code', 'Unknown')
        error_message = e.response.get('Error', {}).get('Message', str(e))
        print(f"Error accessing bucket '{bucket_name}': [{error_code}] {error_message}", file=sys.stderr)
        return []
    except Exception as e:
        print(f"Unexpected error enumerating bucket '{bucket_name}': {e}", file=sys.stderr)
        return []

    return objects


def main() -> None:
    """Main entry point for the script."""
    parser = argparse.ArgumentParser(
        description='Enumerate unaudited ARMOR buckets for large objects during bug window',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=f"""
Example:
  python enumerate-unaudited-buckets.py --b2-key-id YOUR_KEY_ID --b2-key YOUR_APPLICATION_KEY

This will enumerate the following buckets and filter for objects >5MiB written during:
  Bug window: {BUG_WINDOW_START} to {BUG_WINDOW_END}

Target buckets:
  {', '.join(TARGET_BUCKETS)}
        """
    )

    parser.add_argument(
        '--b2-key-id',
        required=True,
        help='Backblaze B2 key ID (applicationKeyID)'
    )

    parser.add_argument(
        '--b2-key',
        required=True,
        help='Backblaze B2 application key'
    )

    parser.add_argument(
        '--endpoint-url',
        help=f'Custom B2 endpoint URL (default: {B2_ENDPOINT})',
        default=B2_ENDPOINT
    )

    parser.add_argument(
        '--output-file',
        help='Path to save the JSON output file (default: scratch/candidate-objects.json)',
        default='scratch/candidate-objects.json'
    )

    parser.add_argument(
        '--skip-bug-window-filter',
        action='store_true',
        help='Skip filtering by bug window (return all large objects)'
    )

    args = parser.parse_args()

    print("================================================================================", file=sys.stderr)
    print("ARMOR Unaudited Bucket Enumeration", file=sys.stderr)
    print("================================================================================", file=sys.stderr)
    print(f"Bug window: {BUG_WINDOW_START} to {BUG_WINDOW_END}", file=sys.stderr)
    print(f"Size threshold: {SIZE_THRESHOLD / (1024 * 1024)} MiB", file=sys.stderr)
    print(f"Target buckets: {', '.join(TARGET_BUCKETS)}", file=sys.stderr)
    print("================================================================================", file=sys.stderr)
    print()

    all_objects = []

    for bucket in TARGET_BUCKETS:
        print(f"\nProcessing bucket: {bucket}", file=sys.stderr)
        objects = enumerate_bucket(
            bucket_name=bucket,
            b2_key_id=args.b2_key_id,
            b2_key=args.b2_key,
            endpoint_url=args.endpoint_url,
        )
        all_objects.extend(objects)

    print(f"\n{'='*80}", file=sys.stderr)
    print(f"Total large objects found across all buckets: {len(all_objects)}", file=sys.stderr)

    # Filter by bug window unless explicitly disabled
    if not args.skip_bug_window_filter:
        print(f"Filtering by bug window ({BUG_WINDOW_START} to {BUG_WINDOW_END})...", file=sys.stderr)
        filtered_objects = filter_by_bug_window(all_objects)
        print(f"Candidate objects within bug window: {len(filtered_objects)}", file=sys.stderr)
        final_objects = filtered_objects
    else:
        print("Skipping bug window filter - returning all large objects", file=sys.stderr)
        final_objects = all_objects

    # Sort by creation date (newest first)
    final_objects.sort(key=lambda x: x.get('created_at', ''), reverse=True)

    # Output to stdout
    print(json.dumps(final_objects, indent=2))

    # Save to file
    try:
        with open(args.output_file, 'w') as f:
            json.dump(final_objects, f, indent=2)
        print(f"\n{'='*80}", file=sys.stderr)
        print(f"Results saved to: {args.output_file}", file=sys.stderr)
        print(f"Total objects written: {len(final_objects)}", file=sys.stderr)
    except IOError as e:
        print(f"Warning: Could not write to file '{args.output_file}': {e}", file=sys.stderr)

    # Print summary statistics
    if final_objects:
        print(f"\n{'='*80}", file=sys.stderr)
        print("Summary by bucket:", file=sys.stderr)
        bucket_counts = {}
        for obj in final_objects:
            bucket = obj.get('bucket', 'unknown')
            bucket_counts[bucket] = bucket_counts.get(bucket, 0) + 1

        for bucket, count in sorted(bucket_counts.items()):
            print(f"  {bucket}: {count} objects", file=sys.stderr)

    print(f"\n{'='*80}", file=sys.stderr)


if __name__ == "__main__":
    main()