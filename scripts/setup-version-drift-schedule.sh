#!/bin/bash
# Setup ARMOR Version Drift Check Scheduling
# This script sets up a cron job to check ARMOR version drift daily

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARMOR_DIR="$(dirname "$SCRIPT_DIR")"
CRON_SCRIPT="$SCRIPT_DIR/check-armor-version-drift.py"
LOG_DIR="$ARMOR_DIR/logs"

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Check if already scheduled
if crontab -l 2>/dev/null | grep -q "check-armor-version-drift.py"; then
    echo "ARMOR version drift check is already scheduled."
    echo "Current crontab entry:"
    crontab -l | grep "check-armor-version-drift.py"
    echo ""
    echo "To remove it, run:"
    echo "  crontab -e"
    echo "And delete the line containing 'check-armor-version-drift.py'"
    exit 0
fi

# Add cron job (runs daily at 9:17 AM)
echo "Adding ARMOR version drift check to crontab..."
# Using 9:17 AM instead of 9:00 AM to avoid load spike at :00
(crontab -l 2>/dev/null; echo "17 9 * * * $CRON_SCRIPT >> $LOG_DIR/version-drift-check.log 2>&1") | crontab -

echo "✓ Scheduled ARMOR version drift check to run daily at 9:17 AM"
echo "Logs will be written to: $LOG_DIR/version-drift-check.log"
echo ""
echo "To view logs:"
echo "  tail -f $LOG_DIR/version-drift-check.log"
echo ""
echo "To manually run the check:"
echo "  $CRON_SCRIPT"
echo ""
echo "To remove the schedule:"
echo "  crontab -e"
echo "  (delete the line with 'check-armor-version-drift.py')"
