#!/usr/bin/env python3
"""
Verification script for multipart-era corruption audit.
Tests restore/decrypt operations on candidate objects to detect corruption.
"""

import os
import sys
import json
import tempfile
import hashlib
from datetime import datetime
from typing import Dict, List, Any, Optional
import subprocess
import signal

class TimeoutException(Exception):
    pass

def timeout_handler(signum, frame):
    raise TimeoutException("Operation timed out")

def run_armor_decrypt(bucket: str, key: str, output_path: str, timeout_sec: int = 300) -> Dict[str, Any]:
    """Run armor-decrypt to restore and decrypt an object."""
    result = {
        'success': False,
        'error': None,
        'file_size': 0,
        'file_hash': None,
        'duration_seconds': 0
    }

    start_time = datetime.now()

    try:
        # Set signal handler for timeout
        signal.signal(signal.SIGALRM, timeout_handler)
        signal.alarm(timeout_sec)

        # Run armor-decrypt command
        cmd = [
            '/home/coding/ARMOR/armor-decrypt',
            '-bucket', bucket,
            '-key', key,
            '-output', output_path
        ]

        process = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout_sec
        )

        signal.alarm(0)  # Cancel timeout

        if process.returncode == 0:
            # Check if file was created and has content
            if os.path.exists(output_path) and os.path.getsize(output_path) > 0:
                result['success'] = True
                result['file_size'] = os.path.getsize(output_path)

                # Calculate file hash for integrity verification
                with open(output_path, 'rb') as f:
                    file_hash = hashlib.sha256(f.read()).hexdigest()
                result['file_hash'] = file_hash
            else:
                result['error'] = "Decrypt succeeded but no output file created"
        else:
            result['error'] = f"Decrypt failed with code {process.returncode}: {process.stderr}"

    except subprocess.TimeoutExpired:
        signal.alarm(0)
        result['error'] = f"Decrypt timed out after {timeout_sec}s"
    except TimeoutException:
        signal.alarm(0)
        result['error'] = f"Decrypt timed out after {timeout_sec}s"
    except Exception as e:
        signal.alarm(0)
        result['error'] = f"Decrypt exception: {str(e)}"

    end_time = datetime.now()
    result['duration_seconds'] = (end_time - start_time).total_seconds()

    return result

def run_armor_http_get(bucket: str, key: str, timeout_sec: int = 300) -> Dict[str, Any]:
    """Run HTTP GET through ARMOR server path."""
    result = {
        'success': False,
        'error': None,
        'status_code': None,
        'duration_seconds': 0
    }

    start_time = datetime.now()

    try:
        # Try to get object via HTTP GET
        # This tests the ARMOR server read path
        cmd = ['curl', '-s', '-o', '/dev/null', '-w', '%{http_code}']
        # Would need ARMOR endpoint - placeholder for now
        result['error'] = "HTTP GET verification not implemented - needs ARMOR endpoint"

    except Exception as e:
        result['error'] = f"HTTP GET exception: {str(e)}"

    end_time = datetime.now()
    result['duration_seconds'] = (end_time - start_time).total_seconds()

    return result

def verify_object(bucket: str, obj: Dict[str, Any], work_dir: str) -> Dict[str, Any]:
    """Verify a single object for corruption."""
    key = obj['key']
    safe_key = key.replace('/', '_').replace('\\', '_')
    output_path = os.path.join(work_dir, safe_key)

    verification_result = {
        'bucket': bucket,
        'key': key,
        'size': obj['size'],
        'risk_level': obj.get('risk_level', 'UNKNOWN'),
        'in_affected_window': obj.get('in_affected_window', False),
        'is_multipart': obj.get('is_multipart'),
        'decrypt_result': None,
        'http_get_result': None,
        'corruption_detected': False,
        'verification_status': 'FAILED',
        'timestamp': datetime.now().isoformat()
    }

    # Try armor-decrypt (direct disaster recovery path)
    decrypt_result = run_armor_decrypt(bucket, key, output_path)
    verification_result['decrypt_result'] = {
        'success': decrypt_result['success'],
        'error': decrypt_result['error'],
        'duration_seconds': decrypt_result['duration_seconds']
    }

    if decrypt_result['success']:
        verification_result['verification_status'] = 'VERIFIED'
        verification_result['corruption_detected'] = False

        # Clean up output file
        try:
            os.remove(output_path)
        except:
            pass
    else:
        verification_result['corruption_detected'] = True
        verification_result['verification_status'] = 'CORRUPTED'

    return verification_result

