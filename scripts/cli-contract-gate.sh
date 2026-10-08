#!/bin/sh
# Executable parity gate for the operator-facing CLI contracts.
#
# The cross-binary package builds the real armor, restore-verifier, and
# armor-fleet executables and runs a table-driven matrix derived from the
# reference pages.  The command-package tests remain in this gate because
# they add the deeper HTTP, shutdown, and secret-safety checks for each
# companion binary.
set -u

cd "$(cd "$(dirname "$0")/.." && pwd)"

GO="${GO:-go}"
status=0

echo "+ $GO test -count=1 ./tests/cli-contract ./cmd/armor ./cmd/restore-verifier ./cmd/armor-fleet"
if ! "$GO" test -count=1 ./tests/cli-contract ./cmd/armor ./cmd/restore-verifier ./cmd/armor-fleet; then
	status=1
	echo "FAILED: CLI and companion-binary contract parity" >&2
fi

exit "$status"
