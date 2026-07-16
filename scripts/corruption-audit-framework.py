#!/usr/bin/env python3
"""
ARMOR Multipart-Era Corruption Audit Framework

Phase 6: Comprehensive audit of unaudited ARMOR buckets to detect multipart-era corruption.
This script orchestrates the full audit pipeline:
1. Enumerate objects >5MiB from each bucket
2. Cross-reference with affected-version deployment windows
3. Verify each candidate object with real restore/decrypt
4. Generate corruption inventory with remediation plan

Affected version window: 0.1.35–0.1.41 (2026-06-10/11)
Fixed versions: 0.1.42+
"""

import os
import sys
import json
import datetime
import tempfile
import subprocess
from typing import Dict, List, Any, Optional
from pathlib import Path

# Configuration
SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB
AFFECTED_VERSIONS = ["0.1.35", "0.1.36", "0.1.37", "0.1.38", "0.1.39", "0.1.40", "0.1.41"]
FIXED_VERSIONS = ["0.1.42", "0.1.43", "0.1.44", "0.1.45"]  # Will be dynamically updated

# Buckets to audit
BUCKETS_TO_AUDIT = {
    "armor-apexalgo": {
        "cluster": "apexalgo-iad",
        "description": "Confirmed LIVE ACB content - never rotate MEK without listing this bucket first",
        "risk_level": "CRITICAL"
    },
    "ord-devimprint": {
        "cluster": "ord-devimprint",
        "description": "queue-api already confirmed actively corrupted as of 2026-07-14/15 - needs full audit",
        "risk_level": "HIGH"
    },
    "iad-ci": {
        "cluster": "iad-ci",
        "description": "Never audited since original 2026-06 multipart bug",
        "risk_level": "MEDIUM"
    },
    "iad-kalshi": {
        "cluster": "iad-kalshi",
        "description": "Never audited since original 2026-06 multipart bug",
        "risk_level": "MEDIUM"
    },
    "rs-manager": {
        "cluster": "rs-manager",
        "description": "Never audited since original 2026-06 multipart bug",
        "risk_level": "MEDIUM"
    }
}

