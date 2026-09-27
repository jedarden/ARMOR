#!/bin/sh
# Prohibited deployment constructs gate: fails on repository content the org's
# hard rules forbid (home-directory CLAUDE.md, "Hard prohibitions", and
# AGENTS.md, "Hard rules"). GitHub Actions are disabled org-wide, ArgoCD
# cannot prune Job/CronJob pods, and a floating or missing image tag breaks
# rollback — so the gate fails the tree on:
#
#   [workflow-file]         any file under .github/workflows/
#   [kind-job-cronjob]      a real `kind: Job` / `kind: CronJob` manifest line
#   [image-latest]          an `image:` assignment or `FROM` base tagged
#                           :latest, any registry
#   [ronaldraygun-latest]   a ronaldraygun/* reference tagged :latest —
#                           anywhere, notes and prose included
#   [ronaldraygun-unpinned] an `image:` ronaldraygun/* reference carrying
#                           neither a tag nor a digest
#
#   exit 0  no prohibited construct found
#   exit 1  at least one prohibited construct found (each hit on stderr)
#
# Comment lines (leading # or //) are exempt: docs must be able to DESCRIBE
# the prohibitions, and the org guard draws the same line. A prohibited
# construct stated as a real value — a runbook's image line, a fenced yaml
# example — is not exempt: the rules hold "including examples and notes".
# Unpinned-scoping note: prose that merely NAMES an image (release docs
# listing what CI publishes) is not an image reference, so the unpinned rule
# only fires on `image:` assignment lines.
#
# tests/fixtures/prohibited-constructs/ is exempt by name: it holds the
# deliberately-prohibited regression fixtures that
# tests/test_prohibited_constructs.py replays into scratch trees to prove
# every rule fires. The same content anywhere else in the tree fails.
#
# Wired into scripts/definition-of-done.sh and scripts/release-gate.sh.
# POSIX sh + find + grep -E, busybox-compatible: the release gate runs it
# inside golang:alpine build stages.
set -eu

cd "$(cd "$(dirname "$0")/.." && pwd)"

# Skipped everywhere: VCS internals, the bead store (checkpoint JSONL embeds
# historical manifest text), build output, bytecode caches, and the gate's
# own fixtures. Takes the tail of a find expression as arguments.
files() {
	find . \
		-path ./.git -prune -o \
		-path ./.beads -prune -o \
		-path ./bin -prune -o \
		-path ./node_modules -prune -o \
		-path '*/__pycache__' -prune -o \
		-path ./tests/fixtures/prohibited-constructs -prune -o \
		"$@"
}

violations=0

# scan_rule LABEL PATTERN — report each non-comment hit of an ERE. Comment
# lines are dropped after matching (the reporter sees "path:line:content", so
# a content line beginning # or // is a comment).
scan_rule() {
	label="$1"
	pattern="$2"
	hits="$(files -type f -print0 | xargs -0 -r grep -nE -- "$pattern" || true)"
	if [ -n "$hits" ]; then
		hits="$(printf '%s\n' "$hits" | grep -vE '^[^:]+:[0-9]+:[[:space:]]*(#|//)' || true)"
	fi
	if [ -n "$hits" ]; then
		printf 'prohibited construct [%s]:\n%s\n' "$label" "$hits" >&2
		violations=1
	fi
}

# [workflow-file] — the org rule is against the path existing at all, not
# against its content.
wf="$(files -type f -path '*/.github/workflows/*' -print || true)"
if [ -n "$wf" ]; then
	printf 'prohibited construct [workflow-file]: GitHub Actions workflows are disabled org-wide; CI runs on Argo Workflows in iad-ci:\n%s\n' "$wf" >&2
	violations=1
fi

# [kind-job-cronjob] — line-anchored on the yaml key, so a comment or a
# mid-sentence mention ("no kind: Job anywhere") never matches.
scan_rule kind-job-cronjob '^[[:space:]]*kind:[[:space:]]*(Job|CronJob)[[:space:]]*($|#)'

# [image-latest] — a tag that is exactly `latest` at a token boundary,
# on an `image:` assignment or a FROM base line.
scan_rule image-latest '^[[:space:]]*(image:[[:space:]]*|FROM[[:space:]])[^[:space:]]*:latest([[:space:]]|["'\''`]|$)'

# [ronaldraygun-latest] — the same tag on the org's own images, from any
# line shape: manifests, runbooks, notes.
scan_rule ronaldraygun-latest 'ronaldraygun/[A-Za-z0-9._/-]+:latest([[:space:]]|["'\''`]|[,);]|$)'

# [ronaldraygun-unpinned] — an `image:` ronaldraygun/* ref must carry a tag
# or a digest. `name:tag` and `name@digest` are exempted explicitly rather
# than looked ahead to (grep -E has no lookahead): candidates first, then
# drop every line whose ref is followed by : or @. Variable, placeholder and
# templated tags (:$IMAGE_TAG, :${VAR:-default}, :<version>, :{tag}) all
# count as explicit tags.
candidates="$(files -type f -print0 | xargs -0 -r grep -nE '^[[:space:]]*image:[[:space:]]*["'\'']?[^[:space:]]*ronaldraygun/[A-Za-z0-9._/-]+' || true)"
if [ -n "$candidates" ]; then
	candidates="$(printf '%s\n' "$candidates" | grep -vE '^[^:]+:[0-9]+:[[:space:]]*(#|//)' || true)"
fi
if [ -n "$candidates" ]; then
	unpinned="$(printf '%s\n' "$candidates" | grep -vE 'ronaldraygun/[A-Za-z0-9._/-]+[:@]' || true)"
else
	unpinned=""
fi
if [ -n "$unpinned" ]; then
	printf 'prohibited construct [ronaldraygun-unpinned]:\n%s\n' "$unpinned" >&2
	violations=1
fi

if [ "$violations" -ne 0 ]; then
	echo "FAIL: prohibited deployment constructs found — fix the listed lines (org hard rules; see scripts/prohibited-constructs-gate.sh header)" >&2
	exit 1
fi
echo "Prohibited constructs: none found"
