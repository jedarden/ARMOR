#!/bin/bash
# Pluck Log Capture Helper
# Captures Pluck stdout/stderr to timestamped log file

set -e

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="/home/coding/ARMOR"
LOG_FILE="${LOG_DIR}/pluck-debug-${TIMESTAMP}.log"

# Optional: custom session ID (e.g., bead ID)
SESSION_ID="${1:-pluck}"

echo "Capturing Pluck output to: ${LOG_FILE}"
echo "Session ID: ${SESSION_ID}"
echo "---"

# Capture with tee for real-time viewing
pluck 2>&1 | tee "${LOG_FILE}"

echo "---"
echo "Log saved to: ${LOG_FILE}"
echo "View with: less ${LOG_FILE}"
