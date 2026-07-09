#!/run/current-system/sw/bin/bash
# Comprehensive output redirection test for ARMOR Pluck execution
# Tests all redirection patterns with simulated NEEDLE output

WORKSPACE="/home/coding/ARMOR"
LOG_DIR="$WORKSPACE/logs/pluck-debug"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

echo "=== ARMOR Output Redirection Comprehensive Test ==="
echo "Timestamp: $TIMESTAMP"
echo "Log directory: $LOG_DIR"
echo ""

# Test with simulated Pluck execution
echo "Testing simulated Pluck execution..."
PLUCK_LOG="$LOG_DIR/pluck-test-${TIMESTAMP}.log"

timeout 5s bash -c '
    echo "[INFO] Worker booted"
    echo "[DEBUG] Loading configuration from pluck-config.yaml"
    echo "[TRACE] Pluck strand filtering starting"
    echo "Processing bead candidates..."
    for i in {1..3}; do
        echo "[INFO] Evaluating candidate $i"
        echo "[TRACE] Filter applied: label=deferred, result=pass"
        sleep 0.1
    done
    echo "[INFO] Found 3 valid candidates"
    echo "[WARN] High candidate count, filtering may take time" >&2
    echo "[INFO] Pluck execution completed"
' &> "$PLUCK_LOG" || true

if [[ -f "$PLUCK_LOG" && -s "$PLUCK_LOG" ]]; then
    echo "✅ Pluck simulation successful"
    echo "  Log file: $PLUCK_LOG"
    echo "  Size: $(stat -c%s "$PLUCK_LOG" 2>/dev/null || stat -f%z "$PLUCK_LOG" 2>/dev/null) bytes"
    echo "  Lines: $(wc -l < "$PLUCK_LOG")"
    echo ""
    echo "Log content sample:"
    head -5 "$PLUCK_LOG" | sed 's/^/  /'
else
    echo "❌ Pluck simulation failed"
    exit 1
fi

echo ""
echo "=== Test Complete ==="
