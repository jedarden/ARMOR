#!/usr/bin/env python3
"""
Filter unaudited-buckets-objects.json to only include objects created during
the affected deployment windows identified in drift-timeline.json.

The bug window is 2026-03-24 to 2026-07-16, but we only had vulnerable
deployments from 2026-03-28 (first v0.1.0) to 2026-06-11 (v0.1.42).

Objects must be >5MiB (already filtered in source) AND created within
the affected deployment windows.

Output: scratch/candidate-objects.json
"""

import json
import sys
from datetime import datetime, timezone
from typing import List, Dict

# Affected deployment windows from drift-timeline.json
# Bug window: 2026-03-24 to 2026-07-16
# Actual affected deployments: 2026-03-28 to 2026-06-11
BUG_WINDOW_START = "2026-03-24T00:00:00Z"
BUG_WINDOW_END = "2026-07-16T23:59:59Z"

def parse_timestamp(ts: str) -> datetime:
    """Parse ISO8601 timestamp to datetime object."""
    if ts.endswith("Z"):
        ts = ts[:-1] + "+00:00"
    return datetime.fromisoformat(ts)

def is_within_bug_window(created_at: str) -> bool:
    """Check if object was created within the bug window."""
    try:
        obj_time = parse_timestamp(created_at)
        window_start = parse_timestamp(BUG_WINDOW_START)
        window_end = parse_timestamp(BUG_WINDOW_END)
        return window_start <= obj_time <= window_end
    except (ValueError, AttributeError) as e:
        print(f"Warning: failed to parse timestamp {created_at}: {e}", file=sys.stderr)
        return False

def main():
    # Load the full enumeration
    with open("unaudited-buckets-objects.json", "r") as f:
        all_objects = json.load(f)

    print(f"Loaded {len(all_objects)} objects from unaudited-buckets-objects.json", file=sys.stderr)

    # Filter by bug window
    candidates = []
    for obj in all_objects:
        created_at = obj.get("created_at")
        if created_at and is_within_bug_window(created_at):
            candidates.append(obj)

    print(f"Filtered to {len(candidates)} candidates within bug window", file=sys.stderr)

    # Per-bucket breakdown
    bucket_counts = {}
    bucket_sizes = {}
    for obj in candidates:
        bucket = obj["bucket"]
        bucket_counts[bucket] = bucket_counts.get(bucket, 0) + 1
        bucket_sizes[bucket] = bucket_sizes.get(bucket, 0) + obj["size_bytes"]

    print("\nPer-bucket candidate counts:", file=sys.stderr)
    for bucket in sorted(bucket_counts.keys()):
        count = bucket_counts[bucket]
        gb = bucket_sizes[bucket] / (1024 ** 3)
        print(f"  {bucket:16s} {count:6d} objects  {gb:8.2f} GiB", file=sys.stderr)

    total_bytes = sum(obj["size_bytes"] for obj in candidates)
    print(f"\nTotal: {len(candidates)} candidates, {total_bytes / (1024 ** 3):.2f} GiB", file=sys.stderr)

    # Write output
    with open("scratch/candidate-objects.json", "w") as f:
        json.dump(candidates, f, indent=2)

    print("\nWrote scratch/candidate-objects.json", file=sys.stderr)

if __name__ == "__main__":
    main()