class MultipartCorruptionAuditor:
    """Main auditor class for multipart-era corruption detection."""

    def __init__(self, work_dir: str):
        self.work_dir = Path(work_dir)
        self.results = {
            "audit_timestamp": datetime.datetime.now().isoformat(),
            "audit_phase": "Phase 6: Multipart-Era Corruption Audit",
            "summary": {
                "total_buckets": len(BUCKETS_TO_AUDIT),
                "buckets_audited": 0,
                "total_objects_enumerated": 0,
                "candidates_for_verification": 0,
                "verified_clean": 0,
                "corrupted": 0,
                "unable_to_verify": 0
            },
            "buckets": {},
            "affected_version_window": {
                "versions": AFFECTED_VERSIONS,
                "fixed_versions": FIXED_VERSIONS,
                "description": "Multipart corruption bug in 0.1.35-0.1.41, fixed in 0.1.42+"
            }
        }

    def enumerate_objects(self, bucket: str) -> List[Dict[str, Any]]:
        """Enumerate objects >5MiB from a bucket."""
        print(f"Enumerating objects >5MiB from {bucket}...", file=sys.stderr)

        # Try HTTP API first (via port-forward), fall back to direct B2
        try:
            return self._enumerate_via_http(bucket)
        except Exception as e:
            print(f"HTTP enumeration failed: {e}", file=sys.stderr)
            print("Attempting direct B2 enumeration...", file=sys.stderr)
            return self._enumerate_via_b2(bucket)

    def _enumerate_via_http(self, bucket: str) -> List[Dict[str, Any]]:
        """Enumerate via ARMOR HTTP API (requires port-forwards)."""
        http_script = Path(__file__).parent / "enumerate-large-objects-http.py"
        if not http_script.exists():
            raise FileNotFoundError(f"HTTP enumeration script not found: {http_script}")

        result = subprocess.run(
            [sys.executable, str(http_script)],
            capture_output=True,
            text=True,
            timeout=300
        )

        if result.returncode != 0:
            raise RuntimeError(f"HTTP enumeration failed: {result.stderr}")

        data = json.loads(result.stdout)
        bucket_data = data.get("buckets", {}).get(bucket, {})
        return bucket_data.get("objects", [])

    def _enumerate_via_b2(self, bucket: str) -> List[Dict[str, Any]]:
        """Enumerate via direct B2 access (requires B2 credentials)."""
        b2_script = Path(__file__).parent / "enumerate-large-objects.py"
        if not b2_script.exists():
            raise FileNotFoundError(f"B2 enumeration script not found: {b2_script}")

        result = subprocess.run(
            [sys.executable, str(b2_script)],
            capture_output=True,
            text=True,
            timeout=300,
            env=os.environ.copy()
        )

        if result.returncode != 0:
            raise RuntimeError(f"B2 enumeration failed: {result.stderr}")

        data = json.loads(result.stdout)
        bucket_data = data.get("buckets", {}).get(bucket, {})
        return bucket_data.get("objects", [])

    def cross_reference_deployment_windows(self, bucket: str, objects: List[Dict]) -> List[Dict]:
        """Cross-reference object timestamps with affected deployment windows."""
        print(f"Cross-referencing {len(objects)} objects against deployment windows...", file=sys.stderr)

        # Load version drift data from bf-2t1f
        drift_file = Path(__file__).parent.parent / "data" / "version-drift-report.json"
        deployment_windows = {}

        if drift_file.exists():
            with open(drift_file, 'r') as f:
                drift_data = json.load(f)
                for deployment in drift_data.get("deployments", []):
                    cluster = deployment["cluster"]
                    deployment_windows[cluster] = {
                        "deployed_tag": deployment["deployed_tag"],
                        "is_affected": deployment["deployed_tag"] in AFFECTED_VERSIONS,
                        "is_fixed": any(deployment["deployed_tag"].startswith(fv) for fv in FIXED_VERSIONS)
                    }

        candidates = []
        for obj in objects:
            last_modified = datetime.datetime.fromisoformat(obj["last_modified"].replace('Z', '+00:00'))

            # Determine if object was written during affected window
            # This is a simplified check - real implementation would need exact deployment dates
            obj_info = obj.copy()
            obj_info["risk_level"] = "UNKNOWN"
            obj_info["in_affected_window"] = False

            # Mark objects from affected deployments as high-risk
            bucket_info = BUCKETS_TO_AUDIT.get(bucket, {})
            cluster = bucket_info.get("cluster", "")
            if cluster in deployment_windows:
                window = deployment_windows[cluster]
                if window.get("is_affected"):
                    obj_info["in_affected_window"] = True
                    obj_info["risk_level"] = "HIGH"
                elif window.get("is_fixed"):
                    obj_info["risk_level"] = "LOW"

            candidates.append(obj_info)

        return candidates

    def verify_objects(self, bucket: str, candidates: List[Dict]) -> List[Dict]:
        """Verify candidate objects with real restore/decrypt."""
        print(f"Verifying {len(candidates)} candidates from {bucket}...", file=sys.stderr)

        verification_script = Path(__file__).parent / "verify-multipart-integrity.py"
        if not verification_script.exists():
            print(f"Warning: Verification script not found: {verification_script}", file=sys.stderr)
            print("Skipping object verification - marking all as UNABLE_TO_VERIFY", file=sys.stderr)
            return [{"object": c, "verification_status": "UNABLE_TO_VERIFY", "error": "Verification script not found"} for c in candidates]

        # Write candidates to temp file for verification script
        candidates_file = self.work_dir / f"{bucket}_candidates.json"
        with open(candidates_file, 'w') as f:
            json.dump({"by_bucket": {bucket: {"candidates_for_verification": candidates}}}, f)

        try:
            result = subprocess.run(
                [sys.executable, str(verification_script), str(candidates_file)],
                capture_output=True,
                text=True,
                timeout=3600  # 1 hour for verification
            )

            if result.returncode != 0:
                raise RuntimeError(f"Verification failed: {result.stderr}")

            verification_data = json.loads(result.stdout)
            bucket_results = verification_data.get("by_bucket", {}).get(bucket, {})
            return bucket_results.get("objects", [])

        except subprocess.TimeoutExpired:
            print(f"Verification timed out for {bucket}", file=sys.stderr)
            return [{"object": c, "verification_status": "UNABLE_TO_VERIFY", "error": "Verification timed out"} for c in candidates]

    def audit_bucket(self, bucket: str) -> Dict[str, Any]:
        """Audit a single bucket."""
        print(f"\n=== Auditing bucket: {bucket} ===", file=sys.stderr)

        bucket_info = BUCKETS_TO_AUDIT.get(bucket, {})
        bucket_result = {
            "bucket": bucket,
            "cluster": bucket_info.get("cluster", "unknown"),
            "description": bucket_info.get("description", ""),
            "risk_level": bucket_info.get("risk_level", "UNKNOWN"),
            "enumeration_status": "PENDING",
            "objects_found": 0,
            "candidates": [],
            "verification_status": "PENDING",
            "verified_clean": 0,
            "corrupted": 0,
            "unable_to_verify": 0
        }

        try:
            # Step 1: Enumerate objects
            objects = self.enumerate_objects(bucket)
            bucket_result["objects_found"] = len(objects)
            bucket_result["enumeration_status"] = "COMPLETE"
            self.results["summary"]["total_objects_enumerated"] += len(objects)

            # Step 2: Cross-reference with deployment windows
            candidates = self.cross_reference_deployment_windows(bucket, objects)
            bucket_result["candidates"] = candidates
            self.results["summary"]["candidates_for_verification"] += len(candidates)

            # Step 3: Verify each candidate
            verification_results = self.verify_objects(bucket, candidates)

            for v in verification_results:
                status = v.get("verification_status", "UNABLE_TO_VERIFY")
                if status == "VERIFIED":
                    bucket_result["verified_clean"] += 1
                    self.results["summary"]["verified_clean"] += 1
                elif status == "CORRUPTED":
                    bucket_result["corrupted"] += 1
                    self.results["summary"]["corrupted"] += 1
                else:
                    bucket_result["unable_to_verify"] += 1
                    self.results["summary"]["unable_to_verify"] += 1

            bucket_result["verification_status"] = "COMPLETE"
            bucket_result["verification_results"] = verification_results

        except Exception as e:
            print(f"Error auditing {bucket}: {e}", file=sys.stderr)
            bucket_result["enumeration_status"] = "FAILED"
            bucket_result["verification_status"] = "FAILED"
            bucket_result["error"] = str(e)

        self.results["buckets"][bucket] = bucket_result
        self.results["summary"]["buckets_audited"] += 1

        return bucket_result

    def generate_corruption_inventory(self) -> Dict[str, Any]:
        """Generate the final corruption inventory and remediation plan."""
        print("\n=== Generating Corruption Inventory ===", file=sys.stderr)

        inventory = {
            "corruption_summary": {
                "total_corrupted_objects": self.results["summary"]["corrupted"],
                "total_unable_to_verify": self.results["summary"]["unable_to_verify"],
                "buckets_with_corruption": [],
                "buckets_unable_to_verify": []
            },
            "remediation_plan": []
        }

        for bucket, result in self.results["buckets"].items():
            if result.get("corrupted", 0) > 0:
                inventory["corruption_summary"]["buckets_with_corruption"].append(bucket)

                # Find corrupted objects for remediation
                corrupted_objects = [
                    v for v in result.get("verification_results", [])
                    if v.get("verification_status") == "CORRUPTED"
                ]

                for obj in corrupted_objects:
                    inventory["remediation_plan"].append({
                        "bucket": bucket,
                        "key": obj.get("key", "unknown"),
                        "size": obj.get("size", 0),
                        "action": "RE_UPLOAD",
                        "priority": "HIGH" if obj.get("in_affected_window") else "MEDIUM",
                        "reason": "Corrupted - needs re-upload from source"
                    })

            if result.get("unable_to_verify", 0) > 0:
                inventory["corruption_summary"]["buckets_unable_to_verify"].append(bucket)

        self.results["corruption_inventory"] = inventory
        return self.results

    def save_results(self, output_file: Optional[str] = None):
        """Save audit results to file."""
        results_file = output_file or self.work_dir / "corruption_audit_results.json"

        with open(results_file, 'w') as f:
            json.dump(self.results, f, indent=2)

        print(f"\nResults saved to: {results_file}", file=sys.stderr)

        # Print summary
        print(f"\n=== Audit Summary ===", file=sys.stderr)
        print(f"Buckets audited: {self.results['summary']['buckets_audited']}/{self.results['summary']['total_buckets']}", file=sys.stderr)
        print(f"Total objects enumerated: {self.results['summary']['total_objects_enumerated']}", file=sys.stderr)
        print(f"Candidates for verification: {self.results['summary']['candidates_for_verification']}", file=sys.stderr)
        print(f"Verified clean: {self.results['summary']['verified_clean']}", file=sys.stderr)
        print(f"Corrupted: {self.results['summary']['corrupted']}", file=sys.stderr)
        print(f"Unable to verify: {self.results['summary']['unable_to_verify']}", file=sys.stderr)


