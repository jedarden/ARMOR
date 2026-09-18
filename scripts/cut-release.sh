#!/usr/bin/env bash
# Cut an ARMOR release commit: bump VERSION and prepend a CHANGELOG.md entry
# generated from the commits since the previous release tag, in ONE commit.
#
# Usage:
#   scripts/cut-release.sh <MAJOR.MINOR.PATCH> [--no-push] [--dry-run]
#
# Everything after the push is CI's job (declarative-config, armor-build): a
# push that changes VERSION builds and publishes the images, tags v<version>
# at this commit, creates the Forgejo release with the CHANGELOG section
# written here as its body and, once the push mirror has copied the tag,
# the GitHub release. CI never bumps VERSION; this script is the only thing
# that does. Never create the tag by hand.
set -euo pipefail

usage() {
  sed -n '2,13p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
}

VERSION="${1:-}"
[ $# -gt 0 ] && shift
PUSH=1
DRY=0
for arg in "$@"; do
  case "$arg" in
    --no-push) PUSH=0 ;;
    --dry-run) DRY=1; PUSH=0 ;;
    -h|--help) usage ;;
    *) echo "unknown argument: $arg" >&2; usage ;;
  esac
done
[ -n "$VERSION" ] || usage

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "error: '$VERSION' is not MAJOR.MINOR.PATCH" >&2; exit 2 ;;
esac
CURRENT="$(tr -d '[:space:]' < VERSION)"
if [ "$CURRENT" = "$VERSION" ] ||
   [ "$(printf '%s\n%s\n' "$CURRENT" "$VERSION" | sort -V | tail -1)" != "$VERSION" ]; then
  echo "error: $VERSION is not newer than the current VERSION $CURRENT" >&2
  exit 2
fi

# The release commit must carry only VERSION and CHANGELOG.md. Other agents'
# unstaged edits in a shared checkout are fine; a dirty INDEX is not, because
# `git commit -- <paths>` would still record what is already staged.
if ! git diff --cached --quiet; then
  echo "error: the index has staged changes; commit or unstage them first" >&2
  exit 2
fi
BRANCH="$(git branch --show-current)"
if [ "$BRANCH" != "main" ]; then
  echo "error: releases are cut from main (currently on '$BRANCH')" >&2
  exit 2
fi

PREV_TAG="$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)"
if [ -n "$PREV_TAG" ]; then
  RANGE="$PREV_TAG..HEAD"
else
  RANGE="HEAD"
fi

# Release notes: every non-merge commit subject since the previous tag, minus
# bead/checkpoint bookkeeping and earlier release commits.
NOTES="$(git log --no-merges --format='- %s' "$RANGE" \
  | grep -vE '^- (chore\(beads\)|bead\(|release:|chore\(armor\): release|chore\(release\))' || true)"
if [ -z "$NOTES" ]; then
  NOTES="- Maintenance release (no user-visible changes recorded since ${PREV_TAG:-the first commit})"
fi

DATE="$(date -u +%Y-%m-%d)"
ENTRY="## $VERSION ($DATE)

$NOTES
"
export ENTRY

if [ "$DRY" = 1 ]; then
  echo "== dry run: would write VERSION=$VERSION and prepend to CHANGELOG.md:"
  echo
  printf '%s\n' "$ENTRY"
  exit 0
fi

if [ ! -f CHANGELOG.md ]; then
  printf '# Changelog\n\n' > CHANGELOG.md
fi
# Insert the entry immediately before the first existing "## " heading so the
# file stays newest-first; append it when the file has no entries yet.
awk '
  BEGIN { entry = ENVIRON["ENTRY"]; done = 0 }
  /^## / && !done { print entry; done = 1 }
  { print }
  END { if (!done) print entry }
' CHANGELOG.md > CHANGELOG.md.tmp
mv CHANGELOG.md.tmp CHANGELOG.md

printf '%s\n' "$VERSION" > VERSION
git add VERSION CHANGELOG.md
git commit -q -m "release: armor $VERSION" -- VERSION CHANGELOG.md
echo "committed 'release: armor $VERSION' as $(git rev-parse --short HEAD)"

if [ "$PUSH" = 1 ]; then
  git push origin HEAD:main
  echo "pushed. armor-build will build and publish the images, then tag v$VERSION and create the releases."
else
  echo "not pushed (--no-push). Push with: git push origin main"
fi
