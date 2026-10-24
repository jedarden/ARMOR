#!/bin/sh
# Executable parity gate for the operator-facing CLI contracts.
#
# The cross-binary package builds the real armor, restore-verifier, and
# armor-fleet executables and runs a table-driven matrix derived from the
# reference pages, including `armor check --json` probe records, the
# missing-SEAM-token failure classification, the shared -mek-ring grammar,
# and the argv rejection of the credential-value flags.  The command-package
# tests remain in this gate because they add the deeper HTTP, shutdown, and
# secret-safety checks for each companion binary.
#
# Every leg runs with -short: this gate is a definition-of-done --fast leg
# and must stay hermetic — no network, no browser.  The browser-mount
# smokes inside cmd/armor and cmd/armor-fleet skip under -short; the full
# gate owns them via scripts/verify-agentation-mount.sh, which fails on an
# all-skip run so the mount cannot go unverified.
set -u

cd "$(cd "$(dirname "$0")/.." && pwd)"

GO="${GO:-go}"
status=0

echo "+ $GO test -count=1 -short ./tests/cli-contract -run 'TestFleetMissingTokenIsOperationalFailure|TestDocumentedOutputEnvironmentAndSafetyContract'"
if ! "$GO" test -count=1 -short ./tests/cli-contract -run 'TestFleetMissingTokenIsOperationalFailure|TestDocumentedOutputEnvironmentAndSafetyContract'; then
	status=1
	echo "FAILED: focused machine-readable and companion exit contracts" >&2
fi

echo "+ $GO test -count=1 -short ./cmd/armor -run TestCheckJSONAndProbeFailureExitContract"
if ! "$GO" test -count=1 -short ./cmd/armor -run TestCheckJSONAndProbeFailureExitContract; then
	status=1
	echo "FAILED: armor check JSON and probe exit contract" >&2
fi

echo "+ $GO test -count=1 -short ./tests/cli-contract -run 'TestSecretFlagsAreRejectedNamingEnvVars|TestFlagVocabularyIsUniform'"
if ! "$GO" test -count=1 -short ./tests/cli-contract -run 'TestSecretFlagsAreRejectedNamingEnvVars|TestFlagVocabularyIsUniform'; then
	status=1
	echo "FAILED: secret-flag rejection and flag-vocabulary contracts" >&2
fi

echo "+ $GO test -count=1 -short ./internal/mekring"
if ! "$GO" test -count=1 -short ./internal/mekring; then
	status=1
	echo "FAILED: shared MEK-ring grammar" >&2
fi

echo "+ $GO test -count=1 -short ./tests/cli-contract ./cmd/armor ./cmd/restore-verifier ./cmd/armor-fleet"
if ! "$GO" test -count=1 -short ./tests/cli-contract ./cmd/armor ./cmd/restore-verifier ./cmd/armor-fleet; then
	status=1
	echo "FAILED: CLI and companion-binary contract parity" >&2
fi

exit "$status"
