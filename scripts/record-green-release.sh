#!/usr/bin/env bash
# Record a fully published, compat-green release and move the repository's
# consumer image pins to it (armor-19792903).
#
# Three surfaces must always name one version — the docs/release-status.md
# latest-green marker, the README Quick Start's `export ARMOR_VERSION=` line,
# and every compose.yaml ${ARMOR_VERSION:-...} default — and
# scripts/compose-version-parity.sh fails any tree where they disagree. This
# script is the one-edit way to move them: run it AFTER a release's
# publication is verified green (docs/release-process.md, "Verifying a
# release" — images, tag, and both releases present), then write the
# release-status evidence prose for the same version in the same change.
#
# Usage:
#   scripts/record-green-release.sh <MAJOR.MINOR.PATCH>
#
# Refuses a version that is not MAJOR.MINOR.PATCH, is newer than the
# checkout's VERSION (only a cut release can be recorded green), or already
# equals the marker (nothing to do). Rewrites exactly the three pin
# surfaces, verifies the parity gate afterwards, and never commits —
# staging and the release-status prose stay with the caller. A failed run
# restores nothing by itself; git checkout -- <paths> discards a partial
# rewrite.
set -euo pipefail

usage() {
  sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
}

VERSION=""
for arg in "$@"; do
  case "$arg" in
    -h|--help) usage ;;
    -*) echo "unknown argument: $arg" >&2; usage ;;
    *)
      if [ -n "$VERSION" ]; then
        echo "error: multiple versions supplied ('$VERSION' and '$arg')" >&2
        usage
      fi
      VERSION="$arg"
      ;;
  esac
done
[ -n "$VERSION" ] || usage

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "error: '$VERSION' is not MAJOR.MINOR.PATCH" >&2; exit 2 ;;
esac
if [ ! -f VERSION ]; then
  echo "error: VERSION not found; only a cut release can be recorded green" >&2
  exit 2
fi
CURRENT="$(tr -d '[:space:]' < VERSION)"
if [ "$(printf '%s\n%s\n' "$VERSION" "$CURRENT" | sort -V | tail -1)" != "$CURRENT" ]; then
  echo "error: '$VERSION' is newer than the checkout's VERSION $CURRENT; cut and publish it first" >&2
  exit 2
fi

STATUS_DOC="docs/release-status.md"
README="README.md"
COMPOSE="compose.yaml"
for f in "$STATUS_DOC" "$README" "$COMPOSE"; do
  if [ ! -f "$f" ]; then
    echo "error: $f not found; it carries one of the pins this script moves" >&2
    exit 2
  fi
done

RECORDED="$(awk '
	match($0, /release: latest-green version: [0-9][0-9A-Za-z._-]*/) { print substr($0, RSTART + 31, RLENGTH - 31) }
' "$STATUS_DOC")"
if [ "$(printf '%s\n' "$RECORDED" | grep -c .)" != 1 ]; then
  echo "error: $STATUS_DOC must carry exactly one 'release: latest-green version: <v>' marker" >&2
  exit 2
fi
if [ "$RECORDED" = "$VERSION" ]; then
  echo "latest-green is already $VERSION; nothing to record" >&2
  exit 2
fi
if ! grep -q '^export ARMOR_VERSION=' "$README"; then
  echo "error: $README carries no 'export ARMOR_VERSION=' line to rewrite" >&2
  exit 2
fi
if ! grep -q 'ARMOR_VERSION:-' "$COMPOSE"; then
  echo "error: $COMPOSE carries no \${ARMOR_VERSION:-...} default to rewrite" >&2
  exit 2
fi
if [ "$(grep -c '^export ARMOR_VERSION=' "$README")" != 1 ]; then
  echo "error: $README must carry exactly one 'export ARMOR_VERSION=' line" >&2
  exit 2
fi

# Rewrite each surface through a temp file, verifying the rewrite count, so
# a malformed file stops the run before anything is moved into place.
rewrite() {
  # rewrite <file> <awk program> <expected count> <needle>
  file="$1"; program="$2"; expect="$3"; needle="$4"
  awk -v v="$VERSION" "$program" "$file" > "$file.tmp"
  got="$(grep -o "$needle" "$file.tmp" | wc -l | tr -d ' ')"
  if [ "$got" -ne "$expect" ]; then
    rm -f "$file.tmp"
    echo "error: rewrote $got of $expect '$needle' occurrences in $file; fix the file and retry" >&2
    exit 2
  fi
  mv "$file.tmp" "$file"
}

markers_before="$(grep -c 'release: latest-green version:' "$STATUS_DOC" || true)"
rewrite "$STATUS_DOC" '
	{ gsub(/release: latest-green version: [0-9][0-9A-Za-z._-]+/, "release: latest-green version: " v); print }
' "$markers_before" "release: latest-green version: $VERSION"
rewrite "$README" '
	{ if (!done && /^export ARMOR_VERSION=/) { sub(/=.*/, "=" v); done = 1 } print }
' 1 "export ARMOR_VERSION=$VERSION"
compose_defaults="$(grep -o 'ARMOR_VERSION:-' "$COMPOSE" | wc -l | tr -d ' ')"
rewrite "$COMPOSE" '
	{ gsub(/ARMOR_VERSION:-[A-Za-z0-9._-]+/, "ARMOR_VERSION:-" v); print }
' "$compose_defaults" "ARMOR_VERSION:-$VERSION"

./scripts/compose-version-parity.sh

echo "recorded latest-green $VERSION (was $RECORDED)"
echo "next: write the $VERSION evidence prose in $STATUS_DOC and commit the three pin surfaces together"