def main():
    """Main entry point."""
    import argparse

    parser = argparse.ArgumentParser(
        description='ARMOR Multipart-Era Corruption Audit Framework'
    )
    parser.add_argument(
        '--work-dir',
        default='./audit_work',
        help='Working directory for temporary files'
    )
    parser.add_argument(
        '--output',
        help='Output file for audit results'
    )
    parser.add_argument(
        '--bucket',
        help='Audit specific bucket only (default: all buckets)'
    )

    args = parser.parse_args()

    # Create work directory
    work_dir = Path(args.work_dir)
    work_dir.mkdir(parents=True, exist_ok=True)

    # Run audit
    auditor = MultipartCorruptionAuditor(str(work_dir))

    buckets_to_audit = [args.bucket] if args.bucket else list(BUCKETS_TO_AUDIT.keys())

    for bucket in buckets_to_audit:
        auditor.audit_bucket(bucket)

    # Generate final inventory
    results = auditor.generate_corruption_inventory()

    # Save results
    auditor.save_results(args.output)

    # Exit with error code if corruption found
    if results["summary"]["corrupted"] > 0:
        print(f"\n⚠️  CORRUPTION DETECTED: {results['summary']['corrupted']} objects corrupted", file=sys.stderr)
        sys.exit(1)
    elif results["summary"]["unable_to_verify"] > 0:
        print(f"\n⚠️  UNABLE TO VERIFY: {results['summary']['unable_to_verify']} objects could not be verified", file=sys.stderr)
        sys.exit(2)

    print(f"\n✅ AUDIT COMPLETE: All {results['summary']['verified_clean']} verified objects are clean", file=sys.stderr)
    sys.exit(0)


if __name__ == "__main__":
    main()