#!/usr/bin/env bash
# Repository definition of done for ARMOR.
#
# Usage:
#   scripts/definition-of-done.sh --fast   # build + vet + the fast test suites
#   scripts/definition-of-done.sh          # --fast plus the full short Go suite
#
# --fast runs everything deterministic and quick: the toolchain parity gate
# (scripts/toolchain-parity.sh), the compose.yaml ↔ VERSION parity gate
# (scripts/compose-version-parity.sh), the prohibited deployment constructs
# gate (scripts/prohibited-constructs-gate.sh), the Go build and vet, and the
# Python scripts test suite (tests/test_drift_check.py,
# tests/test_toolchain_parity.py, tests/test_compose_version_parity.py,
# tests/test_cut_release.py, tests/test_restore_verifier_inventory.py,
# tests/test_restore_verifier_scope_validation.py,
# tests/test_prohibited_constructs.py, tests/test_gate_inventory.py,
# tests/test_publish_release.py). The
# default mode adds `go test ./... -short`. CI (iad-ci armor-build) runs the
# containerized build/lint legs; this script is the local gate. The pytest
# entry point is `python3 -m pytest` rather than the `pytest` shim, whose
# shebang is stale on NixOS hosts.
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

run ./scripts/toolchain-parity.sh
run ./scripts/compose-version-parity.sh
run ./scripts/prohibited-constructs-gate.sh
run "$GO" build "${go_build_flags[@]}" ./...
run "$GO" vet "${go_build_flags[@]}" ./...
run "$PY" -m pytest tests/test_drift_check.py tests/test_toolchain_parity.py tests/test_compose_version_parity.py tests/test_cut_release.py tests/test_restore_verifier_inventory.py tests/test_restore_verifier_scope_validation.py tests/test_prohibited_constructs.py tests/test_gate_inventory.py tests/test_publish_release.py -q

if [ "$fast" = 0 ]; then
  run "$GO" test ./... -short
fi

exit "$fail"
