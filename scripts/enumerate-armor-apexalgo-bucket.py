#!/usr/bin/env python3
"""
Enumerates all objects >5MiB from the armor-apexalgo ARMOR bucket.

armor-apexalgo is a real B2 bucket (region us-west-002) whose B2 bucket name
matches its consumer-facing ARMOR bucket name — unlike rs-manager (which is
backed by the nap-dashboard B2 bucket; see enumerate-rs-manager-bucket.py).
Because the names coincide and there is no ARMOR_PREFIX rewrite, a direct B2
listing of armor-apexalgo is the authoritative enumeration of the bucket. The
master B2 key cached in the b2 CLI can read every bucket, so no port-forward
or cluster secret is needed (unlike the ARMOR-HTTP variant).

.armor/* keys are ARMOR bookkeeping (canaries, manifests) and are excluded
from ARMOR's consumer-facing ListObjects responses, so they are excluded here
too. They are all small (<5MiB), so the size filter already drops them, but
excluding explicitly matches the consumer-facing view and the rs-manager
script's convention.

Output format: [{"bucket": "armor-apexalgo", "key": "...", "size_bytes": N, "created_at": "ISO8601"}, ...]
"""

import subprocess
import json
import sys
from datetime import datetime, timezone
from typing import List, Dict, Any

SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes

# armor-apexalgo's B2 backing bucket has the same name as its ARMOR bucket.
LOGICAL_BUCKET = "armor-apexalgo"
BACKING_BUCKET = "armor-apexalgo"


def enumerate_armor_apexalgo_bucket() -> List[Dict[str, Any]]:
    """Enumerate objects from armor-apexalgo via its (same-named) B2 bucket."""
    objects: List[Dict[str, Any]] = []

    print(f"Enumerating armor-apexalgo bucket (B2 backing bucket: {BACKING_BUCKET}) "
          f"for objects >5MiB...", file=sys.stderr)

    cmd = ["b2", "ls", "--json", "--recursive", f"b2://{BACKING_BUCKET}"]
    # armor-apexalgo holds ~85k objects; allow ample time for the full listing.
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)

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
    print(f"Enumerating armor-apexalgo bucket for objects > "
          f"{SIZE_THRESHOLD / (1024 * 1024)} MiB", file=sys.stderr)

    objects = enumerate_armor_apexalgo_bucket()

    output = objects
    print(json.dumps(output, indent=2))

    output_file = "armor-apexalgo-objects.json"
    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    print(f"\nTotal large objects found: {len(objects)}", file=sys.stderr)
    print(f"Saved to: {output_file}", file=sys.stderr)


if __name__ == "__main__":
    main()
