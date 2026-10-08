#!/bin/sh
set -eu

# Release-critical gate for the ARMOR server image.  The repository still has
# unrelated legacy CLI/dashboard/fleet tests awaiting repair; those must not
# hide regressions in the encryption and multipart paths used by production.
# Keep this gate narrow, deterministic, and shared by CI and the Docker build.

race_flag=""
if [ "${ARMOR_RELEASE_RACE:-0}" = "1" ]; then
	race_flag="-race"
fi

# Repository-level legs (script gates, docs parity, publisher contract). The
# Docker build context (.dockerignore) ships only the Go source and this
# script, so the Dockerfile sets ARMOR_GATE_SCOPE=image and runs the Go legs
# below alone; CI's go-test step and the definition of done run the full gate
# on the same tree, so the image is never built from a tree that skipped them.
if [ "${ARMOR_GATE_SCOPE:-full}" = "full" ]; then
	# Toolchain parity (armor-255e8f31): the builder base images must pin go.mod's
	# `toolchain` directive — drift here means the image compiles with a different
	# Go than the repository declares. Cheapest gate, so it runs first; also in
	# the definition of done, `make docker`, and its own pytest suite.
	./scripts/toolchain-parity.sh

	# compose.yaml ↔ VERSION parity (armor-3c849ea3): the tracked compose file
	# pins its demo and production image defaults to the VERSION file, and
	# cut-release.sh bumps them in the release commit. Gate it here so no image
	# is ever built from a tree whose compose pin lags VERSION.
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

go test ${race_flag} -count=1 ./internal/crypto \
	-run '^(TestV3Counter|TestV3BlockHMACKeys|TestV3MaxBlockSizeConstraint|TestV3HMACInputFormat)$'

go test ${race_flag} -count=1 ./internal/backend \
	-run '^TestMultipartV3|^TestMultipartV2Format$|^TestFSBackend_MultipartUpload$|^TestB2PutIfAbsentForwardsAtomicCondition$|^TestFSBackendPutIfAbsentDoesNotOverwrite$'

# Restore discovery (armor-8290de05): getLatestObject must continue past list
# pages the backend returns EMPTY because every key on them was .armor/*
# internal bookkeeping — an unprefixed bucket whose lexicographic head is
# thousands of canary objects otherwise blinds the verifier completely.
go test ${race_flag} -count=1 ./internal/restoreverifier \
	-run '^TestGetLatestObject'

go test ${race_flag} -count=1 ./internal/canary

# Key-ring variables share the ARMOR_MEK_ prefix with named keys. Keep their
# parsing in the release gate so a deployable image accepts both empty and
# populated default/named rings.
go test -count=1 ./internal/config \
  -run '^TestLoadWith(KeyRing|EmptyKeyRing|NamedKeyRing|RingValidationErrors)$'

# The S3 API streams multi-gigabyte operations. A server-wide WriteTimeout
# terminates CompleteMultipartUpload while the backing store is still
# composing the object, so keep the timeout policy in the release gate.
go test -count=1 ./cmd/armor \
	-run '^TestNewS3HTTPServerAllowsLongRunningRequestsAndResponses$'

go test ${race_flag} -count=1 ./internal/server/handlers \
	-run '^TestMultipartV3HTTPConcurrentShuffledUnalignedRoundTrip$|^TestMultipartV3|^TestV3GetObject|^TestV3FilesystemPutGetRoundTrip$|^TestPutObjectIfNoneMatchCreateOnlySmallAndStreaming$|^TestPutObjectRejectsUnsupportedIfNoneMatchValue$|^TestPutObjectDoesNotEmulateConditionalWrite$'

# Compile the credentialed B2 integration suite without contacting a backend.
go test -count=1 -tags=integration ./tests/integration/... -run '^$'
