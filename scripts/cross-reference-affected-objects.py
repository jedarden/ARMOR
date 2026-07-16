#!/usr/bin/env python3
"""
Cross-reference large objects against affected ARMOR version windows.
Maps object timestamps to deployments of multipart-buggy versions.
"""

import os
import sys
import json
from datetime import datetime, timedelta
from typing import Dict, List, Any, Tuple

# Affected window: multipart implementation (2026-03-24) to multipart read fix (2026-07-15)
MULTIPART_BUG_START = "2026-03-24"
MULTIPART_BUG_END = "2026-07-15"

# Version windows when multipart bug was active
# Based on git history: multipart implemented 2026-03-24, fixed 2026-07-15
# Any version deployed between these dates could have written corrupt multipart objects

def load_version_drift_data() -> Dict[str, Any]:
    """Load version drift data from the drift check output."""
    try:
        # Try to run drift check and get JSON output
        import subprocess
        result = subprocess.run(
            ["python3", "/home/coding/ARMOR/scripts/version-drift-check.py", "--json"],
            capture_output=True, text=True, timeout=30
        )
        if result.returncode == 0:
            return json.loads(result.stdout)
    except Exception as e:
        print(f"Warning: Could not load drift data: {e}", file=sys.stderr)

    return {}

def parse_date(date_str: str) -> datetime:
    """Parse ISO date string to datetime object."""
    if date_str.endswith('Z'):
        date_str = date_str[:-1] + '+00:00'
    try:
        return datetime.fromisoformat(date_str)
    except:
        return None

def is_object_in_affected_window(last_modified: str) -> bool:
    """Check if an object was written during the multipart bug window."""
    obj_date = parse_date(last_modified)
    if not obj_date:
        return False

    bug_start = datetime.fromisoformat(MULTIPART_BUG_START)
    bug_end = datetime.fromisoformat(MULTIPART_BUG_END)

    return bug_start <= obj_date <= bug_end

def get_risk_level(obj: Dict[str, Any]) -> str:
    """Determine risk level based on object properties."""
    # High risk: multipart + in affected window
    if obj.get('is_multipart') and is_object_in_affected_window(obj['last_modified']):
        return "HIGH"

    # Medium risk: in affected window but multipart status unknown
    if not obj.get('is_multipart') and is_object_in_affected_window(obj['last_modified']):
        return "MEDIUM"

    # Low risk: multipart but outside affected window
    if obj.get('is_multipart') and not is_object_in_affected_window(obj['last_modified']):
        return "LOW"

    # Minimal risk: not multipart and outside window
    return "MINIMAL"

def cross_reference_objects(objects_data: Dict[str, Any]) -> Dict[str, Any]:
    """Cross-reference large objects against affected version windows."""
    results = {
        'summary': {
            'total_objects': 0,
            'high_risk': 0,
            'medium_risk': 0,
            'low_risk': 0,
            'minimal_risk': 0
        },
        'by_bucket': {},
        'affected_window': {
            'start': MULTIPART_BUG_START,
            'end': MULTIPART_BUG_END
        }
    }

    for bucket_name, bucket_data in objects_data.get('buckets', {}).items():
        bucket_results = {
            'bucket': bucket_name,
            'total_objects': bucket_data.get('count', 0),
            'candidates_for_verification': [],
            'risk_summary': {
                'HIGH': 0,
                'MEDIUM': 0,
                'LOW': 0,
                'MINIMAL': 0
            }
        }

        for obj in bucket_data.get('objects', []):
            risk = get_risk_level(obj)
            obj['risk_level'] = risk
            obj['in_affected_window'] = is_object_in_affected_window(obj['last_modified'])

            bucket_results['risk_summary'][risk] += 1
            results['summary'][f'{risk.lower()}_risk'] += 1
            results['summary']['total_objects'] += 1

            # Include HIGH and MEDIUM risk objects for verification
            if risk in ['HIGH', 'MEDIUM']:
                bucket_results['candidates_for_verification'].append(obj)

        results['by_bucket'][bucket_name] = bucket_results

    return results

def main():
    """Main cross-reference function."""
    print(f"Cross-referencing against multipart bug window: {MULTIPART_BUG_START} to {MULTIPART_BUG_END}", file=sys.stderr)

    if len(sys.argv) > 1:
        # Load from file
        with open(sys.argv[1], 'r') as f:
            objects_data = json.load(f)
    else:
        # Read from stdin
        objects_data = json.load(sys.stdin)

    results = cross_reference_objects(objects_data)

    print(json.dumps(results, indent=2))

    print(f"\n=== Cross-Reference Summary ===", file=sys.stderr)
    print(f"Total objects analyzed: {results['summary']['total_objects']}", file=sys.stderr)
    print(f"HIGH risk (multipart + affected window): {results['summary']['high_risk']}", file=sys.stderr)
    print(f"MEDIUM risk (unknown multipart + affected window): {results['summary']['medium_risk']}", file=sys.stderr)
    print(f"LOW risk (multipart + outside window): {results['summary']['low_risk']}", file=sys.stderr)
    print(f"MINIMAL risk (not multipart + outside window): {results['summary']['minimal_risk']}", file=sys.stderr)

    # Print candidate count for verification
    total_candidates = sum(
        len(b.get('candidates_for_verification', []))
        for b in results['by_bucket'].values()
    )
    print(f"\nCandidates for verification (HIGH + MEDIUM): {total_candidates}", file=sys.stderr)

if __name__ == "__main__":
    main()