def main():
    """Main verification function."""
    if len(sys.argv) < 2:
        print("Usage: verify-multipart-integrity.py <candidates.json> [output.json]", file=sys.stderr)
        print("  candidates.json: Output from cross-reference-affected-objects.py", file=sys.stderr)
        print("  output.json: (Optional) Verification results output file", file=sys.stderr)
        sys.exit(1)

    input_file = sys.argv[1]
    output_file = sys.argv[2] if len(sys.argv) > 2 else None

    with open(input_file, 'r') as f:
        candidates_data = json.load(f)

    print(f"Starting multipart integrity verification...", file=sys.stderr)

    results = {
        'verification_timestamp': datetime.now().isoformat(),
        'summary': {
            'total_candidates': 0,
            'verified': 0,
            'corrupted': 0,
            'failed': 0
        },
        'by_bucket': {}
    }

    # Create work directory for temporary files
    work_dir = tempfile.mkdtemp(prefix='armor_verify_')
    print(f"Using work directory: {work_dir}", file=sys.stderr)

    try:
        for bucket_name, bucket_data in candidates_data.get('by_bucket', {}).items():
            print(f"\n=== Verifying bucket: {bucket_name} ===", file=sys.stderr)

            bucket_results = {
                'bucket': bucket_name,
                'candidates_tested': 0,
                'verified': 0,
                'corrupted': 0,
                'failed': 0,
                'objects': []
            }

            candidates = bucket_data.get('candidates_for_verification', [])
            print(f"Candidates to verify: {len(candidates)}", file=sys.stderr)

            for obj in candidates:
                print(f"  Verifying: {obj['key'][:50]}... ({obj['size_mb']} MiB)", file=sys.stderr)

                result = verify_object(bucket_name, obj, work_dir)
                bucket_results['objects'].append(result)
                bucket_results['candidates_tested'] += 1
                results['summary']['total_candidates'] += 1

                if result['verification_status'] == 'VERIFIED':
                    bucket_results['verified'] += 1
                    results['summary']['verified'] += 1
                elif result['verification_status'] == 'CORRUPTED':
                    bucket_results['corrupted'] += 1
                    results['summary']['corrupted'] += 1
                else:
                    bucket_results['failed'] += 1
                    results['summary']['failed'] += 1

            results['by_bucket'][bucket_name] = bucket_results

        # Output results
        print(json.dumps(results, indent=2))

        if output_file:
            with open(output_file, 'w') as f:
                json.dump(results, f, indent=2)
            print(f"\nResults saved to: {output_file}", file=sys.stderr)

        print(f"\n=== Verification Summary ===", file=sys.stderr)
        print(f"Total candidates tested: {results['summary']['total_candidates']}", file=sys.stderr)
        print(f"Verified (clean): {results['summary']['verified']}", file=sys.stderr)
        print(f"Corrupted: {results['summary']['corrupted']}", file=sys.stderr)
        print(f"Failed (other errors): {results['summary']['failed']}", file=sys.stderr)

    finally:
        # Clean up work directory
        try:
            import shutil
            shutil.rmtree(work_dir)
            print(f"\nCleaned up work directory: {work_dir}", file=sys.stderr)
        except Exception as e:
            print(f"Warning: Could not clean up work directory: {e}", file=sys.stderr)

if __name__ == "__main__":
    main()
