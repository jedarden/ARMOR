#!/usr/bin/env python3
"""
Generate structured filtered object list output with validation and statistics.

This script:
1. Loads filtered objects from cross-reference output
2. Validates completeness of all required fields
3. Generates output in both JSON and CSV formats
4. Creates summary statistics
"""

import json
import csv
from pathlib import Path
from collections import Counter
from typing import Any, Dict, List


def validate_object(obj: Dict[str, Any], idx: int) -> List[str]:
    """Validate a single object has all required fields."""
    errors = []
    required_fields = {
        "bucket": str,
        "key": str,
        "size_bytes": int,
        "created_at": str,
        "affected_armor_versions": list,
    }

    for field, expected_type in required_fields.items():
        if field not in obj:
            errors.append(f"Object {idx}: Missing field '{field}'")
        elif not isinstance(obj[field], expected_type):
            errors.append(
                f"Object {idx}: Field '{field}' has wrong type "
                f"(expected {expected_type.__name__}, got {type(obj[field]).__name__})"
            )

    # Specific validation for affected_armor_versions not empty
    if "affected_armor_versions" in obj and isinstance(obj["affected_armor_versions"], list):
        if len(obj["affected_armor_versions"]) == 0:
            errors.append(f"Object {idx}: Field 'affected_armor_versions' is empty")

    return errors


def main():
    # Paths
    input_file = Path("/home/coding/ARMOR/intermediate/filtered-objects.json")
    output_dir = Path("/home/coding/ARMOR/intermediate")
    output_dir.mkdir(parents=True, exist_ok=True)

    # Load filtered objects
    print(f"Loading filtered objects from {input_file}...")
    with open(input_file, "r") as f:
        objects = json.load(f)

    print(f"Loaded {len(objects)} objects")

    # Validate all objects
    print("Validating objects...")
    all_errors = []
    for idx, obj in enumerate(objects):
        errors = validate_object(obj, idx)
        all_errors.extend(errors)

    if all_errors:
        print(f"\n❌ Validation failed with {len(all_errors)} errors:")
        for error in all_errors[:10]:  # Show first 10 errors
            print(f"  {error}")
        if len(all_errors) > 10:
            print(f"  ... and {len(all_errors) - 10} more errors")
        raise ValueError("Validation failed")
    else:
        print("✅ All objects validated successfully")

    # Generate filtered_objects.json (canonical output)
    output_json = output_dir / "filtered_objects.json"
    print(f"\nWriting JSON output to {output_json}...")
    with open(output_json, "w") as f:
        json.dump(objects, f, indent=2)
    print(f"✅ Wrote {len(objects)} objects to JSON")

    # Generate filtered_objects.csv
    output_csv = output_dir / "filtered_objects.csv"
    print(f"\nWriting CSV output to {output_csv}...")
    with open(output_csv, "w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=[
            "bucket", "object_key", "size_bytes", "created_at", "affected_armor_versions"
        ])
        writer.writeheader()

        for obj in objects:
            writer.writerow({
                "bucket": obj["bucket"],
                "object_key": obj["key"],
                "size_bytes": obj["size_bytes"],
                "created_at": obj["created_at"],
                "affected_armor_versions": json.dumps(obj["affected_armor_versions"])
            })
    print(f"✅ Wrote {len(objects)} objects to CSV")

    # Generate summary statistics
    print("\nCalculating summary statistics...")

    total_count = len(objects)
    total_size_bytes = sum(obj["size_bytes"] for obj in objects)

    # Count by bucket
    counts_by_bucket = Counter(obj["bucket"] for obj in objects)

    # Count by affected version
    counts_by_version = Counter()
    for obj in objects:
        for version in obj["affected_armor_versions"]:
            counts_by_version[version] += 1

    # Convert Counters to regular dicts for JSON serialization
    counts_by_bucket = dict(sorted(counts_by_bucket.items()))
    counts_by_version = dict(sorted(counts_by_version.items()))

    # Calculate size in human-readable format
    def format_size(bytes_size: int) -> str:
        for unit in ["B", "KB", "MB", "GB", "TB"]:
            if bytes_size < 1024.0:
                return f"{bytes_size:.2f} {unit}"
            bytes_size /= 1024.0
        return f"{bytes_size:.2f} PB"

    summary = {
        "total_count": total_count,
        "total_size_bytes": total_size_bytes,
        "total_size_human_readable": format_size(total_size_bytes),
        "counts_by_bucket": counts_by_bucket,
        "counts_by_version": counts_by_version,
    }

    output_summary = output_dir / "summary.json"
    print(f"\nWriting summary to {output_summary}...")
    with open(output_summary, "w") as f:
        json.dump(summary, f, indent=2)

    print(f"\n✅ Summary statistics:")
    print(f"  Total objects: {summary['total_count']}")
    print(f"  Total size: {summary['total_size_human_readable']} ({summary['total_size_bytes']} bytes)")
    print(f"  Buckets: {list(summary['counts_by_bucket'].keys())}")
    print(f"  Versions: {list(summary['counts_by_version'].keys())}")

    print(f"\n✅ All outputs generated successfully:")
    print(f"  - {output_json}")
    print(f"  - {output_csv}")
    print(f"  - {output_summary}")


if __name__ == "__main__":
    main()
