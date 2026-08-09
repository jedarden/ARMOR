#!/usr/bin/env python3
"""
Enumerates all objects >5MiB from the rs-manager ARMOR bucket.

Background — why this does NOT query b2://rs-manager:
  There is no B2 bucket named "rs-manager". Each ARMOR deployment is an
  S3-compatible proxy over a real B2 bucket. The rs-manager deployment
  (namespace `armor` on the rs-manager cluster) is backed by the B2 bucket
  `nap-dashboard`, with an *empty* ARMOR_PREFIX — confirmed three ways:
    1. The rs-manager armor pod ReplicaSet (armor-6bd9cbf74d-*) appears as the
       canary writer inside nap-dashboard (.armor/canary/armor-6bd9cbf74d-*).
    2. Canary keys live at the root .armor/ level (not <prefix>/.armor/),
       which only happens when ARMOR_PREFIX is unset (ADR-001).
    3. The restore-verifier-acb manifest comment in declarative-config refers
       to "rs-manager's own nap-dashboard bucket".

  Because the prefix is empty, raw B2 keys are identical to the keys ARMOR
  exposes to consumers, so a direct B2 listing of nap-dashboard is the
  authoritative enumeration of the rs-manager bucket. The master B2 key used
  by the b2 CLI can read every bucket, so no port-forward or cluster secret is
  needed (unlike the ARMOR-HTTP variant in enumerate-rs-manager-armor.py).

  .armor/* keys are ARMOR bookkeeping (canaries, manifests) and are excluded
  from ARMOR's consumer-facing ListObjects responses, so they are excluded
  here too.

Output format: [{"bucket": "rs-manager", "key": "...", "size_bytes": N, "created_at": "ISO8601"}, ...]
"""

import subprocess
import json
import sys
from datetime import datetime, timezone
from typing import List, Dict, Any

SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes

# Logical (consumer-facing) bucket name vs. the real B2 backing bucket.
LOGICAL_BUCKET = "rs-manager"
BACKING_BUCKET = "nap-dashboard"


def enumerate_rs_manager_bucket() -> List[Dict[str, Any]]:
    """Enumerate objects from the rs-manager bucket via its B2 backing bucket."""
    objects: List[Dict[str, Any]] = []

    print(f"Enumerating rs-manager bucket (B2 backing bucket: {BACKING_BUCKET}) "
          f"for objects >5MiB...", file=sys.stderr)

    cmd = ["b2", "ls", "--json", "--recursive", f"b2://{BACKING_BUCKET}"]
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=600)

    if result.returncode != 0:
        print(f"Error enumerating {BACKING_BUCKET}: {result.stderr}", file=sys.stderr)
        return []

    try:
        data = json.loads(result.stdout)
        if not isinstance(data, list):
            print(f"Warning: expected JSON array from {BACKING_BUCKET}, "
                  f"got {type(data)}", file=sys.stderr)
            return []
    except json.JSONDecodeError as e:
        print(f"Warning: failed to parse JSON from {BACKING_BUCKET}: {e}", file=sys.stderr)
        return []

    total_objects = 0
    bookkeeping = 0
    for obj in data:
        total_objects += 1
        key = obj.get("fileName", "")
        size = obj.get("size", 0)

        # ARMOR hides its own .armor/* bookkeeping from consumers.
        if key.startswith(".armor/"):
            bookkeeping += 1
            continue

        if size > SIZE_THRESHOLD:
            # b2 reports uploadTimestamp as ms since epoch.
            upload_ts = obj.get("uploadTimestamp")
            if upload_ts:
                created_at = datetime.fromtimestamp(
                    upload_ts / 1000, tz=timezone.utc
                ).isoformat()
            else:
                created_at = None

            objects.append({
                "bucket": LOGICAL_BUCKET,
                "key": key,
                "size_bytes": size,
                "created_at": created_at,
            })

    print(f"Total objects scanned: {total_objects} "
          f"({bookkeeping} .armor/* bookkeeping excluded)", file=sys.stderr)
    print(f"Large objects found (>5MiB): {len(objects)}", file=sys.stderr)

    return objects


def main() -> None:
    print(f"Enumerating rs-manager bucket for objects > "
          f"{SIZE_THRESHOLD / (1024 * 1024)} MiB", file=sys.stderr)

    objects = enumerate_rs_manager_bucket()

    output = objects
    print(json.dumps(output, indent=2))

    output_file = "rs-manager-objects.json"
    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    print(f"\nTotal large objects found: {len(objects)}", file=sys.stderr)
    print(f"Saved to: {output_file}", file=sys.stderr)


if __name__ == "__main__":
    main()
