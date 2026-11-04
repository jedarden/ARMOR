#!/bin/sh
# Image-pin parity gate: the README Quick Start export, every compose.yaml
# ${ARMOR_VERSION:-<version>} default, and the docs/release-status.md
# latest-green record must name the same fully published, compat-green
# release (armor-19792903) — the tag a consumer should actually run. A cut
# whose publication is incomplete (images but no releases, like 0.1.2006)
# leaves every pin where it was; the checkout's VERSION file is only the
# development counter and may run ahead, so the register may never name a
# green release newer than it. scripts/record-green-release.sh moves all
# three surfaces in one edit; this gate is what makes forgetting one
# impossible anywhere else.
#
#   exit 0  every pin names the release-status latest-green version
#   exit 1  drift: a compose default or the README export pins something
#           else, or the register names a green release newer than VERSION
#   exit 2  structural break: a missing or malformed latest-green marker or
#           VERSION file, a missing compose.yaml or README.md, no
#           ${ARMOR_VERSION:-...} default, or no (or more than one)
#           `export ARMOR_VERSION=` line
#
# Wired into scripts/definition-of-done.sh, scripts/release-gate.sh (so an
# image is never built from a tree whose consumer pins disagree), and
# `make docker`. POSIX sh + awk: it runs inside golang:alpine build stages.
set -eu

cd "$(cd "$(dirname "$0")/.." && pwd)"

STATUS_DOC="docs/release-status.md"

fail2() {
	echo "FAIL: $*" >&2
	exit 2
}

# --- the register's record: the one version every pin must name -----------
if [ ! -f "$STATUS_DOC" ]; then
	fail2 "$STATUS_DOC not found; its latest-green marker is what README and compose.yaml must pin"
fi
# "release: latest-green version: " is 31 characters; the marker value runs
# to the next whitespace. Exactly one marker may carry the pin.
green="$(awk '
	{
		line = $0
		while (match(line, /release: latest-green version: [0-9][0-9A-Za-z._-]*/)) {
			count++
			value = substr(line, RSTART + 31, RLENGTH - 31)
			line = substr(line, RSTART + RLENGTH)
		}
	}
	END { if (count == 1) print value; else exit 2 }
' "$STATUS_DOC")" || fail2 "$STATUS_DOC must carry exactly one 'release: latest-green version: <v>' marker"
case "$green" in
	[0-9]*.[0-9]*.[0-9]*) ;;
	*)
		fail2 "latest-green version '$green' in $STATUS_DOC is not MAJOR.MINOR.PATCH"
		;;
esac

# --- VERSION bounds the register: a green release must have been cut ------
if [ ! -f VERSION ]; then
	fail2 "VERSION not found; the register's latest-green release must not be newer than it"
fi
current="$(tr -d '[:space:]' < VERSION)"
case "$current" in
	[0-9]*.[0-9]*.[0-9]*) ;;
	*)
		fail2 "VERSION '$current' is not MAJOR.MINOR.PATCH; refusing to reason about the latest-green record"
		;;
esac
if [ "$green" != "$current" ] &&
   [ "$(printf '%s\n%s\n' "$green" "$current" | sort -V | tail -1)" = "$green" ]; then
	echo "FAIL: $STATUS_DOC records latest-green $green, newer than the checkout's VERSION $current — record a release that was actually cut" >&2
	exit 1
fi

# --- compose.yaml: every default names the register's release -------------
if [ ! -f compose.yaml ]; then
	fail2 "compose.yaml not found; its ARMOR_VERSION defaults must pin the latest-green release $green"
fi
# Default value = the text between "ARMOR_VERSION:-" (15 characters) and the
# closing brace of the ${...} interpolation. Everything after the last
# matched default on a line is re-scanned, so multiple defaults per line are
# all checked. Status: 1 drift, 2 structural break.
violations="$(awk -v want="$green" '
	{
		line = $0
		while (match(line, /ARMOR_VERSION:-[^}]*/)) {
			got = substr(line, RSTART + 15, RLENGTH - 15)
			seen = 1
			if (got != want) {
				printf "compose.yaml:%d: ARMOR_VERSION default \"%s\" is not the release-status latest-green pin \"%s\"\n", NR, got, want
				bad = 1
			}
			line = substr(line, RSTART + RLENGTH)
		}
	}
	END {
		if (!seen) {
			print "compose.yaml: carries no ${ARMOR_VERSION:-...} default to pin against the latest-green release"
			exit 2
		}
		if (bad) exit 1
	}
' compose.yaml)" && status=0 || status=$?
if [ "$status" -eq 2 ]; then
	fail2 "$violations"
fi
if [ -n "$violations" ]; then
	echo "$violations" >&2
	echo "FAIL: compose.yaml ARMOR_VERSION drift — record green releases with scripts/record-green-release.sh, which moves the defaults with the README export and the release-status marker, or pin them to $green in the same change (AGENTS.md)" >&2
	exit 1
fi

# --- README: exactly one export line names the register's release ---------
if [ ! -f README.md ]; then
	fail2 "README.md not found; its ARMOR_VERSION export must pin the latest-green release $green"
fi
readme="$(awk -v want="$green" '
	/^export ARMOR_VERSION=/ {
		count++
		got = substr($0, 22)
		if (got != want) {
			printf "README.md:%d: export pins \"%s\", not the release-status latest-green pin \"%s\"\n", NR, got, want
			bad = 1
		}
	}
	END {
		if (count == 0) {
			print "README.md: carries no export ARMOR_VERSION= line to pin the latest-green release"
			exit 2
		}
		if (count > 1) {
			print "README.md: " count " export ARMOR_VERSION= lines; exactly one may carry the latest-green pin"
			exit 2
		}
		if (bad) exit 1
	}
' README.md)" && rstatus=0 || rstatus=$?
if [ "$rstatus" -eq 2 ]; then
	fail2 "$readme"
fi
if [ -n "$readme" ]; then
	echo "$readme" >&2
	echo "FAIL: README ARMOR_VERSION drift — record green releases with scripts/record-green-release.sh, which moves the export with the compose defaults and the release-status marker, or pin it to $green in the same change (AGENTS.md)" >&2
	exit 1
fi

echo "Release-pin parity: the README export and every compose.yaml ARMOR_VERSION default name the release-status latest-green release $green"
