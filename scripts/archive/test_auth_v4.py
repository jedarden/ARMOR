#!/usr/bin/env python3
"""
Test ARMOR endpoint authentication with proper AWS Signature V4 signing.
This verifies that authentication works correctly with various scenarios.
"""

import sys
import os
import hashlib
import hmac
import datetime
import urllib.parse
import subprocess
from typing import Dict, Optional, Tuple

# Configuration
ARMOR_ENDPOINT = os.getenv("ARMOR_ENDPOINT", "http://localhost:9000")
ARMOR_ACCESS_KEY = os.getenv("ARMOR_ACCESS_KEY", "")
ARMOR_SECRET_KEY = os.getenv("ARMOR_SECRET_KEY", "")
ARMOR_BUCKET = os.getenv("ARMOR_BUCKET", "test-bucket")
ARMOR_REGION = os.getenv("ARMOR_REGION", "us-east-1")

# Colors for output
GREEN = "\033[0;32m"
RED = "\033[0;31m"
YELLOW = "\033[1;33m"
NC = "\033[0m"

class AWSSigV4Signer:
    """AWS Signature V4 signer"""

    def __init__(self, access_key: str, secret_key: str, region: str, service: str = "s3"):
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.service = service

    def _sign(self, key: bytes, msg: str) -> bytes:
        """HMAC SHA256 signing"""
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    def _get_signature_key(self, key: str, date_stamp: str, region_name: str, service_name: str) -> bytes:
        """Derive signature key"""
        k_date = self._sign(('AWS4' + key).encode('utf-8'), date_stamp)
        k_region = self._sign(k_date, region_name)
        k_service = self._sign(k_region, service_name)
        k_signing = self._sign(k_service, 'aws4_request')
        return k_signing

    def sign_request(
        self,
        method: str,
        host: str,
        endpoint: str,
        query_params: Optional[Dict[str, str]] = None,
        body: str = "",
        headers: Optional[Dict[str, str]] = None,
        timestamp: Optional[datetime.datetime] = None,
    ) -> Dict[str, str]:
        """
        Sign an HTTP request with AWS Signature V4.
        Returns the headers that should be added to the request.
        """
        if timestamp is None:
            timestamp = datetime.datetime.utcnow()

        amz_date = timestamp.strftime('%Y%m%dT%H%M%SZ')
        date_stamp = timestamp.strftime('%Y%m%d')

        # Parse endpoint
        parsed = urllib.parse.urlparse(endpoint)
        path = parsed.path or "/"

        # Build query string
        if query_params:
            encoded_params = []
            for k in sorted(query_params.keys()):
                encoded_params.append(f"{urllib.parse.quote(k, safe='')}/{urllib.parse.quote(str(query_params[k]), safe='')}")
            query_string = '&'.join(encoded_params)
        else:
            query_string = ""

        # Canonical headers
        canonical_headers = []
        signed_headers = []

        # Add custom headers first
        if headers:
            for k in sorted(headers.keys()):
                canonical_headers.append(f"{k.lower()}:{headers[k]}")
                signed_headers.append(k.lower())

        # Always add host and x-amz-date
        canonical_headers.append(f"host:{host}")
        signed_headers.append("host")
        canonical_headers.append(f"x-amz-date:{amz_date}")
        signed_headers.append("x-amz-date")

        canonical_headers_str = '\n'.join(canonical_headers) + '\n'
        signed_headers_str = ';'.join(signed_headers)

        # Payload hash
        payload_hash = hashlib.sha256(body.encode('utf-8')).hexdigest()

        # Canonical request
        canonical_request = '\n'.join([
            method,
            path,
            query_string,
            canonical_headers_str,
            signed_headers_str,
            payload_hash
        ])

        # String to sign
        credential_scope = f"{date_stamp}/{self.region}/{self.service}/aws4_request"
        canonical_request_hash = hashlib.sha256(canonical_request.encode('utf-8')).hexdigest()

        string_to_sign = '\n'.join([
            'AWS4-HMAC-SHA256',
            amz_date,
            credential_scope,
            canonical_request_hash
        ])

        # Calculate signature
        signing_key = self._get_signature_key(self.secret_key, date_stamp, self.region, self.service)
        signature = hmac.new(signing_key, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

        # Build authorization header
        authorization_header = (
            f"AWS4-HMAC-SHA256 "
            f"Credential={self.access_key}/{credential_scope}, "
            f"SignedHeaders={signed_headers_str}, "
            f"Signature={signature}"
        )

        # Return all headers
        result = headers.copy() if headers else {}
        result['Authorization'] = authorization_header
        result['X-Amz-Date'] = amz_date
        result['X-Amz-Content-Sha256'] = payload_hash

        return result


def curl_request(url: str, headers: Dict[str, str], method: str = "GET") -> Tuple[int, str]:
    """Make a curl request and return status code and body"""
    header_list = []
    for k, v in headers.items():
        header_list.extend(["-H", f"{k}: {v}"])

    try:
        result = subprocess.run(
            ["curl", "-s", "-i", "-X", method] + header_list + [url],
            capture_output=True,
            text=True,
            timeout=10
        )

        # Parse status code from response
        lines = result.stdout.split('\n')
        for line in lines:
            if line.startswith('HTTP/'):
                parts = line.split(' ')
                if len(parts) >= 3:
                    status_code = int(parts[1])
                    return status_code, result.stdout
        return -1, result.stdout
    except Exception as e:
        return -1, str(e)


def test_result(name: str, passed: bool, details: str = ""):
    """Print test result"""
    if passed:
        print(f"{GREEN}✓ PASS{NC}: {name}")
    else:
        print(f"{RED}✗ FAIL{NC}: {name}")
        if details:
            print(f"  {YELLOW}{details}{NC}")


def main():
    """Run all authentication tests"""
    print("=== ARMOR Authentication Test Suite ===")
    print()
    print(f"Endpoint: {ARMOR_ENDPOINT}")
    print(f"Bucket: {ARMOR_BUCKET}")
    print()

    # Check if server is running
    print("Checking if ARMOR server is running... ", end="", flush=True)
    try:
        code, _ = curl_request(f"{ARMOR_ENDPOINT}/healthz", {}, "GET")
        if code == 200:
            print(f"{GREEN}OK{NC}")
        else:
            print(f"{RED}FAILED (status {code}){NC}")
            print("ARMOR server is not responding correctly")
            sys.exit(1)
    except Exception as e:
        print(f"{RED}FAILED: {e}{NC}")
        print("Cannot connect to ARMOR server")
        sys.exit(1)

    print()
    print("Running authentication tests...")
    print()

    tests_passed = 0
    tests_failed = 0

    # Test 1: Missing authentication
    print("Test 1: Request without authentication")
    code, body = curl_request(f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key", {}, "GET")
    if code in (403, 401):
        test_result("No authentication header", True)
        tests_passed += 1
    else:
        test_result("No authentication header", False, f"Expected 403/401, got {code}")
        tests_failed += 1
    print()

    # Test 2: Wrong access key
    print("Test 2: Invalid access key")
    try:
        signer = AWSSigV4Signer("WRONG_ACCESS_KEY", "wrong_secret", ARMOR_REGION)
        url = f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key"
        headers = signer.sign_request("GET", "localhost:9000", url)
        code, body = curl_request(url, headers, "GET")
        if code == 403:
            test_result("Invalid access key rejected", True)
            tests_passed += 1
        else:
            test_result("Invalid access key rejected", False, f"Expected 403, got {code}")
            tests_failed += 1
    except Exception as e:
        test_result("Invalid access key rejected", False, str(e))
        tests_failed += 1
    print()

    # Test 3: Wrong signature (right key, wrong secret)
    print("Test 3: Invalid signature")
    try:
        if ARMOR_ACCESS_KEY and ARMOR_SECRET_KEY:
            signer = AWSSigV4Signer(ARMOR_ACCESS_KEY, "wrong_secret", ARMOR_REGION)
            url = f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key"
            headers = signer.sign_request("GET", "localhost:9000", url)
            code, body = curl_request(url, headers, "GET")
            if code == 403:
                test_result("Invalid signature rejected", True)
                tests_passed += 1
            else:
                test_result("Invalid signature rejected", False, f"Expected 403, got {code}")
                tests_failed += 1
        else:
            print(f"{YELLOW}Skipping - ARMOR_ACCESS_KEY or ARMOR_SECRET_KEY not set{NC}")
    except Exception as e:
        test_result("Invalid signature rejected", False, str(e))
        tests_failed += 1
    print()

    # Test 4: Expired timestamp
    print("Test 4: Expired request timestamp")
    try:
        if ARMOR_ACCESS_KEY and ARMOR_SECRET_KEY:
            # Create a timestamp 20 minutes in the past
            old_timestamp = datetime.datetime.utcnow() - datetime.timedelta(minutes=20)
            signer = AWSSigV4Signer(ARMOR_ACCESS_KEY, ARMOR_SECRET_KEY, ARMOR_REGION)
            url = f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key"
            headers = signer.sign_request("GET", "localhost:9000", url, timestamp=old_timestamp)
            code, body = curl_request(url, headers, "GET")
            if code == 403:
                test_result("Expired timestamp rejected", True)
                tests_passed += 1
            else:
                test_result("Expired timestamp rejected", False, f"Expected 403, got {code}")
                tests_failed += 1
        else:
            print(f"{YELLOW}Skipping - ARMOR_ACCESS_KEY or ARMOR_SECRET_KEY not set{NC}")
    except Exception as e:
        test_result("Expired timestamp rejected", False, str(e))
        tests_failed += 1
    print()

    # Test 5: Future timestamp
    print("Test 5: Future request timestamp")
    try:
        if ARMOR_ACCESS_KEY and ARMOR_SECRET_KEY:
            # Create a timestamp 20 minutes in the future
            future_timestamp = datetime.datetime.utcnow() + datetime.timedelta(minutes=20)
            signer = AWSSigV4Signer(ARMOR_ACCESS_KEY, ARMOR_SECRET_KEY, ARMOR_REGION)
            url = f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key"
            headers = signer.sign_request("GET", "localhost:9000", url, timestamp=future_timestamp)
            code, body = curl_request(url, headers, "GET")
            if code == 403:
                test_result("Future timestamp rejected", True)
                tests_passed += 1
            else:
                test_result("Future timestamp rejected", False, f"Expected 403, got {code}")
                tests_failed += 1
        else:
            print(f"{YELLOW}Skipping - ARMOR_ACCESS_KEY or ARMOR_SECRET_KEY not set{NC}")
    except Exception as e:
        test_result("Future timestamp rejected", False, str(e))
        tests_failed += 1
    print()

    # Test 6: Valid authentication
    print("Test 6: Valid authentication")
    try:
        if ARMOR_ACCESS_KEY and ARMOR_SECRET_KEY:
            signer = AWSSigV4Signer(ARMOR_ACCESS_KEY, ARMOR_SECRET_KEY, ARMOR_REGION)
            url = f"{ARMOR_ENDPOINT}/{ARMOR_BUCKET}/test-key"
            headers = signer.sign_request("GET", "localhost:9000", url)
            code, body = curl_request(url, headers, "GET")
            # We expect either 404 (key doesn't exist) or 200 (it exists), but NOT 403 (auth error)
            if code in (200, 404):
                test_result("Valid authentication accepted", True)
                tests_passed += 1
            else:
                test_result("Valid authentication accepted", False, f"Expected 200/404, got {code}")
                tests_failed += 1
        else:
            print(f"{YELLOW}Skipping - ARMOR_ACCESS_KEY or ARMOR_SECRET_KEY not set{NC}")
    except Exception as e:
        test_result("Valid authentication accepted", False, str(e))
        tests_failed += 1
    print()

    # Test 7: Public endpoints
    print("Test 7: Public endpoints")
    code, _ = curl_request(f"{ARMOR_ENDPOINT}/healthz", {}, "GET")
    if code == 200:
        test_result("Health endpoint is public", True)
        tests_passed += 1
    else:
        test_result("Health endpoint is public", False, f"Expected 200, got {code}")
        tests_failed += 1

    code, _ = curl_request(f"{ARMOR_ENDPOINT}/readyz", {}, "GET")
    if code in (200, 503):
        test_result("Ready endpoint is public", True)
        tests_passed += 1
    else:
        test_result("Ready endpoint is public", False, f"Expected 200/503, got {code}")
        tests_failed += 1
    print()

    # Summary
    print("=== Test Summary ===")
    print(f"Tests Passed: {GREEN}{tests_passed}{NC}")
    print(f"Tests Failed: {RED}{tests_failed}{NC}")
    print()

    if tests_failed == 0:
        print(f"{GREEN}All authentication tests passed!{NC}")
        sys.exit(0)
    else:
        print(f"{RED}Some authentication tests failed!{NC}")
        sys.exit(1)


if __name__ == "__main__":
    main()
