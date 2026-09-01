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

go vet ./...

go test ${race_flag} -count=1 ./internal/crypto \
	-run '^(TestV3Counter|TestV3BlockHMACKeys|TestV3MaxBlockSizeConstraint|TestV3HMACInputFormat)$'

go test ${race_flag} -count=1 ./internal/backend \
	-run '^TestMultipartV3|^TestMultipartV2Format$|^TestFSBackend_MultipartUpload$'

go test ${race_flag} -count=1 ./internal/canary

# Key-ring variables share the ARMOR_MEK_ prefix with named keys. Keep their
# parsing in the release gate so a deployable image accepts both empty and
# populated default/named rings.
go test -count=1 ./internal/config \
  -run '^TestLoadWith(KeyRing|EmptyKeyRing|NamedKeyRing|RingValidationErrors)$'

go test ${race_flag} -count=1 ./internal/server/handlers \
	-run '^TestMultipartV3HTTPConcurrentShuffledUnalignedRoundTrip$|^TestMultipartV3|^TestV3GetObject|^TestV3FilesystemPutGetRoundTrip$'

# Compile the credentialed B2 integration suite without contacting a backend.
go test -count=1 -tags=integration ./tests/integration/... -run '^$'
