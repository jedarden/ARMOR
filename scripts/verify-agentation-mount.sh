#!/usr/bin/env bash
# Browser smoke test for the workspace Agentation rule: every ARMOR web UI
# entry point must load the toolbar with its import map and be verified by
# MOUNTING (#agentation-root in the rendered DOM), never by grepping the
# script tag — the tag without the map renders a perfect page and mounts
# nothing.
#
# Drives a real headless Chromium (or any browser via AGENTATION_BROWSER)
# against every entry point served from the real route tables:
#   - the proxy dashboard  (internal/dashboard, /dashboard)
#   - the demo dashboard   (armor demo, admin /dashboard)
#   - the fleet console    (cmd/armor-fleet, /)
#
# The underlying tests skip when no browser is found or esm.sh is
# unreachable. A skip means the mount was NOT verified, so this script
# treats an all-skip run as a failure — it exists to get a real answer, and
# a green answer that proved nothing would re-create the tag-without-map
# blind spot.
#
# Usage: scripts/verify-agentation-mount.sh
#   AGENTATION_BROWSER=/path/to/chromium  override the browser search
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== Agentation browser mount smoke: proxy dashboard + demo dashboard + fleet console =="
output="$(go test ./internal/dashboard ./cmd/armor ./cmd/armor-fleet \
    -run 'TestDashboardAgentationMountsInBrowser|TestDemoAgentationMountsInBrowser|TestFleetAgentationMountsInBrowser' \
    -count=1 -v "$@")" || {
  printf '%s\n' "$output"
  echo "FAIL: go test exited non-zero" >&2
  exit 1
}
printf '%s\n' "$output"

if grep -q -- '--- SKIP:' <<<"$output"; then
  echo "FAIL: the mount checks skipped, so nothing was verified." >&2
  echo "  Install a Chromium-family browser or point AGENTATION_BROWSER at one," >&2
  echo "  and ensure https://esm.sh is reachable (the pages import React from it)." >&2
  exit 1
fi

echo "OK: Agentation mounted on every ARMOR web UI entry point."
