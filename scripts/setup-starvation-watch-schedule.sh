#!/bin/bash
# Install the ARMOR starvation watcher as a systemd --user timer.
#
# The watcher consumes .beads/diagnostics/pluck-diagnostics.json (rewritten
# by bead-rs on every ready query) and files one plain task bead per
# starvation episode, only when the condition reproduces across consecutive
# snapshots. Steady state is a single JSON read per run.
#
# This is a worker-machine-side step, deliberately NOT a k8s Job/CronJob
# (prohibited in this environment) and NOT a NEEDLE worker-loop change
# (NEEDLE's PluckNoCandidate telemetry already covers the counter side).

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARMOR_DIR="$(dirname "$SCRIPT_DIR")"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_NAME="armor-starvation-watch.service"
TIMER_NAME="armor-starvation-watch.timer"

install_unit() {
    local src="$1" dst="$UNIT_DIR/$2"
    # Rewrite the checked-in unit's absolute paths for this checkout.
    # The interpreter is resolved here rather than hardcoded: this box
    # (NixOS) has no /usr/bin/python3, and a wrong ExecStart path fails
    # at fire time with status=203/EXEC.
    local py
    py="$(command -v python3)"
    sed -e "s|/home/coding/ARMOR|$ARMOR_DIR|g" -e "s|__PYTHON__|$py|g" "$src" > "$dst"
    chmod 644 "$dst"
}

echo "Installing units to $UNIT_DIR ..."
mkdir -p "$UNIT_DIR"
install_unit "$SCRIPT_DIR/$SERVICE_NAME" "$SERVICE_NAME"
install_unit "$SCRIPT_DIR/$TIMER_NAME" "$TIMER_NAME"

systemctl --user daemon-reload
systemctl --user enable --now "$TIMER_NAME"

echo
echo "✓ $TIMER_NAME enabled (every 15 minutes at :07/:22/:37/:52)"
echo
echo "Verify:"
echo "  systemctl --user list-timers $TIMER_NAME"
echo "  systemctl --user start $SERVICE_NAME   # one cycle now"
echo "  journalctl --user -u $SERVICE_NAME -n 20"
echo
echo "To remove the schedule:"
echo "  systemctl --user disable --now $TIMER_NAME"
echo "  rm $UNIT_DIR/$SERVICE_NAME $UNIT_DIR/$TIMER_NAME"
echo "  systemctl --user daemon-reload"
