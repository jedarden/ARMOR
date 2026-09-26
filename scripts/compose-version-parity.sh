#!/bin/sh
# compose.yaml ↔ VERSION parity gate: the tracked demo/production compose
# file pins its image through ${ARMOR_VERSION:-<version>} defaults, and the
# README's Quick Start points readers at the repository's VERSION-file tag —
# so every default must equal the VERSION file exactly.
# scripts/cut-release.sh rewrites the defaults in the release commit; this
# gate is what makes forgetting that impossible anywhere else. Modeled on
# scripts/toolchain-parity.sh.
#
#   exit 0  compose.yaml carries at least one ARMOR_VERSION default and
#           every one of them equals the VERSION file
#   exit 1  drift: a default pins anything other than the VERSION value
#   exit 2  structural break: a missing or non-semver VERSION file, a
#           missing compose.yaml, or no ARMOR_VERSION default at all
#
# Wired into scripts/definition-of-done.sh, scripts/release-gate.sh (so an
# image is never built from a tree whose compose pin lags VERSION), and
# `make docker`. POSIX sh + awk: it runs inside golang:alpine build stages.
set -eu

cd "$(cd "$(dirname "$0")/.." && pwd)"

if [ ! -f VERSION ]; then
	echo "FAIL: VERSION not found; compose.yaml's ARMOR_VERSION defaults are pinned to it" >&2
	exit 2
fi
want="$(tr -d '[:space:]' < VERSION)"
case "$want" in
	[0-9]*.[0-9]*.[0-9]*) ;;
	*)
		echo "FAIL: VERSION '$want' is not MAJOR.MINOR.PATCH; refusing to reason about compose.yaml's pin" >&2
		exit 2
		;;
esac
if [ ! -f compose.yaml ]; then
	echo "FAIL: compose.yaml not found; its ARMOR_VERSION defaults must pin VERSION $want" >&2
	exit 2
fi

# Default value = the text between "ARMOR_VERSION:-" (15 characters) and the
# closing brace of the ${...} interpolation. Everything after the last
# matched default on a line is re-scanned, so multiple defaults per line are
# all checked. Status: 1 drift, 2 structural break.
violations="$(awk -v want="$want" '
	{
		line = $0
		while (match(line, /ARMOR_VERSION:-[^}]*/)) {
			got = substr(line, RSTART + 15, RLENGTH - 15)
			seen = 1
			if (got != want) {
				printf "compose.yaml:%d: ARMOR_VERSION default \"%s\" is not the VERSION-file pin \"%s\"\n", NR, got, want
				bad = 1
			}
			line = substr(line, RSTART + RLENGTH)
		}
	}
	END {
		if (!seen) {
			print "compose.yaml: carries no ${ARMOR_VERSION:-...} default to pin against VERSION"
			exit 2
		}
		if (bad) exit 1
	}
' compose.yaml)" && status=0 || status=$?
if [ "$status" -eq 2 ]; then
	echo "FAIL: $violations" >&2
	exit 2
fi
if [ -n "$violations" ]; then
	echo "$violations" >&2
	echo "FAIL: compose.yaml ARMOR_VERSION drift — cut releases with scripts/cut-release.sh, which bumps the defaults in the release commit, or pin them to VERSION $want in the same change (AGENTS.md)" >&2
	exit 1
fi
echo "Compose version parity: every ARMOR_VERSION default in compose.yaml matches VERSION $want"
