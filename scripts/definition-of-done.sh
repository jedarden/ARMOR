#!/usr/bin/env bash
# Repository definition of done for ARMOR.
#
# Usage:
#   scripts/definition-of-done.sh --fast   # the hermetic local gate
#   scripts/definition-of-done.sh          # --fast plus the full short Go suite
#   make dod / make dod-full               # the same two lanes via make
#
# --fast is the hermetic lane: everything it runs is deterministic, quick,
# and offline — no network, no browser. It runs the toolchain parity gate
# (scripts/toolchain-parity.sh), the compose.yaml ↔ VERSION parity gate
# (scripts/compose-version-parity.sh), the prohibited deployment constructs
# gate (scripts/prohibited-constructs-gate.sh), the Dockerfile image
# contract gate (scripts/image-contract-gate.sh), the documentation status
# gate (scripts/documentation-status-gate.sh), the executable CLI and
# companion-binary contract gate (scripts/cli-contract-gate.sh — its go
# test legs run with -short, so the browser-mount smokes inside cmd/ skip),
# the Go build and vet, and the Python scripts test suite: the suites
# enumerated in tests/gate-suites.txt, the single edit point for the gate
# inventory (tests/test_gate_inventory.py holds every documented surface
# to that file and to the invocation below).
#
# The default (full) mode adds `go test ./... -short` and the Agentation
# browser mount smoke (scripts/verify-agentation-mount.sh): the proxy
# dashboard, demo dashboard, and fleet console each load in a headless
# Chromium and must show #agentation-root mounted in the rendered DOM.
# That leg needs a browser binary and esm.sh reachability — exactly the
# two things --fast must not need — and deliberately fails, never silently
# skips, without them, because an unverified mount is what this gate
# exists to catch. CI (iad-ci armor-build) runs the containerized
# build/lint legs; this script is the local gate.
#
# Each stage prints its duration after it finishes and the run prints a
# total, so wall-clock regressions are visible (the --fast lane targets
# < 90 s on codinghome). The pytest entry point is `python3 -m pytest`
# rather than the `pytest` shim, whose shebang is stale on NixOS hosts.
set -uo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

GO="${GO:-go}"
PY="${PY:-python3}"

# The Python gate inventory: one suite per line in this file. It is the
# single edit point — the pytest leg below executes exactly these suites,
# in file order.
gate_suites_file=tests/gate-suites.txt

fast=0
[ "${1:-}" = "--fast" ] && fast=1

fail=0
total_ms=0

run() {
  echo "+ $*"
  local started elapsed
  started=$(date +%s%N)
  if ! "$@"; then
    echo "FAILED: $*" >&2
    fail=1
  fi
  elapsed=$(( ($(date +%s%N) - started) / 1000000 ))
  total_ms=$((total_ms + elapsed))
  printf '[%d.%01ds] %s\n' $((elapsed / 1000)) $(((elapsed % 1000) / 100)) "$*"
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
run ./scripts/image-contract-gate.sh
run ./scripts/documentation-status-gate.sh
run ./scripts/cli-contract-gate.sh
run "$GO" build "${go_build_flags[@]}" ./...
run "$GO" vet "${go_build_flags[@]}" ./...

gate_suites=()
if [ -f "$gate_suites_file" ]; then
  while IFS= read -r suite || [ -n "$suite" ]; do
    case "$suite" in
      ''|'#'*) continue ;;
    esac
    gate_suites+=("$suite")
  done < "$gate_suites_file"
else
  echo "FAILED: $gate_suites_file (the Python gate inventory) is missing" >&2
  fail=1
fi

for suite in "${gate_suites[@]}"; do
  if [ ! -f "$suite" ]; then
    echo "FAILED: gate suite $suite (listed in $gate_suites_file) does not exist" >&2
    fail=1
  fi
done

if [ "${#gate_suites[@]}" -gt 0 ]; then
  run "$PY" -m pytest "${gate_suites[@]}" -q
else
  echo "FAILED: $gate_suites_file lists no suites" >&2
  fail=1
fi

if [ "$fast" = 0 ]; then
  run "$GO" test ./... -short
  # Runtime Agentation smoke: load every UI entry point in a real browser
  # and require the toolbar to have mounted (see the header note for why
  # this is a full-mode leg).
  run ./scripts/verify-agentation-mount.sh
fi

printf '== definition of done (%s): %d.%01ds total ==\n' \
  "$([ "$fast" = 1 ] && echo fast || echo full)" \
  $((total_ms / 1000)) $(((total_ms % 1000) / 100))

exit "$fail"
