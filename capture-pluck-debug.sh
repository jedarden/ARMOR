#!/bin/bash
# Script to capture complete Pluck filtering debug output
# This script runs NEEDLE with maximum debug logging to capture all Pluck strand decisions

set -e

WORKSPACE="${1:-/home/coding/ARMOR}"
OUTPUT_FILE="${2:-pluck-debug-capture-$(date +%Y%m%d-%H%M%S).log}"
COUNT="${3:-1}"

echo "=== Pluck Filtering Debug Capture ==="
echo "Workspace: $WORKSPACE"
echo "Output file: $OUTPUT_FILE"
echo "Count: $COUNT"
echo ""

# Set comprehensive debug logging for Pluck and related components
export RUST_LOG="needle::strand::pluck=trace,needle::strand=debug,needle::bead_store=debug,needle::worker=debug,needle::dispatch=debug"

echo "Starting NEEDLE with trace logging..."
echo "RUST_LOG=$RUST_LOG"
echo ""

# Run NEEDLE and capture all output
RUST_LOG="$RUST_LOG" needle run -w "$WORKSPACE" -c "$COUNT" 2>&1 | tee "$OUTPUT_FILE"

echo ""
echo "Capture complete. Output saved to: $OUTPUT_FILE"
echo ""
echo "To analyze Pluck filtering decisions:"
echo "  grep -i 'pluck' $OUTPUT_FILE"
echo "  grep -i 'filter' $OUTPUT_FILE"
echo "  grep -i 'exclude' $OUTPUT_FILE"
echo "  grep -i 'candidate' $OUTPUT_FILE"
