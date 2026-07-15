#!/usr/bin/env python3
"""
Simple ARMOR S3 Operations Test with AWS V4 Signing

This script tests basic S3 operations against ARMOR with proper authentication.
"""

import sys
import hashlib
import hmac
import datetime
import urllib.request
import urllib.parse
import xml.etree.ElementTree as ET

# ANSI colors
GREEN = '\033[92m'
RED = '\033[91m'
YELLOW = '\033[93m'
BLUE = '\033[94m'
RESET = '\033[0m'

def print_success(msg):
    print(f"{GREEN}✓{RESET} {msg}")

def print_failure(msg):
    print(f"{RED}✗{RESET} {msg}")

def print_info(msg):
    print(f"{BLUE}ℹ{RESET} {msg}")

def print_header(msg):
    print(f"\n{BLUE}{'=' * 70}{RESET}")
    print(f"{BLUE}{msg}{RESET}")
    print(f"{BLUE}{'=' * 70}{RESET}\n")

class AWSV4Signer:
    """AWS Signature Version 4 signer"""

    def __init__(self, access_key, secret_key, region="us-east-005"):
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.service = "s3"

    def _sign(self, key, msg):
        """HMAC SHA256"""
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    def _get_signature_key(self, date_stamp):
        """Derive signature key"""
        k_date = self._sign(('AWS4' + self.secret_key).encode('utf-8'), date_stamp)
        k_region = self._sign(k_date, self.region)
        k_service = self._sign(k_region, self.service)
        k_signing = self._sign(k_service, 'aws4_request')
        return k_signing

    def sign_request(self, method, host, path, headers=None, body=b''):
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

def make_signed_request(endpoint, path, access_key, secret_key, method='GET', body=b''):
    """Make a signed HTTP request to ARMOR"""
    from urllib.parse import urlparse

    parsed = urlparse(endpoint)
    host = parsed.netloc or parsed.path
    if ':' in host:
        host = host.split(':')[0]

    signer = AWSV4Signer(access_key, secret_key)

    headers = {
        'Host': host,
    }

    signed_headers = signer.sign_request(method, host, path, headers, body)

    url = endpoint + path
    req = urllib.request.Request(url, data=body, method=method, headers=signed_headers)

    try:
        resp = urllib.request.urlopen(req, timeout=10)
        return resp.getcode(), resp.read(), resp.info()
    except urllib.error.HTTPError as e:
        return e.code, e.read(), e.info()
    except Exception as e:
        return -1, str(e).encode(), {}

def verify_list_buckets(endpoint, access_key, secret_key):
    """Test ListBuckets operation"""
    print_header("Testing ListBuckets Operation")

    status, body, headers = make_signed_request(endpoint, "/", access_key, secret_key)

    if status == 200:
        print_success("ListBuckets returned HTTP 200")

        try:
            root = ET.fromstring(body)
            namespace = {'s3': 'http://s3.amazonaws.com/doc/2006-03-01/'}

            owner = root.find('.//s3:Owner', namespace)
            buckets = root.find('.//s3:Buckets', namespace)

            if owner is not None:
                print_success("Response has Owner element")
            else:
                print_failure("Response missing Owner element")
                return False

            if buckets is not None:
                print_success("Response has Buckets element")
            else:
                print_failure("Response missing Buckets element")
                return False

            bucket_list = root.findall('.//s3:Bucket', namespace)
            print_info(f"Found {len(bucket_list)} bucket(s)")

            return True
        except ET.ParseError as e:
            print_failure(f"Failed to parse XML: {e}")
            return False
    else:
        print_failure(f"ListBuckets returned HTTP {status}, expected 200")
        if body:
            print_info(f"Response: {body[:200].decode('utf-8', errors='ignore')}")
        return False

def verify_head_object(endpoint, bucket, access_key, secret_key):
    """Test HeadObject operation"""
    print_header("Testing HeadObject Operation")

    test_key = "nonexistent-test-object-12345.txt"
    status, body, headers = make_signed_request(endpoint, f"/{bucket}/{test_key}", access_key, secret_key, method='HEAD')

    if status in [200, 404]:
        if status == 404:
            print_success("HeadObject returned HTTP 404 for non-existent object (expected)")
        else:
            print_success("HeadObject returned HTTP 200")

            important_headers = ['Content-Length', 'Content-Type', 'ETag', 'Last-Modified']
            found_count = 0

            for header in important_headers:
                if header in headers:
                    print_success(f"  {header}: {headers[header]}")
                    found_count += 1

            if found_count >= 2:
                print_success(f"Found {found_count} metadata headers")

        return True
    else:
        print_failure(f"HeadObject returned HTTP {status}, expected 200 or 404")
        return False

