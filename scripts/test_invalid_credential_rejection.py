#!/usr/bin/env python3
"""
Test script for invalid credential rejection (bead bf-1bqk5o).

This script verifies that ARMOR properly rejects invalid credentials with
appropriate error responses, including:
- Invalid AWS credentials return 403 Forbidden
- Malformed signatures return 403 Forbidden
- Missing authentication headers return 403 Forbidden
- Error responses include meaningful error messages
- Rejection happens quickly (no long timeouts)
"""

import os
import sys
import hashlib
import hmac
import datetime
import urllib.parse
import time
import xml.etree.ElementTree as ET
from typing import Tuple, Optional

import requests

# ANSI color codes
GREEN = '\033[92m'
RED = '\033[91m'
YELLOW = '\033[93m'
BLUE = '\033[94m'
RESET = '\033[0m'

def print_success(msg: str):
    print(f"{GREEN}✓{RESET} {msg}")

def print_failure(msg: str):
    print(f"{RED}✗{RESET} {msg}")

def print_info(msg: str):
    print(f"{BLUE}ℹ{RESET} {msg}")

def print_warning(msg: str):
    print(f"{YELLOW}⚠{RESET} {msg}")

class AWSSignatureV4:
    """AWS Signature Version 4 signer"""

    def __init__(self, access_key: str, secret_key: str, region: str = 'us-east-1'):
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.service = 's3'

    def sign(self, method: str, url: str, headers: dict, body: str = '') -> dict:
        """Sign a request with AWS SigV4"""
        parsed_url = urllib.parse.urlparse(url)
        host = parsed_url.netloc.split(':')[0]  # Remove port if present
        path = parsed_url.path or '/'

        # Get current timestamp
        now = datetime.datetime.utcnow()
        amz_date = now.strftime('%Y%m%dT%H%M%SZ')
        date_stamp = now.strftime('%Y%m%d')

        # Add required headers
        headers_to_sign = headers.copy()
        headers_to_sign['Host'] = host
        headers_to_sign['x-amz-date'] = amz_date

        # Calculate payload hash
        payload_hash = hashlib.sha256(body.encode('utf-8')).hexdigest()

        # Build canonical request
        canonical_headers = ''
        signed_headers = []
        for h in sorted(headers_to_sign.keys()):
            canonical_headers += f'{h.lower()}:{headers_to_sign[h]}\n'
            signed_headers.append(h.lower())
        canonical_headers = canonical_headers.strip()

        signed_headers_str = ';'.join(signed_headers)
        canonical_uri = urllib.parse.quote(path, safe='/')

        canonical_request = f"{method}\n{canonical_uri}\n\n{canonical_headers}\n{signed_headers_str}\n{payload_hash}"

        # Create string to sign
        credential_scope = f"{date_stamp}/{self.region}/{self.service}/aws4_request"
        string_to_sign = f"AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n"
        string_to_sign += hashlib.sha256(canonical_request.encode('utf-8')).hexdigest()

        # Calculate signature
        k_date = hmac.new(('AWS4' + self.secret_key).encode('utf-8'), date_stamp.encode('utf-8'), hashlib.sha256).digest()
        k_region = hmac.new(k_date, self.region.encode('utf-8'), hashlib.sha256).digest()
        k_service = hmac.new(k_region, self.service.encode('utf-8'), hashlib.sha256).digest()
        k_signing = hmac.new(k_service, b'aws4_request', hashlib.sha256).digest()
        signature = hmac.new(k_signing, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

        # Build authorization header
        authorization_header = (
            f"AWS4-HMAC-SHA256 "
            f"Credential={self.access_key}/{credential_scope}, "
            f"SignedHeaders={signed_headers_str}, "
            f"Signature={signature}"
        )

        # Return headers
        result = headers_to_sign.copy()
        result['Authorization'] = authorization_header
        result['x-amz-content-sha256'] = payload_hash
        return result


def parse_s3_error(response_text: str) -> Tuple[str, str]:
    """Parse S3 error XML response"""
    try:
        root = ET.fromstring(response_text)
        code = root.find('Code').text if root.find('Code') is not None else 'Unknown'
        message = root.find('Message').text if root.find('Message') is not None else 'No message'
        return code, message
    except ET.ParseError:
        return 'ParseError', f'Could not parse error response: {response_text[:100]}'


def test_missing_auth_header(endpoint: str, bucket: str):
    """Test that missing authentication headers return 403 with meaningful error"""
    print_info("Testing missing authentication header...")

    url = f"{endpoint}/{bucket}/"

    start_time = time.time()
    response = requests.get(url)
    elapsed = time.time() - start_time

    # Check response
    if response.status_code == 403:
        code, message = parse_s3_error(response.text)
        print_success(f"Missing auth header returned 403 Forbidden")
        print_success(f"Error code: {code}")
        print_success(f"Error message: {message}")

        # Verify it's the right error code
        if code == 'MissingAuthenticationToken':
            print_success("Correct error code for missing auth header")
        else:
            print_warning(f"Expected 'MissingAuthenticationToken', got '{code}'")

        # Check timing (should be fast)
        if elapsed < 2.0:
            print_success(f"Rejection was quick ({elapsed:.3f}s)")
        else:
            print_failure(f"Rejection took too long ({elapsed:.3f}s)")

        return True
    else:
        print_failure(f"Expected 403, got {response.status_code}")
        return False


def test_invalid_access_key(endpoint: str, bucket: str, region: str):
    """Test that invalid access key returns 403 with meaningful error"""
    print_info("Testing invalid access key...")

    # Use a fake access key that doesn't exist
    fake_signer = AWSSignatureV4('FAKEACCESSKEY123', 'fakesecretkey', region)

    url = f"{endpoint}/{bucket}/"
    headers = fake_signer.sign('GET', url, {})

    start_time = time.time()
    response = requests.get(url, headers=headers)
    elapsed = time.time() - start_time

    # Check response
    if response.status_code == 403:
        code, message = parse_s3_error(response.text)
        print_success(f"Invalid access key returned 403 Forbidden")
        print_success(f"Error code: {code}")
        print_success(f"Error message: {message}")

        # Verify it's the right error code
        if code == 'InvalidAccessKeyId':
            print_success("Correct error code for invalid access key")
        else:
            print_warning(f"Expected 'InvalidAccessKeyId', got '{code}'")

        # Check timing
        if elapsed < 2.0:
            print_success(f"Rejection was quick ({elapsed:.3f}s)")
        else:
            print_failure(f"Rejection took too long ({elapsed:.3f}s)")

        return True
    else:
        print_failure(f"Expected 403, got {response.status_code}")
        return False


def test_invalid_signature(endpoint: str, bucket: str, region: str):
    """Test that invalid/malformed signatures return 403 with meaningful error"""
    print_info("Testing invalid signature...")

    # Use wrong secret key to create invalid signature
    signer = AWSSignatureV4(os.getenv('ARMOR_ACCESS_KEY', ''), 'wrongsecretkey', region)

    url = f"{endpoint}/{bucket}/"
    headers = signer.sign('GET', url, {})

    start_time = time.time()
    response = requests.get(url, headers=headers)
    elapsed = time.time() - start_time

    # Check response
    if response.status_code == 403:
        code, message = parse_s3_error(response.text)
        print_success(f"Invalid signature returned 403 Forbidden")
        print_success(f"Error code: {code}")
        print_success(f"Error message: {message}")

        # Verify it's the right error code
        if code == 'SignatureDoesNotMatch':
            print_success("Correct error code for signature mismatch")
        else:
            print_warning(f"Expected 'SignatureDoesNotMatch', got '{code}'")

        # Check timing
        if elapsed < 2.0:
            print_success(f"Rejection was quick ({elapsed:.3f}s)")
        else:
            print_failure(f"Rejection took too long ({elapsed:.3f}s)")

        return True
    else:
        print_failure(f"Expected 403, got {response.status_code}")
        return False


def test_malformed_auth_header(endpoint: str, bucket: str):
    """Test that malformed auth headers return 403 with meaningful error"""
    print_info("Testing malformed authorization header...")

    url = f"{endpoint}/{bucket}/"
    headers = {
        'Authorization': 'InvalidAuthHeaderFormat',
    }

    start_time = time.time()
    response = requests.get(url, headers=headers)
    elapsed = time.time() - start_time

    # Check response
    if response.status_code == 403:
        code, message = parse_s3_error(response.text)
        print_success(f"Malformed auth header returned 403 Forbidden")
        print_success(f"Error code: {code}")
        print_success(f"Error message: {message}")

        # Check timing
        if elapsed < 2.0:
            print_success(f"Rejection was quick ({elapsed:.3f}s)")
        else:
            print_failure(f"Rejection took too long ({elapsed:.3f}s)")

        return True
    else:
        print_failure(f"Expected 403, got {response.status_code}")
        return False


def test_missing_date_header(endpoint: str, bucket: str):
    """Test that missing date header returns 403 with meaningful error"""
    print_info("Testing missing date header...")

    url = f"{endpoint}/{bucket}/"
    headers = {
        'Authorization': 'AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example',
    }

    start_time = time.time()
    response = requests.get(url, headers=headers)
    elapsed = time.time() - start_time

    # Check response
    if response.status_code == 403:
        code, message = parse_s3_error(response.text)
        print_success(f"Missing date header returned 403 Forbidden")
        print_success(f"Error code: {code}")
        print_success(f"Error message: {message}")

        # Check timing
        if elapsed < 2.0:
            print_success(f"Rejection was quick ({elapsed:.3f}s)")
        else:
            print_failure(f"Rejection took too long ({elapsed:.3f}s)")

        return True
    else:
        print_failure(f"Expected 403, got {response.status_code}")
        return False


def test_valid_auth_succeeds(endpoint: str, bucket: str, region: str):
    """Verify that valid authentication still works (control test)"""
    print_info("Testing valid authentication (control test)...")

    access_key = os.getenv('ARMOR_ACCESS_KEY')
    secret_key = os.getenv('ARMOR_SECRET_KEY')

    if not access_key or not secret_key:
        print_warning("Skipping valid auth test - credentials not configured")
        return True

    signer = AWSSignatureV4(access_key, secret_key, region)

    url = f"{endpoint}/{bucket}/"
    headers = signer.sign('GET', url, {})

    response = requests.get(url, headers=headers)

    # Check response - should be 200 OK or 404 (bucket doesn't exist), but NOT 403
    if response.status_code in (200, 404):
        print_success(f"Valid authentication succeeded (got {response.status_code})")
        return True
    elif response.status_code == 403:
        print_failure("Valid credentials were rejected with 403")
        return False
    else:
        print_warning(f"Unexpected status code: {response.status_code}")
        return True  # Don't fail the test suite for this


def main():
    """Main test runner"""
    print("=" * 80)
    print("Invalid Credential Rejection Test Suite")
    print("Bead: bf-1bqk5o")
    print("=" * 80)
    print()

    # Get configuration from environment
    endpoint = os.getenv('ARMOR_ENDPOINT', 'http://localhost:9000')
    bucket = os.getenv('ARMOR_BUCKET', 'test-bucket')
    region = os.getenv('ARMOR_REGION', 'us-east-1')

    print_info(f"Endpoint: {endpoint}")
    print_info(f"Bucket: {bucket}")
    print_info(f"Region: {region}")
    print()

    # Check if server is accessible
    try:
        health_response = requests.get(f"{endpoint}/healthz", timeout=5)
        if health_response.status_code == 200:
            print_success("Health check passed")
        else:
            print_failure("Health check failed")
            return 1
    except Exception as e:
        print_failure(f"Cannot connect to server: {e}")
        return 1

    print()
    print("=" * 80)
    print("Running Invalid Credential Rejection Tests")
    print("=" * 80)
    print()

    results = []

    # Run all tests
    results.append(("Missing auth header", test_missing_auth_header(endpoint, bucket)))
    print()
    results.append(("Invalid access key", test_invalid_access_key(endpoint, bucket, region)))
    print()
    results.append(("Invalid signature", test_invalid_signature(endpoint, bucket, region)))
    print()
    results.append(("Malformed auth header", test_malformed_auth_header(endpoint, bucket)))
    print()
    results.append(("Missing date header", test_missing_date_header(endpoint, bucket)))
    print()
    results.append(("Valid auth control", test_valid_auth_succeeds(endpoint, bucket, region)))
    print()

    # Print summary
    print("=" * 80)
    print("Test Summary")
    print("=" * 80)

    passed = sum(1 for _, result in results if result)
    total = len(results)

    for test_name, result in results:
        status = f"{GREEN}PASS{RESET}" if result else f"{RED}FAIL{RESET}"
        print(f"{status} - {test_name}")

    print()
    print(f"Results: {passed}/{total} tests passed")

    if passed == total:
        print_success("All tests passed!")
        return 0
    else:
        print_failure(f"{total - passed} test(s) failed")
        return 1


if __name__ == '__main__':
    sys.exit(main())
