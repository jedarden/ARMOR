#!/usr/bin/env python3
"""
Enumerates objects >5MiB in ARMOR buckets for multipart corruption audit.
Outputs object metadata including size, creation date, and multipart status.
"""

import os
import sys
import json
import boto3
from datetime import datetime
from botocore.client import Config
from typing import List, Dict, Any

# Configuration: threshold for "large" objects (5 MiB)
SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes

# Buckets to audit
AUDITED_BUCKETS = [
    "armor-apexalgo",
    "ord-devimprint",
    "iad-ci",
    "iad-kalshi",
    "rs-manager"
]

def get_b2_client() -> Any:
    """Create B2 S3 client from environment variables."""
    region = os.environ.get("ARMOR_B2_REGION")
    endpoint = os.environ.get("ARMOR_B2_ENDPOINT")
    access_key = os.environ.get("ARMOR_B2_ACCESS_KEY_ID")
    secret_key = os.environ.get("ARMOR_B2_SECRET_ACCESS_KEY")

    if not all([region, endpoint, access_key, secret_key]):
        raise ValueError("Missing required B2 environment variables. Need: ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY")

    config = Config(region_name=region)
    return boto3.client(
        's3',
        endpoint_url=endpoint,
        aws_access_key_id=access_key,
        aws_secret_access_key=secret_key,
        config=config
    )

def enumerate_bucket_objects(bucket_name: str, s3_client: Any) -> List[Dict[str, Any]]:
    """Enumerate all objects in a bucket, returning metadata for large objects."""
    large_objects = []

    try:
        paginator = s3_client.get_paginator('list_objects_v2')
        pages = paginator.paginate(Bucket=bucket_name)

        for page in pages:
            if 'Contents' not in page:
                continue

            for obj in page['Contents']:
                # Only include objects > 5MiB
                if obj['Size'] > SIZE_THRESHOLD:
                    # Get detailed metadata to check multipart status
                    try:
                        head_result = s3_client.head_object(
                            Bucket=bucket_name,
                            Key=obj['Key']
                        )

                        # Extract ARMOR-specific metadata
                        metadata = head_result.get('Metadata', {})
                        is_multipart = metadata.get('armor-multipart', 'false') == 'true'

                        large_objects.append({
                            'key': obj['Key'],
                            'size': obj['Size'],
                            'size_mb': round(obj['Size'] / (1024 * 1024), 2),
                            'last_modified': obj['LastModified'].isoformat(),
                            'etag': obj['ETag'].strip('"'),
                            'is_multipart': is_multipart,
                            'storage_class': obj.get('StorageClass', 'STANDARD')
                        })
                    except Exception as e:
                        print(f"Warning: Failed to head object {obj['Key']}: {e}", file=sys.stderr)
                        # Still include basic info
                        large_objects.append({
                            'key': obj['Key'],
                            'size': obj['Size'],
                            'size_mb': round(obj['Size'] / (1024 * 1024), 2),
                            'last_modified': obj['LastModified'].isoformat(),
                            'etag': obj['ETag'].strip('"'),
                            'is_multipart': None,
                            'storage_class': obj.get('StorageClass', 'STANDARD'),
                            'error': str(e)
                        })

    except Exception as e:
        print(f"Error enumerating bucket {bucket_name}: {e}", file=sys.stderr)
        return []

    return large_objects

def main():
    """Main enumeration function."""
    print(f"Starting large object enumeration (> {SIZE_THRESHOLD / (1024*1024)} MiB)", file=sys.stderr)
    print(f"Buckets to audit: {', '.join(AUDITED_BUCKETS)}", file=sys.stderr)

    try:
        s3_client = get_b2_client()
        results = {}

        for bucket in AUDITED_BUCKETS:
            print(f"Enumerating bucket: {bucket}...", file=sys.stderr)
            objects = enumerate_bucket_objects(bucket, s3_client)
            results[bucket] = {
                'count': len(objects),
                'objects': objects
            }
            print(f"  Found {len(objects)} large objects", file=sys.stderr)

        # Output JSON results
        output = {
            'timestamp': datetime.now().isoformat(),
            'size_threshold_bytes': SIZE_THRESHOLD,
            'size_threshold_mb': SIZE_THRESHOLD / (1024 * 1024),
            'buckets': results
        }

        print(json.dumps(output, indent=2))

        # Print summary
        total_objects = sum(b['count'] for b in results.values())
        print(f"\nTotal large objects found: {total_objects}", file=sys.stderr)

    except Exception as e:
        print(f"Fatal error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