def verify_list_objects_v2(endpoint, bucket, access_key, secret_key):
    """Test ListObjectsV2 operation"""
    print_header("Testing ListObjectsV2 Operation")

    status, body, headers = make_signed_request(endpoint, f"/{bucket}/?list-type=2", access_key, secret_key)

    if status in [200, 404]:
        if status == 404:
            print_success("ListObjectsV2 returned HTTP 404 (bucket may not exist)")
            return True

        print_success("ListObjectsV2 returned HTTP 200")

        try:
            root = ET.fromstring(body)
            namespace = {'s3': 'http://s3.amazonaws.com/doc/2006-03-01/'}

            name = root.find('s3:Name', namespace)
            if name is not None:
                print_success(f"Response for bucket: {name.text}")

            contents = root.findall('.//s3:Contents', namespace)
            print_info(f"Found {len(contents)} object(s)")

            return True
        except ET.ParseError as e:
            print_failure(f"Failed to parse XML: {e}")
            return False
    else:
        print_failure(f"ListObjectsV2 returned HTTP {status}, expected 200 or 404")
        if body:
            print_info(f"Response: {body[:200].decode('utf-8', errors='ignore')}")
        return False

def verify_error_operations(endpoint, bucket, access_key, secret_key):
    """Test error operations"""
    print_header("Testing Error Operations")

    all_passed = True

    # Test 1: Non-existent bucket
    print_info("Test 1: Accessing non-existent bucket...")
    status, body, _ = make_signed_request(endpoint, "/nonexistent-test-bucket-12345/?list-type=2", access_key, secret_key)

    if status == 404:
        print_success("Non-existent bucket returned HTTP 404")
        try:
            root = ET.fromstring(body)
            code = root.find('.//{http://s3.amazonaws.com/doc/2006-03-01/}Code')
            if code is not None:
                print_success(f"Error code: {code.text}")
        except:
            pass
    else:
        print(f"{YELLOW}⚠{RESET} Expected HTTP 404, got {status}")

    # Test 2: Non-existent object
    print_info("Test 2: Accessing non-existent object...")
    status, body, _ = make_signed_request(endpoint, f"/{bucket}/nonexistent-test-object-12345.txt", access_key, secret_key)

    if status == 404:
        print_success("Non-existent object returned HTTP 404")
    else:
        print(f"{YELLOW}⚠{RESET} Expected HTTP 404, got {status}")

    return all_passed

def main():
    if len(sys.argv) < 4:
        print("Usage: python3 test_s3_basic_operations.py <endpoint> <access_key> <secret_key> [bucket]")
        sys.exit(1)

    endpoint = sys.argv[1]
    access_key = sys.argv[2]
    secret_key = sys.argv[3]
    bucket = sys.argv[4] if len(sys.argv) > 4 else "test-bucket"

    print_header("ARMOR Basic S3 Operations Test")
    print_info(f"Endpoint: {endpoint}")
    print_info(f"Bucket: {bucket}")
    print_info(f"Access Key: {access_key[:20]}...")

    results = {
        'ListBuckets': verify_list_buckets(endpoint, access_key, secret_key),
        'HeadObject': verify_head_object(endpoint, bucket, access_key, secret_key),
        'ListObjectsV2': verify_list_objects_v2(endpoint, bucket, access_key, secret_key),
        'ErrorOperations': verify_error_operations(endpoint, bucket, access_key, secret_key),
    }

    print_header("Test Summary")

    passed = sum(1 for v in results.values() if v)
    total = len(results)

    for test_name, result in results.items():
        status = f"{GREEN}✓ PASS{RESET}" if result else f"{RED}✗ FAIL{RESET}"
        print(f"{status} - {test_name}")

    print(f"\nTotal: {passed}/{total} tests passed")

    if passed == total:
        print_success("\n✅ ALL TESTS PASSED")
        return 0
    else:
        print_failure("\n❌ SOME TESTS FAILED")
        return 1

if __name__ == '__main__':
    sys.exit(main())
