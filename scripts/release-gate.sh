#!/bin/sh
set -eu

# Release-critical gate for the ARMOR server image. Keep this gate deterministic
# and shared by CI and the Docker build.

race_flag=""
if [ "${ARMOR_RELEASE_RACE:-0}" = "1" ]; then
	race_flag="-race"
fi
# ARMOR_RELEASE_RACE is a gate control, not an ARMOR runtime setting. Keep it
# out of child test processes so configuration's unknown-ARMOR_* warning test
# does not mistake the CI control variable for an application variable.
unset ARMOR_RELEASE_RACE

# Repository-level legs (script gates, docs parity, publisher contract). The
# Docker build context (.dockerignore) ships the Go source, docs/ — the
# server leg's doc-parity tests read docs/http-api-reference.md — and this
# script, so the Dockerfile sets ARMOR_GATE_SCOPE=image and runs the Go legs
# below alone; CI's go-test step and the definition of done run the full gate
# on the same tree, so the image is never built from a tree that skipped them.
if [ "${ARMOR_GATE_SCOPE:-full}" = "full" ]; then
	# Toolchain parity (armor-255e8f31): the builder base images must pin go.mod's
	# `toolchain` directive — drift here means the image compiles with a different
	# Go than the repository declares. Cheapest gate, so it runs first; also in
	# the definition of done, `make docker`, and its own pytest suite.
	./scripts/toolchain-parity.sh

	# Consumer image-pin parity (armor-3c849ea3, armor-19792903): the README
	# export, the tracked compose file's demo/production image defaults and
	# the release-status latest-green marker must name one fully published,
	# compat-green release. Gate it here so no image is ever built from a
	# tree whose consumer pins disagree.
	./scripts/compose-version-parity.sh

	# Prohibited deployment constructs (armor-7a6ffd3c): no .github/workflows
	# files, kind: Job / kind: CronJob manifests, or :latest / unpinned
	# ronaldraygun image references — org hard rules, enforced on repository
	# content before anything is built from it.
	./scripts/prohibited-constructs-gate.sh

	# Documentation status (armor-abff8d6e, armor-749804d5): operator pages may
	# describe repository-tested behavior, but cannot promote pending capabilities
	# to release-verified/fully supported or reduce an active regression to a
	# historical-only note without the evidence update.
	./scripts/documentation-status-gate.sh

	# Dockerfile image contract (armor-33bc86b9): an untargeted build publishes
	# the LAST Dockerfile stage as ronaldraygun/armor, so that stage must be
	# the armor server (ENTRYPOINT ["/armor"], no CMD) and the companion
	# --target stages (restore-verifier-runtime, armor-fleet-runtime) must stay
	# addressable. Images 0.1.1833-0.1.1870 shipped /restore-verifier as the
	# default entrypoint because a runtime stage sat last after a multi-stage
	# refactor; this script runs in the builder stage, so the image build
	# itself fails before that can happen again.
	./scripts/image-contract-gate.sh

	# CLI contract parity (armor-a48b2e70): build the real armor,
	# restore-verifier, and armor-fleet binaries and exercise the documented
	# flags, inputs, outputs, exit codes, environment handling, HTTP surfaces,
	# shutdown paths, and secret-safety boundaries.
	./scripts/cli-contract-gate.sh

	# Publisher contract tests (armor-562d57c9): the armor-build publish-release
	# step runs scripts/publish_release.py to cut the annotated tag and both
	# releases. These pin its contract — idempotency, version/tag consistency,
	# release artifact coverage, and rejection of floating tags — against fake
	# endpoints, network-free. Needs python3 + pytest: the Dockerfile builder
	# stage and CI's go-test step install them for this leg.
	python3 -m pytest tests/test_publish_release.py -q
else
	echo "release-gate: ARMOR_GATE_SCOPE=${ARMOR_GATE_SCOPE}; skipping repository-level legs (full gate runs in CI and the definition of done)"
fi

go vet ./...

# Exercise every package's short-test suite so the release gate covers all
# binaries and libraries, including the dashboards and fleet console.
if [ ! -f docs/http-api-reference.md ]; then
	echo "FAIL: docs/http-api-reference.md missing from the build context; the server leg's doc-parity tests read docs/ (.dockerignore drift?)" >&2
	exit 1
fi
go test ${race_flag} -count=1 -short ./...
