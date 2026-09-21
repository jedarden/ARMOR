#!/usr/bin/env bash
# Repository definition of done for ARMOR.
#
# Usage:
#   scripts/definition-of-done.sh --fast   # build + vet + the fast test suites
#   scripts/definition-of-done.sh          # --fast plus the full short Go suite
#
# --fast runs everything deterministic and quick: the Go build and vet and
# the Python scripts test suite (tests/test_drift_check.py). The default mode
# adds `go test ./... -short`. CI (iad-ci armor-build) runs the containerized
# build/lint legs; this script is the local gate. The pytest entry point is
# `python3 -m pytest` rather than the `pytest` shim, whose shebang is stale on
# NixOS hosts.
set -uo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GO="${GO:-go}"
PY="${PY:-python3}"

fast=0
[ "${1:-}" = "--fast" ] && fast=1

fail=0
run() {
  echo "+ $*"
  if ! "$@"; then
    echo "FAILED: $*" >&2
    fail=1
  fi
}

go_build_flags=()
if [ ! -d .git ]; then
  # git archive verification has no .git directory. Go VCS stamping otherwise
  # fails before compiling any package.
  go_build_flags=(-buildvcs=false)
fi

echo "== ARMOR definition of done ($([ "$fast" = 1 ] && echo fast || echo full)) =="

run "$GO" build "${go_build_flags[@]}" ./...
run "$GO" vet "${go_build_flags[@]}" ./...
run "$PY" -m pytest tests/test_drift_check.py -q

if [ "$fast" = 0 ]; then
  run "$GO" test ./... -short
fi

exit "$fail"
