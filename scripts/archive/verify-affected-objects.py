#!/usr/bin/env python3
"""
Verify affected objects using armor-decrypt.

This script processes all objects enumerated in intermediate/affected-objects.json
and runs armor-decrypt verification on each, capturing results for corruption
detection.
"""

import json
import subprocess
import sys
from pathlib import Path
from typing import Dict, List, Any
from datetime import datetime
import os

# Configuration
AFFECTED_OBJECTS_FILE = Path("intermediate/affected-objects.json")
RESULTS_FILE = Path("intermediate/verification-results.json")
SUMMARY_FILE = Path("intermediate/verification-summary.json")
ARMOR_DECRYPT_BIN = Path(__file__).parent.parent / "armor-decrypt"  # Absolute path to armor-decrypt

def load_affected_objects() -> List[Dict[str, Any]]:
    """Load the list of affected objects from enumeration."""
    if not AFFECTED_OBJECTS_FILE.exists():
        print(f"Error: {AFFECTED_OBJECTS_FILE} not found. Run enumeration first.")
        sys.exit(1)

    with open(AFFECTED_OBJECTS_FILE) as f:
        objects = json.load(f)
    print(f"Loaded {len(objects)} affected objects from {AFFECTED_OBJECTS_FILE}")
    return objects

def verify_object(obj: Dict[str, Any]) -> Dict[str, Any]:
    """Verify a single object using armor-decrypt."""
    bucket = obj["bucket"]
    key = obj["key"]
    armor_versions = obj["affected_armor_versions"]

    result = {
        "bucket": bucket,
        "key": key,
        "size_bytes": obj["size_bytes"],
        "created_at": obj["created_at"],
        "affected_armor_versions": armor_versions,
        "status": "pending",
        "error_details": None,
        "verification_time": datetime.utcnow().isoformat() + "Z"
    }

    # Build B2 URL
    b2_url = f"b2://{bucket}/{key}"

    # Run armor-decrypt with verbose output
    cmd = [
        str(ARMOR_DECRYPT_BIN),
        "-input", b2_url,
        "-v"  # Verbose to stderr
    ]

    try:
        # Run the command, capture both stdout and stderr
        process = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        stdout, stderr = process.communicate(timeout=300)  # 5 minute timeout per object

        # Check exit code
        if process.returncode == 0:
            result["status"] = "valid"
            result["error_details"] = None
        else:
            # Parse error type from stderr
            stderr_lower = stderr.lower()
            if "hmac" in stderr_lower or "checksum" in stderr_lower:
                result["status"] = "corrupted"
                result["error_details"] = "HMAC/Checksum verification failed"
            elif "plaintext sha" in stderr_lower:
                result["status"] = "corrupted"
                result["error_details"] = "Plaintext SHA-256 verification failed"
            elif "unwrap dek" in stderr_lower:
                result["status"] = "failed"
                result["error_details"] = "DEK unwrap failed (wrong MEK or corrupted wrapped DEK)"
            elif "head b2" in stderr_lower or "read" in stderr_lower:
                result["status"] = "failed"
                result["error_details"] = "B2 read error"
            else:
                result["status"] = "failed"
                result["error_details"] = f"Unknown error: {stderr.strip()}"

        # Store stderr for debugging
        result["stderr"] = stderr.strip()

    except subprocess.TimeoutExpired:
        result["status"] = "failed"
        result["error_details"] = "Verification timeout (>5 minutes)"
        process.kill()
        process.communicate()

    except Exception as e:
        result["status"] = "failed"
        result["error_details"] = f"Exception during verification: {str(e)}"

    return result

def check_prerequisites() -> bool:
    """Check if required tools and credentials are available."""
    # Check armor-decrypt binary
    if not ARMOR_DECRYPT_BIN.exists():
        print(f"Error: {ARMOR_DECRYPT_BIN} not found. Build with: go build -o armor-decrypt ./cmd/armor-decrypt")
        return False

    # Check if we can run armor-decrypt --help
    try:
        result = subprocess.run([str(ARMOR_DECRYPT_BIN), "-help"],
                             capture_output=True, timeout=5)
        if result.returncode != 0:
            print(f"Error: {ARMOR_DECRYPT_BIN} not executable")
            return False
    except Exception as e:
        print(f"Error running {ARMOR_DECRYPT_BIN}: {e}")
        return False

    # Check for required environment variables
    required_vars = ["ARMOR_MEK", "ARMOR_B2_REGION", "ARMOR_B2_ENDPOINT",
                     "ARMOR_B2_ACCESS_KEY_ID", "ARMOR_B2_SECRET_ACCESS_KEY"]

    missing_vars = [var for var in required_vars if not os.getenv(var)]
    if missing_vars:
        print("Error: Missing required environment variables:")
        for var in missing_vars:
            print(f"  - {var}")
        print("\nThese credentials are required to run armor-decrypt verification.")
        print("Please set them before running this script.")
        return False

    print("✓ Prerequisites check passed")
    return True

