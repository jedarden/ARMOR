#!/usr/bin/env python3
"""
Enumerates objects >5MiB in ARMOR buckets via HTTP API for multipart corruption audit.
Uses ARMOR's S3-compatible HTTP API instead of direct B2 access.
"""

import os
import sys
import json
import hmac
import hashlib
import datetime
import urllib.request
import urllib.parse
import xml.etree.ElementTree as ET
from typing import List, Dict, Any, Optional

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

# ARMOR cluster endpoints (using local port-forward for each cluster)
# We'll use different local ports for each cluster
CLUSTER_PORTS = {
    "iad-ci": 9000,
    "iad-kalshi": 9001,
    "rs-manager": 9002,
    "ord-devimprint": 9003,
    "armor-apexalgo": 9004
}


class AWSV4Signer:
    """AWS Signature Version 4 signer for ARMOR authentication"""

    def __init__(self, access_key: str, secret_key: str, region: str = "us-east-005"):
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.service = "s3"

    def _sign(self, key: bytes, msg: str) -> bytes:
        """HMAC SHA256"""
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    def _get_signature_key(self, date_stamp: str) -> bytes:
        """Derive signature key"""
        k_date = self._sign(('AWS4' + self.secret_key).encode('utf-8'), date_stamp)
        k_region = self._sign(k_date, self.region)
        k_service = self._sign(k_region, self.service)
        k_signing = self._sign(k_service, 'aws4_request')
        return k_signing

    def sign_request(self, method: str, host: str, path: str,
                     headers: Dict[str, str] = None, body: bytes = b'') -> Dict[str, str]:
        """Sign an HTTP request with AWS V4 signature"""
        now = datetime.datetime.now(datetime.timezone.utc)
        amz_date = now.strftime('%Y%m%dT%H%M%SZ')
        date_stamp = now.strftime('%Y%m%d')

        if headers is None:
            headers = {}

        headers['X-Amz-Date'] = amz_date
        if 'Host' not in headers:
            headers['Host'] = host

        payload_hash = hashlib.sha256(body).hexdigest()
        headers['X-Amz-Content-Sha256'] = payload_hash

        # Canonical headers
        canonical_headers = ''
        signed_headers_list = []
        for key in sorted(headers.keys()):
            lower_key = key.lower()
            signed_headers_list.append(lower_key)
            value = str(headers[key]).strip()
            canonical_headers += f'{lower_key}:{value}\n'

        signed_headers = ';'.join(signed_headers_list)

        # Canonical request
        canonical_uri = urllib.parse.quote(path, safe='/')
        canonical_querystring = ''
        canonical_request = f'{method}\n{canonical_uri}\n{canonical_querystring}\n{canonical_headers}\n{signed_headers}\n{payload_hash}'

        # Create string to sign
        credential_scope = f'{date_stamp}/{self.region}/{self.service}/aws4_request'
        canonical_request_hash = hashlib.sha256(canonical_request.encode('utf-8')).hexdigest()
        string_to_sign = f'AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n{canonical_request_hash}'

        # Calculate signature
        signing_key = self._get_signature_key(date_stamp)
        signature = hmac.new(signing_key, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

        # Add authorization header
        authorization_header = (
            f'AWS4-HMAC-SHA256 '
            f'Credential={self.access_key}/{credential_scope}, '
            f'SignedHeaders={signed_headers}, '
            f'Signature={signature}'
        )
        headers['Authorization'] = authorization_header

        return headers


def get_armor_credentials() -> tuple[str, str]:
    """Get ARMOR credentials from environment or prompt."""
    access_key = os.environ.get("ARMOR_AUTH_ACCESS_KEY")
    secret_key = os.environ.get("ARMOR_AUTH_SECRET_KEY")

    if not access_key:
        # Try to get from cluster deployment
        print("Warning: ARMOR_AUTH_ACCESS_KEY not set in environment", file=sys.stderr)
        print("Attempting to use cluster credentials...", file=sys.stderr)

    if not secret_key:
        print("Warning: ARMOR_AUTH_SECRET_KEY not set in environment", file=sys.stderr)

    return access_key, secret_key


def list_objects_v2(bucket: str, port: int, access_key: str, secret_key: str) -> List[Dict[str, Any]]:
    """List objects in bucket using ListObjectsV2 via ARMOR HTTP API (localhost port-forward)"""
    objects = []

    try:
        # Use localhost with specified port
        host = "localhost"
        base_url = f"http://{host}:{port}"

        # Create signer
        signer = AWSV4Signer(access_key, secret_key)

        # ListObjectsV2 request
        path = f"/{bucket}?list-type=2"
        continuation_token = None

        while True:
            # Build query parameters
            query_params = [("list-type", "2")]
            if continuation_token:
                query_params.append(("continuation-token", continuation_token))

            query_string = urllib.parse.urlencode(query_params)
            full_path = f"/{bucket}?{query_string}"

            # Sign request
            headers = {
                "Host": host,
            }
            signed_headers = signer.sign_request("GET", host, full_path, headers=headers)

            # Make request
            url = f"{base_url}{full_path}"
            req = urllib.request.Request(url, headers=signed_headers, method="GET")

            try:
                with urllib.request.urlopen(req, timeout=30) as response:
                    xml_data = response.read().decode('utf-8')

                # Parse XML response
                root = ET.fromstring(xml_data)

                # Namespace
                ns = {'s3': 'http://s3.amazonaws.com/doc/2006-03-01/'}

                # Get objects
                for contents in root.findall('s3:Contents', ns):
                    key = contents.find('s3:Key', ns).text
                    size = int(contents.find('s3:Size', ns).text)
                    last_modified = contents.find('s3:LastModified', ns).text
                    etag = contents.find('s3:ETag', ns).text.strip('"')

                    objects.append({
                        'key': key,
                        'size': size,
                        'size_mb': round(size / (1024 * 1024), 2),
                        'last_modified': last_modified,
                        'etag': etag
                    })

                # Check for continuation token
                next_continuation = root.find('s3:NextContinuationToken', ns)
                if next_continuation is not None:
                    continuation_token = next_continuation.text
                else:
                    break

            except urllib.error.HTTPError as e:
                print(f"HTTP Error listing objects in {bucket}: {e.code} - {e.reason}", file=sys.stderr)
                if e.code == 403:
                    print("Access denied - check credentials", file=sys.stderr)
                elif e.code == 404:
                    print(f"Bucket {bucket} not found or inaccessible", file=sys.stderr)
                break
            except Exception as e:
                print(f"Error listing objects in {bucket}: {e}", file=sys.stderr)
                break

    except Exception as e:
        print(f"Error setting up request for {bucket}: {e}", file=sys.stderr)

    return objects


def enumerate_bucket_objects(bucket_name: str, port: int,
                           access_key: str, secret_key: str) -> List[Dict[str, Any]]:
    """Enumerate all objects in a bucket via local port-forward, returning metadata for large objects."""
    large_objects = []

    try:
        print(f"  Listing all objects in {bucket_name} via localhost:{port}...", file=sys.stderr)
        all_objects = list_objects_v2(bucket_name, port, access_key, secret_key)
        print(f"  Found {len(all_objects)} total objects", file=sys.stderr)

        for obj in all_objects:
            # Only include objects > 5MiB
            if obj['size'] > SIZE_THRESHOLD:
                large_objects.append(obj)

    except Exception as e:
        print(f"Error enumerating bucket {bucket_name}: {e}", file=sys.stderr)

    return large_objects


def main():
    """Main enumeration function."""
    print(f"Starting large object enumeration (> {SIZE_THRESHOLD / (1024*1024)} MiB) via ARMOR HTTP API", file=sys.stderr)
    print(f"Buckets to audit: {', '.join(AUDITED_BUCKETS)}", file=sys.stderr)

    # Get credentials
    access_key, secret_key = get_armor_credentials()

    if not access_key or not secret_key:
        print("Error: ARMOR credentials not available. Set ARMOR_AUTH_ACCESS_KEY and ARMOR_AUTH_SECRET_KEY.", file=sys.stderr)
        sys.exit(1)

    print(f"Using access key: {access_key[:8]}...", file=sys.stderr)

    results = {}

    for bucket in AUDITED_BUCKETS:
        print(f"\nEnumerating bucket: {bucket}...", file=sys.stderr)

        port = CLUSTER_PORTS.get(bucket)
        if not port:
            print(f"  Warning: No port configured for {bucket}, skipping", file=sys.stderr)
            continue

        objects = enumerate_bucket_objects(bucket, port, access_key, secret_key)
        results[bucket] = {
            'count': len(objects),
            'objects': objects
        }
        print(f"  Found {len(objects)} large objects (>5 MiB)", file=sys.stderr)

    # Output JSON results
    output = {
        'timestamp': datetime.datetime.now().isoformat(),
        'size_threshold_bytes': SIZE_THRESHOLD,
        'size_threshold_mb': SIZE_THRESHOLD / (1024 * 1024),
        'buckets': results
    }

    print(json.dumps(output, indent=2))

    # Print summary
    total_objects = sum(b['count'] for b in results.values())
    print(f"\nTotal large objects found: {total_objects}", file=sys.stderr)


if __name__ == "__main__":
    main()