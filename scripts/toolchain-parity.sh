#!/bin/sh
# Toolchain parity gate: go.mod's `toolchain` directive is the single source
# of the Go version, and every golang:<tag> base image in Dockerfile and
# Dockerfile.test must pin exactly that version (AGENTS.md, "Build, test, and
# the gates"). `make toolchain-check` only warns, and only about the local go
# (a newer local go is fine); this gate is about repository content, so it
# hard-fails on drift.
#
#   exit 0  every golang base image pins the directive's version
#   exit 1  drift: a base image pins a different version, an untagged
#           (implicit :latest) or digest-pinned golang base, or a floating
#           minor tag that does not pin the declared toolchain
#   exit 2  structural break: no toolchain directive, a missing Dockerfile,
#           or a Dockerfile with no golang base image to compare
#
# Wired into scripts/definition-of-done.sh, scripts/release-gate.sh (so the
# Dockerfile builder stage gates itself before compiling anything), and
# `make docker`. POSIX sh + awk: it runs inside golang:alpine build stages.
set -eu

cd "$(cd "$(dirname "$0")/.." && pwd)"

directive="$(awk '$1 == "toolchain" { print $2; exit }' go.mod)"
if [ -z "$directive" ]; then
	echo "FAIL: go.mod declares no toolchain directive; it is the single source of the Go version (AGENTS.md)" >&2
	exit 2
fi
want="${directive#go}"

drift=0
for f in Dockerfile Dockerfile.test; do
	if [ ! -f "$f" ]; then
		echo "FAIL: $f not found; the parity gate covers both Dockerfiles" >&2
		exit 2
	fi
	# Base image = first non-flag token after FROM; the stage name follows AS.
	# Only golang bases carry a toolchain; runtime stages (scratch, debian, ...)
	# are out of scope. Status: 1 drift, 2 structural break.
	violations="$(awk -v want="$want" -v file="$f" '
		toupper($1) == "FROM" {
			img = ""; idx = 0
			for (i = 2; i <= NF; i++) {
				if ($i ~ /^--/) continue
				img = $i; idx = i
				break
			}
			if (img == "") next
			if (img !~ /(^|\/)golang(:|@|$)/) next
			stagename = ""
			for (i = idx + 1; i <= NF; i++) {
				if (toupper($i) == "AS" && i < NF) { stagename = $(i + 1); break }
			}
			if (stagename == "") stagename = "?"
			# An untagged or digest-pinned golang base still counts as present:
			# it is drift, not a structural absence.
			seen = 1
			if (img ~ /@/) {
				printf "%s: stage %s base \"%s\" is digest-pinned; cannot verify parity with go.mod %s — pin golang:%s-alpine\n", file, stagename, img, want, want
				bad = 1
				next
			}
			if (img !~ /:/) {
				printf "%s: stage %s base \"%s\" carries no version tag (implicit :latest); go.mod pins %s\n", file, stagename, img, want
				bad = 1
				next
			}
			seen = 1
			tag = img
			sub(/^[^:]*:/, "", tag)
			sub(/-.*/, "", tag)
			if (tag != want) {
				printf "%s: stage %s base \"%s\" pins Go %s; go.mod toolchain is %s\n", file, stagename, img, tag, want
				bad = 1
			}
		}
		END {
			if (!seen) {
				printf "%s: has no golang base image to compare against go.mod %s\n", file, want
				exit 2
			}
			if (bad) exit 1
		}
	' "$f")" && status=0 || status=$?
	if [ "$status" -eq 2 ]; then
		echo "FAIL: $violations" >&2
		exit 2
	fi
	if [ -n "$violations" ]; then
		echo "$violations" >&2
		drift=1
	fi
done

if [ "$drift" -ne 0 ]; then
	echo "FAIL: Dockerfile toolchain drift — pin every golang: base to go.mod's $directive in the same change (AGENTS.md)" >&2
	exit 1
fi
echo "Toolchain parity: every golang base image matches go.mod $directive"