def group_results(results: List[Dict[str, Any]]) -> Dict[str, Any]:
    """Group results by bucket and ARMOR version."""
    grouped = {
        "by_bucket": {},
        "by_armor_version": {},
        "summary": {
            "total": len(results),
            "valid": 0,
            "corrupted": 0,
            "failed": 0
        }
    }

    for result in results:
        bucket = result["bucket"]
        status = result["status"]
        versions = result["affected_armor_versions"]

        # Update summary
        grouped["summary"][status] = grouped["summary"].get(status, 0) + 1

        # Group by bucket
        if bucket not in grouped["by_bucket"]:
            grouped["by_bucket"][bucket] = {
                "total": 0,
                "valid": 0,
                "corrupted": 0,
                "failed": 0
            }

        grouped["by_bucket"][bucket]["total"] += 1
        grouped["by_bucket"][bucket][status] += 1

        # Group by ARMOR version
        for version in versions:
            if version not in grouped["by_armor_version"]:
                grouped["by_armor_version"][version] = {
                    "total": 0,
                    "valid": 0,
                    "corrupted": 0,
                    "failed": 0
                }

            grouped["by_armor_version"][version]["total"] += 1
            grouped["by_armor_version"][version][status] += 1

    return grouped

def main():
    """Main verification workflow."""
    print("=" * 70)
    print("ARMOR Affected Objects Verification")
    print("=" * 70)
    print()

    # Check prerequisites
    if not check_prerequisites():
        sys.exit(1)

    print()

    # Load affected objects
    objects = load_affected_objects()

    print(f"\nStarting verification of {len(objects)} objects...")
    print()

    results = []
    for i, obj in enumerate(objects, 1):
        print(f"[{i}/{len(objects)}] Verifying {obj['bucket']}/{obj['key'][:50]}...", end=" ")

        result = verify_object(obj)
        results.append(result)

        # Print compact status
        if result["status"] == "valid":
            print("✓ VALID")
        elif result["status"] == "corrupted":
            print(f"✗ CORRUPTED ({result['error_details']})")
        else:
            print(f"✗ FAILED ({result['error_details']})")

    # Group and analyze results
    print("\n" + "=" * 70)
    print("Verification Results Summary")
    print("=" * 70)
    print()

    grouped = group_results(results)

    # Print summary
    summary = grouped["summary"]
    print(f"Total objects verified: {summary['total']}")
    print(f"  ✓ Valid: {summary['valid']}")
    print(f"  ✗ Corrupted: {summary['corrupted']}")
    print(f"  ✗ Failed: {summary['failed']}")
    print()

    # Print by bucket
    print("By Bucket:")
    for bucket, stats in grouped["by_bucket"].items():
        print(f"  {bucket}:")
        print(f"    Total: {stats['total']}")
        print(f"    Valid: {stats['valid']}")
        print(f"    Corrupted: {stats['corrupted']}")
        print(f"    Failed: {stats['failed']}")
    print()

    # Print by ARMOR version
    print("By ARMOR Version:")
    for version, stats in sorted(grouped["by_armor_version"].items()):
        print(f"  {version}:")
        print(f"    Total: {stats['total']}")
        print(f"    Valid: {stats['valid']}")
        print(f"    Corrupted: {stats['corrupted']}")
        print(f"    Failed: {stats['failed']}")
    print()

    # Save results
    print("Saving results...")
    with open(RESULTS_FILE, 'w') as f:
        json.dump(results, f, indent=2)

    with open(SUMMARY_FILE, 'w') as f:
        json.dump(grouped, f, indent=2)

    print(f"✓ Detailed results: {RESULTS_FILE}")
    print(f"✓ Summary: {SUMMARY_FILE}")

    # Exit with error code if corruption found
    if summary['corrupted'] > 0:
        print("\n⚠ WARNING: Corruption detected in {summary['corrupted']} objects!")
        sys.exit(1)
    elif summary['failed'] > 0:
        print(f"\n⚠ WARNING: {summary['failed']} objects failed verification!")
        sys.exit(2)
    else:
        print("\n✓ All objects verified successfully!")
        sys.exit(0)

if __name__ == "__main__":
    main()
