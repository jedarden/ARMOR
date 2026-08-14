#!/usr/bin/env python3
"""
Cross-reference objects against deployment windows.

For each object, check if its created_at timestamp falls within any affected
ARMOR version deployment window, and tag with the specific affected version(s).
"""

import json
from datetime import datetime
from typing import List, Dict, Any


def parse_date(date_str: str) -> datetime:
    """Parse ISO date string to datetime object."""
    # Handle various ISO 8601 formats
    if date_str.endswith('Z'):
        date_str = date_str[:-1] + '+00:00'
    try:
        return datetime.fromisoformat(date_str.replace('+00:00', ''))
    except:
        return None


def is_within_window(obj_date: datetime, window_start: str, window_end: str) -> bool:
    """Check if object date falls within deployment window."""
    start = parse_date(window_start)
    end = parse_date(window_end)

    if not start or not end or not obj_date:
        return False

    return start <= obj_date <= end


def find_affected_versions(obj_date: datetime, deployment_windows: List[Dict]) -> List[str]:
    """Find which ARMOR versions were active when object was created."""
    affected_versions = []

    for deployment in deployment_windows:
        window_start = deployment.get('window_start')
        window_end = deployment.get('window_end')

        if is_within_window(obj_date, window_start, window_end):
            version = deployment.get('version')
            if version:
                affected_versions.append(version)

    return affected_versions


def cross_reference_objects(objects: List[Dict], deployment_windows: Dict) -> List[Dict]:
    """Cross-reference objects against deployment windows."""
    windows = deployment_windows.get('affected_deployment_windows', [])

    filtered_objects = []

    for obj in objects:
        created_at = obj.get('created_at')
        if not created_at:
            continue

        obj_date = parse_date(created_at)
        if not obj_date:
            continue

        # Find affected versions for this object
        affected_versions = find_affected_versions(obj_date, windows)

        # Only include objects that overlap with affected deployments
        if affected_versions:
            obj_copy = obj.copy()
            obj_copy['affected_armor_versions'] = affected_versions
            filtered_objects.append(obj_copy)

    return filtered_objects


def main():
    """Main cross-reference function."""
    # Load objects
    with open('/tmp/unaudited-buckets-objects.json', 'r') as f:
        objects = json.load(f)

    # Load deployment windows
    with open('/home/coding/ARMOR/intermediate/deployment-windows.json', 'r') as f:
        deployment_windows = json.load(f)

    print(f"Total objects: {len(objects)}", file=__import__('sys').stderr)
    print(f"Affected deployment windows: {len(deployment_windows.get('affected_deployment_windows', []))}", file=__import__('sys').stderr)

    # Cross-reference
    filtered = cross_reference_objects(objects, deployment_windows)

    # Output filtered objects
    print(json.dumps(filtered, indent=2))

    print(f"\nFiltered objects (in affected windows): {len(filtered)}", file=__import__('sys').stderr)

    # Print summary by affected version
    version_counts = {}
    for obj in filtered:
        for version in obj.get('affected_armor_versions', []):
            version_counts[version] = version_counts.get(version, 0) + 1

    print(f"\nObjects by affected version:", file=__import__('sys').stderr)
    for version, count in sorted(version_counts.items()):
        print(f"  {version}: {count}", file=__import__('sys').stderr)


if __name__ == "__main__":
    main()
