#!/usr/bin/env python3
"""
Enumerate all objects >5MiB from the four never-audited ARMOR buckets:
iad-ci, iad-kalshi, rs-manager, armor-apexalgo.

Why this script exists
----------------------
Each ARMOR deployment is an S3-compatible proxy over a real Backblaze B2
bucket. The consumer-facing (logical) bucket name rarely equals the B2 bucket
name, so a naive `b2 ls b2://<logical-name>` enumerates the wrong thing (or
nothing — there is no B2 bucket named "iad-kalshi" or "rs-manager"). Earlier
per-bucket scripts also failed because they were run against an application
key restricted to the `iad-ci` bucket only, and silently wrote `[]`.

This script uses the master B2 key authorized in the `b2` CLI (accountId
20ad67013917, `buckets: null` → every bucket) and enumerates each logical
bucket via its confirmed B2 backing bucket.

Backing-bucket mapping (each confirmed by the ARMOR canary the deployment
writes into its own backing bucket under .armor/canary/<pod-rs>/):

  logical           B2 backing bucket   confirmation
  ----------------  ------------------  ----------------------------------------
  iad-ci            iad-ci              direct (names match; 694 objs)
  iad-kalshi        kalshi-tape         canary RS 55cd4bb764 is in iad-kalshi's
                                        armor ReplicaSet history
  rs-manager        nap-dashboard       canary markers + 3-way check (ADR-001,
                                        empty ARMOR_PREFIX; see git history)
  armor-apexalgo    armor-apexalgo      same-named B2 bucket

.armor/* keys are ARMOR bookkeeping (canaries, manifests) that ARMOR hides
from consumer-facing ListObjects responses, so they are excluded here too.
They are also all <5MiB, so the size filter drops them regardless, but
excluding explicitly matches the consumer-facing view.

Output
------
Per-bucket intermediate files (<logical>-objects.json) and a consolidated
unaudited-buckets-objects.json, each a JSON array of:
  {"bucket": "<logical>", "key": "...", "size_bytes": N, "created_at": "ISO8601"}

Usage
-----
  python3 scripts/enumerate-all-unaudited-buckets.py            # all four + consolidate
  python3 scripts/enumerate-all-unaudited-buckets.py iad-kalshi # one logical bucket
"""

import argparse
import json
import subprocess
import sys
from datetime import datetime, timezone
from typing import Dict, List, Optional

SIZE_THRESHOLD = 5 * 1024 * 1024  # 5 MiB in bytes

# logical (consumer-facing) name -> B2 backing bucket name
BUCKETS: Dict[str, str] = {
    "iad-ci": "iad-ci",
    "iad-kalshi": "kalshi-tape",
    "rs-manager": "nap-dashboard",
    "armor-apexalgo": "armor-apexalgo",
}


def enumerate_bucket(logical: str, backing: str) -> List[Dict[str, object]]:
    """Enumerate objects >5MiB from one B2 backing bucket."""
    objects: List[Dict[str, object]] = []

    print(f"Enumerating {logical} (B2 backing: {backing}) for objects >5MiB...",
          file=sys.stderr)

    cmd = ["b2", "ls", "--json", "--recursive", f"b2://{backing}"]
    # armor-apexalgo holds ~85k objects; allow ample time for a full listing.
    result = subprocess.run(cmd, capture_output=True, text=True, timeout=1800)

    if result.returncode != 0:
        # Surface the real error instead of silently producing an empty array
        # (the bug that masked the restricted-key failures the first time).
        print(f"FATAL enumerating {backing} ({logical}): {result.stderr.strip()}",
              file=sys.stderr)
        sys.exit(1)

    try:
        data = json.loads(result.stdout)
    except json.JSONDecodeError as e:
        print(f"FATAL: failed to parse JSON from {backing} ({logical}): {e}",
              file=sys.stderr)
        sys.exit(1)
    if not isinstance(data, list):
        print(f"FATAL: expected JSON array from {backing}, got {type(data)}",
              file=sys.stderr)
        sys.exit(1)

    total = 0
    bookkeeping = 0
    for obj in data:
        total += 1
        key = obj.get("fileName", "")
        size = obj.get("size", 0)

        # ARMOR hides its own .armor/* bookkeeping from consumers.
        if key.startswith(".armor/"):
            bookkeeping += 1
            continue

        if size > SIZE_THRESHOLD:
            upload_ts = obj.get("uploadTimestamp")
            if upload_ts:
                created_at = datetime.fromtimestamp(
                    upload_ts / 1000, tz=timezone.utc
                ).isoformat()
            else:
                created_at = None

            objects.append({
                "bucket": logical,
                "key": key,
                "size_bytes": size,
                "created_at": created_at,
            })

    print(f"  scanned {total} objects ({bookkeeping} .armor/* excluded), "
          f"{len(objects)} >5MiB", file=sys.stderr)
    return objects


