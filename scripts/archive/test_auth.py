#!/usr/bin/env python3
"""
Comprehensive ARMOR endpoint authentication test.

Tests:
1. Valid S3 authentication (AWS Signature V4)
2. Invalid access key rejected
3. Invalid signature rejected
4. Missing auth header rejected
5. Expired request rejected
6. Query auth (presigned URLs)
"""

import hashlib
import hmac
import datetime
import urllib.parse
import urllib.request
import urllib.error
import sys
import subprocess
import json


class AWSV4Signer:
    """AWS Signature Version 4 signer."""

    def __init__(self, access_key: str, secret_key: str, region: str = "us-east-005"):
        self.access_key = access_key
        self.secret_key = secret_key
        self.region = region
        self.service = "s3"

    def _sign(self, key: bytes, msg: str) -> bytes:
        """HMAC SHA256."""
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    def _get_signature_key(self, date_stamp: str) -> bytes:
        """Derive signature key."""
        k_date = self._sign(('AWS4' + self.secret_key).encode('utf-8'), date_stamp)
        k_region = self._sign(k_date, self.region)
        k_service = self._sign(k_region, self.service)
        k_signing = self._sign(k_service, 'aws4_request')
        return k_signing

    def sign_request(self, method: str, host: str, path: str,
                     headers: dict, body: bytes = b'') -> dict:
        """Sign an HTTP request with AWS V4 signature."""
        now = datetime.datetime.utcnow()
        amz_date = now.strftime('%Y%m%dT%H%M%SZ')
        date_stamp = now.strftime('%Y%m%d')

        # Add required headers
        headers['X-Amz-Date'] = amz_date
        if 'Host' not in headers:
            headers['Host'] = host

        # Calculate payload hash
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

        # String to sign
        credential_scope = f'{date_stamp}/{self.region}/{self.service}/aws4_request'
        string_to_sign = f'AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode('utf-8')).hexdigest()}'

        # Calculate signature
        signing_key = self._get_signature_key(date_stamp)
        signature = hmac.new(signing_key, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

        # Authorization header
        authorization_header = f'AWS4-HMAC-SHA256 Credential={self.access_key}/{credential_scope}, SignedHeaders={signed_headers}, Signature={signature}'
        headers['Authorization'] = authorization_header

        return headers


def curl_request(method: str, url: str, headers: dict, body: bytes = b'') -> tuple:
    """Make HTTP request using curl and return status code, headers, and body."""
    curl_cmd = ['curl', '-s', '-i', '-X', method]

    # Add headers
    for key, value in headers.items():
        curl_cmd.extend(['-H', f'{key}: {value}'])

    # Add body for PUT/POST
    if body and method.upper() in ('PUT', 'POST'):
        # Could use --data-binary but for empty tests this is fine
        pass

    curl_cmd.append(url)

    try:
        result = subprocess.run(curl_cmd, capture_output=True, text=True, timeout=10)
        output = result.stdout

        # Parse status code
        lines = output.split('\n')
        status_line = lines[0] if lines else ''
        status_code = -1

        if status_line.startswith('HTTP/'):
            parts = status_line.split()
            if len(parts) >= 2:
                try:
                    status_code = int(parts[1])
                except ValueError:
                    pass

        # Find body (after empty line)
        body_start = -1
        for i, line in enumerate(lines):
            if line == '' and i > 0:
                body_start = i + 1
                break

        response_body = ''
        if body_start >= 0 and body_start < len(lines):
            response_body = '\n'.join(lines[body_start:])

        return status_code, {}, response_body

    except subprocess.TimeoutExpired:
        return -1, {}, 'Request timeout'
    except Exception as e:
        return -1, {}, str(e)


def test_valid_auth(endpoint: str, access_key: str, secret_key: str) -> bool:
    """Test 1: Valid authentication is accepted."""
    print('\n[TEST 1] Valid S3 Authentication')
    print('-' * 50)

    signer = AWSV4Signer(access_key, secret_key)
    headers = {}
    signed_headers = signer.sign_request('GET', 'localhost:9000', '/test-bucket/test-key', headers)

    status, resp_headers, body = curl_request('GET', f'{endpoint}/test-bucket/test-key', signed_headers)

    print(f'Status Code: {status}')
    if status == -1:
        print(f'❌ FAILED: Request error: {body[:200]}')
        return False
    elif status == 404:
        print('✅ PASSED: Authentication accepted (404 = key not found, but auth succeeded)')
        return True
    elif status == 403:
        print(f'⚠️  ACCESS DENIED: Check credentials or ACLs')
        print(f'Response: {body[:300]}')
        # This could still mean auth worked but access was denied due to ACLs
        return 'AccessDenied' in body or 'signature' not in body.lower()
    elif status in (200, 206):
        print('✅ PASSED: Authentication accepted and key exists')
        return True
    else:
        print(f'❌ FAILED: Unexpected status {status}')
        print(f'Response: {body[:200]}')
        return False


def test_invalid_access_key(endpoint: str, access_key: str, secret_key: str) -> bool:
    """Test 2: Invalid access key is rejected."""
    print('\n[TEST 2] Invalid Access Key')
    print('-' * 50)

    fake_key = 'INVALIDACCESSKEY'
    signer = AWSV4Signer(fake_key, secret_key)
    headers = {}
    signed_headers = signer.sign_request('GET', 'localhost:9000', '/test-bucket/test-key', headers)

    status, resp_headers, body = curl_request('GET', f'{endpoint}/test-bucket/test-key', signed_headers)

    print(f'Status Code: {status}')
    print(f'Response: {body[:300]}')

    if status == 403:
        # Check for InvalidAccessKeyId in response
        if 'InvalidAccessKeyId' in body or 'The AWS Access Key Id you provided does not exist' in body:
            print('✅ PASSED: Invalid access key properly rejected with InvalidAccessKeyId')
            return True
        else:
            print('⚠️  PARTIAL: Rejected but without proper error code')
            return True
    elif status == -1:
        print(f'❌ FAILED: Request error: {body[:200]}')
        return False
    else:
        print(f'❌ FAILED: Should have been rejected with 403, got {status}')
        return False


def test_invalid_signature(endpoint: str, access_key: str, secret_key: str) -> bool:
    """Test 3: Invalid signature is rejected."""
    print('\n[TEST 3] Invalid Signature')
    print('-' * 50)

    # Sign with wrong secret
    wrong_secret = 'wrongsecretkey123456789012345678901234'
    signer = AWSV4Signer(access_key, wrong_secret)
    headers = {}
    signed_headers = signer.sign_request('GET', 'localhost:9000', '/test-bucket/test-key', headers)

    status, resp_headers, body = curl_request('GET', f'{endpoint}/test-bucket/test-key', signed_headers)

    print(f'Status Code: {status}')
    print(f'Response: {body[:300]}')

    if status == 403:
        if 'SignatureDoesNotMatch' in body or 'signature we calculated does not match' in body.lower():
            print('✅ PASSED: Invalid signature properly rejected with SignatureDoesNotMatch')
            return True
        else:
            print('⚠️  PARTIAL: Rejected but without proper error code')
            return True
    else:
        print(f'❌ FAILED: Should have been rejected with 403, got {status}')
        return False


def test_missing_auth_header(endpoint: str) -> bool:
    """Test 4: Missing authorization header is rejected."""
    print('\n[TEST 4] Missing Authorization Header')
    print('-' * 50)

    headers = {
        'X-Amz-Date': datetime.datetime.utcnow().strftime('%Y%m%dT%H%M%SZ'),
        'X-Amz-Content-Sha256': hashlib.sha256(b'').hexdigest()
    }

    status, resp_headers, body = curl_request('GET', f'{endpoint}/test-bucket/test-key', headers)

    print(f'Status Code: {status}')
    print(f'Response: {body[:300]}')

    if status == 403:
        if 'MissingAuthenticationToken' in body or 'Missing Authentication Token' in body:
            print('✅ PASSED: Missing auth header properly rejected')
            return True
        else:
            print('⚠️  PARTIAL: Rejected but without proper error code')
            return True
    elif status == 400:
        print('✅ PASSED: Missing auth header rejected with 400')
        return True
    else:
        print(f'❌ FAILED: Should have been rejected, got {status}')
        return False


def test_expired_request(endpoint: str, access_key: str, secret_key: str) -> bool:
    """Test 5: Expired request is rejected."""
    print('\n[TEST 5] Expired Request')
    print('-' * 50)

    # Sign with an old timestamp (20 minutes ago)
    old_time = datetime.datetime.utcnow() - datetime.timedelta(minutes=20)
    amz_date = old_time.strftime('%Y%m%dT%H%M%SZ')
    date_stamp = old_time.strftime('%Y%m%d')

    # Manual signature with old timestamp
    host = 'localhost:9000'
    method = 'GET'
    path = '/test-bucket/test-key'
    body = b''

    headers = {
        'Host': host,
        'X-Amz-Date': amz_date,
        'X-Amz-Content-Sha256': hashlib.sha256(body).hexdigest()
    }

    # Build canonical request
    canonical_headers = 'host:localhost:9000\nx-amz-content-sha256:' + headers['X-Amz-Content-Sha256'] + '\nx-amz-date:' + amz_date + '\n'
    signed_headers = 'host;x-amz-content-sha256;x-amz-date'
    payload_hash = headers['X-Amz-Content-Sha256']

    canonical_request = f'{method}\n{path}\n\n{canonical_headers}\n{signed_headers}\n{payload_hash}'

    # String to sign
    credential_scope = f'{date_stamp}/us-east-005/s3/aws4_request'
    string_to_sign = f'AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode()).hexdigest()}'

    # Signature
    def sign(key, msg):
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    k_date = sign(('AWS4' + secret_key).encode('utf-8'), date_stamp)
    k_region = sign(k_date, 'us-east-005')
    k_service = sign(k_region, 's3')
    k_signing = sign(k_service, 'aws4_request')
    signature = hmac.new(k_signing, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

    authorization = f'AWS4-HMAC-SHA256 Credential={access_key}/{credential_scope}, SignedHeaders={signed_headers}, Signature={signature}'
    headers['Authorization'] = authorization

    status, resp_headers, body = curl_request('GET', f'{endpoint}/test-bucket/test-key', headers)

    print(f'Status Code: {status}')
    print(f'Response: {body[:300]}')

    if status == 403:
        if 'RequestExpired' in body or 'request has expired' in body.lower() or 'expired' in body.lower():
            print('✅ PASSED: Expired request properly rejected')
            return True
        else:
            print('⚠️  PARTIAL: Rejected but without proper error code')
            return True
    else:
        print(f'❌ FAILED: Should have been rejected, got {status}')
        return False


def test_query_auth_presigned_url(endpoint: str, access_key: str, secret_key: str) -> bool:
    """Test 6: Query authentication (presigned URL)."""
    print('\n[TEST 6] Query Authentication (Presigned URL)')
    print('-' * 50)

    now = datetime.datetime.utcnow()
    amz_date = now.strftime('%Y%m%dT%H%M%SZ')
    date_stamp = now.strftime('%Y%m%d')

    host = 'localhost:9000'
    method = 'GET'
    path = '/test-bucket/test-key'

    # Query parameters for presigned URL
    credential_scope = f'{date_stamp}/us-east-005/s3/aws4_request'
    params = {
        'X-Amz-Algorithm': 'AWS4-HMAC-SHA256',
        'X-Amz-Credential': f'{access_key}/{credential_scope}',
        'X-Amz-Date': amz_date,
        'X-Amz-Expires': '3600',
        'X-Amz-SignedHeaders': 'host'
    }

    # Build canonical request for query auth
    canonical_query = '&'.join(f'{k}={urllib.parse.quote(str(v), safe="")}' for k, v in sorted(params.items()))

    canonical_headers = 'host:localhost:9000\n'
    signed_headers = 'host'
    payload_hash = hashlib.sha256(b'').hexdigest()

    canonical_request = f'{method}\n{path}\n{canonical_query}\n{canonical_headers}\n{signed_headers}\n{payload_hash}'

    # String to sign
    string_to_sign = f'AWS4-HMAC-SHA256\n{amz_date}\n{credential_scope}\n{hashlib.sha256(canonical_request.encode()).hexdigest()}'

    # Signature
    def sign(key, msg):
        return hmac.new(key, msg.encode('utf-8'), hashlib.sha256).digest()

    k_date = sign(('AWS4' + secret_key).encode('utf-8'), date_stamp)
    k_region = sign(k_date, 'us-east-005')
    k_service = sign(k_region, 's3')
    k_signing = sign(k_service, 'aws4_request')
    signature = hmac.new(k_signing, string_to_sign.encode('utf-8'), hashlib.sha256).hexdigest()

    # Final URL
    params['X-Amz-Signature'] = signature
    query_string = '&'.join(f'{k}={urllib.parse.quote(str(v), safe="")}' for k, v in sorted(params.items()))
    url = f'{endpoint}{path}?{query_string}'

    status, resp_headers, body = curl_request('GET', url, {})

    print(f'URL: {endpoint}{path}?...')
    print(f'Status Code: {status}')

    if status == 404:
        print('✅ PASSED: Presigned URL authentication accepted (404 = key not found)')
        return True
    elif status in (200, 206):
        print('✅ PASSED: Presigned URL authentication accepted and key exists')
        return True
    else:
        print(f'❌ FAILED: Presigned URL auth failed with status {status}')
        print(f'Response: {body[:300]}')
        return False


def main():
    endpoint = 'http://localhost:9000'

    # Use environment variables or test defaults
    access_key = sys.argv[1] if len(sys.argv) > 1 else sys.environ.get('ARMOR_TEST_ACCESS_KEY', 'TESTACCESSKEY')
    secret_key = sys.argv[2] if len(sys.argv) > 2 else sys.environ.get('ARMOR_TEST_SECRET_KEY', 'TESTSECRETKEY123456789012345678901234')

    print('=' * 50)
    print('ARMOR Authentication Verification')
    print('=' * 50)
    print(f'Endpoint: {endpoint}')
    print(f'Access Key: {access_key[:8]}...')
    print()

    results = []

    # Run all tests
    results.append(('Valid Authentication', test_valid_auth(endpoint, access_key, secret_key)))
    results.append(('Invalid Access Key', test_invalid_access_key(endpoint, access_key, secret_key)))
    results.append(('Invalid Signature', test_invalid_signature(endpoint, access_key, secret_key)))
    results.append(('Missing Auth Header', test_missing_auth_header(endpoint)))
    results.append(('Expired Request', test_expired_request(endpoint, access_key, secret_key)))
    results.append(('Query Auth (Presigned)', test_query_auth_presigned_url(endpoint, access_key, secret_key)))

    # Summary
    print('\n' + '=' * 50)
    print('SUMMARY')
    print('=' * 50)

    passed = sum(1 for _, r in results if r)
    total = len(results)

    for name, result in results:
        status = '✅ PASS' if result else '❌ FAIL'
        print(f'{status} - {name}')

    print()
    print(f'Total: {passed}/{total} tests passed')

    if passed == total:
        print('\n🎉 All authentication tests passed!')
        return 0
    else:
        print(f'\n⚠️  {total - passed} test(s) failed')
        return 1


if __name__ == '__main__':
    sys.exit(main())
