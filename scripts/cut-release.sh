#!/usr/bin/env bash
# Cut an ARMOR release commit: bump VERSION and prepend a CHANGELOG.md entry
# generated from the commits since the previous release tag, in ONE commit.
#
# Before writing anything the script lints the tree with golangci-lint —
# the same command and the same major.minor armor-build's lint step pins —
# and refuses to cut when the binary is missing, its version drifts from
# the pin, or the run is red: a cut that fails CI's lint step burns the
# version number, because resolve-version pins the VERSION commit
# (0.1.2003, 0.1.2005). --dry-run lints too; there is no skip flag.
#
# Usage:
#   scripts/cut-release.sh [--no-push] [--dry-run] <MAJOR.MINOR.PATCH>
#
# Everything after the push is CI's job (declarative-config, armor-build): a
# push that changes VERSION builds and publishes the images, tags v<version>
# at this commit, creates the Forgejo release with the CHANGELOG section
# written here as its body and, once the push mirror has copied the tag,
# the GitHub release. CI never bumps VERSION; this script is the only thing
# that does. The consumer image pins (README export, compose.yaml defaults,
# release-status marker) deliberately do NOT ride in this commit: a fresh
# cut is not a green release yet. Once publication is verified, scripts/
# record-green-release.sh moves them in the same change as the release-status
# evidence, and scripts/compose-version-parity.sh gates every other change.
# Never create the tag by hand.
set -euo pipefail

usage() {
  sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'
  exit 2
}

VERSION=""
PUSH=1
DRY=0
for arg in "$@"; do
  case "$arg" in
    --no-push) PUSH=0 ;;
    --dry-run) DRY=1; PUSH=0 ;;
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
CURRENT="$(tr -d '[:space:]' < VERSION)"
if [ "$CURRENT" = "$VERSION" ] ||
   [ "$(printf '%s\n%s\n' "$CURRENT" "$VERSION" | sort -V | tail -1)" != "$VERSION" ]; then
  echo "error: $VERSION is not newer than the current VERSION $CURRENT" >&2
  exit 2
fi

# The release commit must carry only VERSION, CHANGELOG.md and compose.yaml.
# Other agents' unstaged edits in a shared checkout are fine; a dirty INDEX is
# not, because `git commit -- <paths>` would still record what is already
# staged.
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

# The publisher creates tags through Forgejo after a successful image build.
# A checkout that has not fetched that tag would repeat old commits in the
# next CHANGELOG entry. Read the authoritative origin tags without changing
# this checkout; fail with a useful instruction if local tags are stale.
if git remote get-url origin >/dev/null 2>&1; then
  if ! REMOTE_TAGS="$(git ls-remote --tags --refs origin)"; then
    echo "error: cannot read release tags from origin; retry before cutting" >&2
    exit 2
  fi
  REMOTE_LATEST="$(printf '%s\n' "$REMOTE_TAGS" | sed -n 's#.*refs/tags/\(v[0-9][^ ]*\)$#\1#p' | sort -V | tail -1)"
  if [ "$REMOTE_LATEST" != "$PREV_TAG" ]; then
    echo "error: local release tags lag origin ($PREV_TAG vs $REMOTE_LATEST); run git fetch --tags origin" >&2
    exit 2
  fi
fi
if [ -n "$PREV_TAG" ]; then
  RANGE="$PREV_TAG..HEAD"
else
  RANGE="HEAD"
fi

# Lint gate, before anything is written. CI's first step after
# resolve-version is golangci-lint (armor-workflowtemplate.yml,
# golangci/golangci-lint v2.13.2-alpine), and a red cut burns the version
# number — resolve-version pins the VERSION commit, so a failed build can
# only be repaired by another cut (0.1.2003, 0.1.2005). Same command, same
# major.minor, so a green lint here means a green lint step there. Bump the
# pin here and in declarative-config together.
GOLANGCI_LINT="${GOLANGCI_LINT:-golangci-lint}"
LINT_PIN="2.13"
if ! command -v "$GOLANGCI_LINT" >/dev/null 2>&1; then
  echo "error: golangci-lint not found (looked for '$GOLANGCI_LINT'); refusing to cut —" >&2
  echo "error: install v${LINT_PIN}.x, the major.minor armor-build's lint step pins" >&2
  exit 1
fi
LINT_VERSION="$("$GOLANGCI_LINT" version 2>&1 || true)"
echo "== lint: $LINT_VERSION"
case "$LINT_VERSION" in
  *" version ${LINT_PIN}."*) ;;
  *)
    echo "error: golangci-lint v${LINT_PIN}.x required (the major.minor armor-build's lint step pins); found: ${LINT_VERSION:-<no version output>}" >&2
    exit 1
    ;;
esac
if ! "$GOLANGCI_LINT" run --timeout 15m ./...; then
  echo "error: golangci-lint failed — fix the findings above before cutting;" >&2
  echo "error: a cut that ships them burns the version number" >&2
  exit 1
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
