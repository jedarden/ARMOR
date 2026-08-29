#!/usr/bin/env python3
"""
Filter objects from unaudited buckets that were created during affected ARMOR version deployments.

This script:
1. Loads unaudited-buckets-objects.json
2. Filters to objects >5MiB
3. Cross-references with deployment windows from intermediate/deployment-windows.json
4. Outputs structured list of affected objects
"""

import json
import sys
from datetime import datetime
from pathlib import Path

# Constants
MIN_SIZE_BYTES = 5 * 1024 * 1024  # 5MiB
INPUT_OBJECTS = Path("/home/coding/ARMOR/unaudited-buckets-objects.json")
INPUT_DEPLOYMENTS = Path("/home/coding/ARMOR/intermediate/deployment-windows.json")
OUTPUT_FILE = Path("/home/coding/ARMOR/intermediate/affected-objects.json")


def parse_timestamp(ts_str):
    """Parse ISO timestamp string to datetime object."""
    if ts_str.endswith("+00:00"):
        ts_str = ts_str.replace("+00:00", "Z")
    if ts_str.endswith("Z"):
        return datetime.fromisoformat(ts_str.replace("Z", "+00:00"))
    return datetime.fromisoformat(ts_str)


def main():
    print("Loading deployment windows...")
    with open(INPUT_DEPLOYMENTS) as f:
        deployment_data = json.load(f)

    deployment_windows = deployment_data["affected_deployment_windows"]
    print(f"  Loaded {len(deployment_windows)} affected deployment windows")

    # Create a list of (version, window_start, window_end) tuples
    windows = []
    for dw in deployment_windows:
        start = parse_timestamp(dw["window_start"])
        end = parse_timestamp(dw["window_end"])
        windows.append({
            "version": dw["version"],
            "start": start,
            "end": end,
            "deployed_at": parse_timestamp(dw["deployed_at"]),
        })

    print(f"  Windows span from {min(w['start'] for w in windows)} to {max(w['end'] for w in windows)}")

    print("\nLoading objects from unaudited buckets...")
    with open(INPUT_OBJECTS) as f:
        objects = json.load(f)

    print(f"  Loaded {len(objects)} total objects")

    # Filter to objects >5MiB
    large_objects = [obj for obj in objects if obj["size_bytes"] > MIN_SIZE_BYTES]
    print(f"  Found {len(large_objects)} objects >5MiB")

    # Cross-reference with deployment windows
    print("\nCross-referencing with deployment windows...")
    affected_objects = []

    for obj in large_objects:
        obj_time = parse_timestamp(obj["created_at"])

        # Find which deployment windows this object falls into
        obj_versions = []
        for window in windows:
            if window["start"] <= obj_time <= window["end"]:
                obj_versions.append(window["version"])

        if obj_versions:
            affected_objects.append({
                "bucket": obj["bucket"],
                "key": obj["key"],
                "size_bytes": obj["size_bytes"],
                "size_human": f"{obj['size_bytes'] / (1024*1024):.2f} MiB",
                "created_at": obj["created_at"],
                "affected_armor_versions": sorted(obj_versions),
                "affected_version_count": len(obj_versions),
            })

    print(f"  Found {len(affected_objects)} objects created during affected deployments")

    # Sort by creation time, then bucket, then key
    affected_objects.sort(key=lambda x: (x["created_at"], x["bucket"], x["key"]))

    # Save results
    print(f"\nSaving results to {OUTPUT_FILE}...")
    with open(OUTPUT_FILE, "w") as f:
        json.dump(affected_objects, f, indent=2)

    # Summary statistics
    print("\n=== Summary ===")
    print(f"Total objects analyzed: {len(objects)}")
    print(f"Objects >5MiB: {len(large_objects)}")
    print(f"Objects in affected windows: {len(affected_objects)}")

    # Count by bucket
    bucket_counts = {}
    for obj in affected_objects:
        bucket_counts[obj["bucket"]] = bucket_counts.get(obj["bucket"], 0) + 1

    print("\nBy bucket:")
    for bucket, count in sorted(bucket_counts.items()):
        print(f"  {bucket}: {count}")

    # Count by affected version
    version_counts = {}
    for obj in affected_objects:
        for version in obj["affected_armor_versions"]:
            version_counts[version] = version_counts.get(version, 0) + 1

    print("\nBy affected ARMOR version:")
    for version, count in sorted(version_counts.items()):
        print(f"  {version}: {count}")

    # Show sample of most affected objects (those that overlap with most versions)
    if affected_objects:
        most_affected = max(affected_objects, key=lambda x: x["affected_version_count"])
        print(f"\nMost affected object (overlaps with {most_affected['affected_version_count']} versions):")
        print(f"  Bucket: {most_affected['bucket']}")
        print(f"  Key: {most_affected['key']}")
        print(f"  Size: {most_affected['size_human']}")
        print(f"  Created: {most_affected['created_at']}")
        print(f"  Versions: {', '.join(most_affected['affected_armor_versions'])}")

    return 0


if __name__ == "__main__":
    sys.exit(main())