def validate(objects: List[Dict[str, object]]) -> None:
    """Validate structure + ISO8601 timestamps; exit nonzero on any problem."""
    required = {"bucket", "key", "size_bytes", "created_at"}
    bad = 0
    for i, o in enumerate(objects):
        if set(o.keys()) != required:
            print(f"VALIDATION ERROR entry {i}: keys {sorted(o.keys())} != {sorted(required)}",
                  file=sys.stderr)
            bad += 1
            continue
        if not isinstance(o["size_bytes"], int) or o["size_bytes"] <= SIZE_THRESHOLD:
            print(f"VALIDATION ERROR entry {i}: size {o['size_bytes']} not > {SIZE_THRESHOLD}",
                  file=sys.stderr)
            bad += 1
            continue
        ts = o["created_at"]
        if not isinstance(ts, str) or not ts:
            print(f"VALIDATION ERROR entry {i}: missing created_at", file=sys.stderr)
            bad += 1
            continue
        try:
            datetime.fromisoformat(ts)
        except ValueError:
            print(f"VALIDATION ERROR entry {i}: bad ISO8601 timestamp {ts!r}",
                  file=sys.stderr)
            bad += 1
    if bad:
        print(f"\n{bad} validation errors; aborting.", file=sys.stderr)
        sys.exit(1)
    print(f"Validation OK: {len(objects)} entries, all ISO8601 + structural checks passed.",
          file=sys.stderr)


def write_json(path: str, data: object) -> None:
    with open(path, "w") as f:
        json.dump(data, f, indent=2)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("bucket", nargs="?", choices=sorted(BUCKETS),
                        help="enumerate a single logical bucket (default: all + consolidate)")
    args = parser.parse_args()

    targets = [args.bucket] if args.bucket else list(BUCKETS)

    per_bucket: Dict[str, List[Dict[str, object]]] = {}
    for logical in targets:
        objs = enumerate_bucket(logical, BUCKETS[logical])
        per_bucket[logical] = objs
        write_json(f"{logical}-objects.json", objs)
        print(f"  wrote {logical}-objects.json ({len(objs)} objects)", file=sys.stderr)

    if args.bucket:
        # single-bucket mode: still validate what we produced
        validate(per_bucket[args.bucket])
        return

    # consolidate + validate
    consolidated: List[Dict[str, object]] = []
    for logical in BUCKETS:  # stable order
        consolidated.extend(per_bucket[logical])

    validate(consolidated)
    write_json("unaudited-buckets-objects.json", consolidated)

    total_bytes = sum(o["size_bytes"] for o in consolidated)  # type: ignore[arg-type]
    print(file=sys.stderr)
    print("Per-bucket object counts (>5MiB):", file=sys.stderr)
    for logical in BUCKETS:
        n = len(per_bucket[logical])
        gb = sum(o["size_bytes"] for o in per_bucket[logical]) / (1024 ** 3)  # type: ignore[arg-type]
        print(f"  {logical:16s} {n:6d} objects  {gb:8.2f} GiB", file=sys.stderr)
    print(f"\nConsolidated: {len(consolidated)} objects, "
          f"{total_bytes / (1024 ** 3):.2f} GiB total", file=sys.stderr)
    print("Wrote unaudited-buckets-objects.json", file=sys.stderr)


if __name__ == "__main__":
    main()
