#!/bin/sh
# Keep release-status and known-limitations claims honest in operator docs.
# This gate is intentionally separate from docs-index/link validation: a link
# can be valid while the surrounding documentation overstates its evidence.
# Wired into the definition-of-done and release gates.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
exec python3 "$ROOT/scripts/documentation-status-gate.py" --root "$ROOT"
