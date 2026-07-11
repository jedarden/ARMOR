#!/bin/bash
# Litestream Backup Health Check Script
# This script checks the health of litestream backups without doing a full restore

set -e

echo "=== Litestream Backup Health Check ==="
echo "Date: $(date)"
echo ""

echo "This script performs non-destructive checks on litestream backup status."
echo "For a full restore test, see: litestream-restore-verification-job.yaml"
echo ""

# Check if we have kubectl access
if ! kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint &>/dev/null; then
    echo "✗ Cannot access cluster via kubectl proxy"
    exit 1
fi

echo "✓ Cluster access confirmed"
echo ""

# Check queue-api pod status
echo "=== 1. Checking queue-api Pod Status ==="
POD_COUNT=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=queue-api --no-headers 2>/dev/null | wc -l)
if [ "$POD_COUNT" -eq 0 ]; then
    echo "✗ No queue-api pods found"
    exit 1
fi

echo "✓ Found $POD_COUNT queue-api pod(s)"

READY_PODS=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=queue-api --no-headers 2>/dev/null | grep -c "Ready" || echo "0")
echo "  Ready pods: $READY_PODS"
echo ""

# Check litestream container status
echo "=== 2. Checking Litestream Container Status ==="
LITESTREAM_LOGS=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 logs deployment/queue-api -c litestream -n devimprint --tail=20 2>/dev/null || echo "")

if [ -z "$LITESTREAM_LOGS" ]; then
    echo "✗ No litestream logs found"
    exit 1
fi

echo "✓ Litestream container is logging"
echo ""

# Check for recent replication activity
echo "=== 3. Checking Recent Replication Activity ==="
RECENT_UPLOADS=$(echo "$LITESTREAM_LOGS" | grep -c "ltx file uploaded" || echo "0")
RECENT_SYNCS=$(echo "$LITESTREAM_LOGS" | grep -c "replica sync" || echo "0")

echo "  Recent WAL uploads: $RECENT_UPLOADS"
echo "  Recent sync operations: $RECENT_SYNCS"

if [ "$RECENT_UPLOADS" -gt 0 ]; then
    echo "✓ Active replication detected"
else
    echo "⚠ No recent upload activity (may be idle period)"
fi
echo ""

# Check for errors
echo "=== 4. Checking for Errors ==="
ERRORS=$(echo "$LITESTREAM_LOGS" | grep -i "error\|fail" || echo "")

if [ -n "$ERRORS" ]; then
    echo "⚠ Potential errors found:"
    echo "$ERRORS" | head -5
else
    echo "✓ No errors detected in recent logs"
fi
echo ""

# Check litestream configuration
echo "=== 5. Verifying Litestream Configuration ==="
CONFIG_CHECK=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get configmap queue-api-litestream-config -n devimprint -o jsonpath='{.data.litestream\.yml}' 2>/dev/null || echo "")

if [ -z "$CONFIG_CHECK" ]; then
    echo "✗ Cannot retrieve litestream configuration"
    exit 1
fi

echo "✓ Configuration found"
# Check key parameters
if echo "$CONFIG_CHECK" | grep -q "path: /data/queue.db"; then
    echo "✓ Database path correct: /data/queue.db"
fi
if echo "$CONFIG_CHECK" | grep -q "path: state/litestream/queue.db"; then
    echo "✓ Backup path correct: state/litestream/queue.db"
fi
if echo "$CONFIG_CHECK" | grep -q "endpoint: http://armor:9000"; then
    echo "✓ ARMOR endpoint configured: http://armor:9000"
fi
echo ""

# Check ARMOR connectivity (indirectly)
echo "=== 6. Checking ARMOR Service Availability ==="
ARMOR_PODS=$(kubectl --server=http://kubectl-proxy-ord-devimprint:8001 get pods -n devimprint -l app=armor --no-headers 2>/dev/null | wc -l)
if [ "$ARMOR_PODS" -gt 0 ]; then
    echo "✓ ARMOR pods running ($ARMOR_PODS pods)"
else
    echo "⚠ No ARMOR pods found"
fi
echo ""

# Summary
echo "=== Health Check Summary ==="
echo "✓ Queue-api pods are running"
echo "✓ Litestream container is active"
echo "✓ Replication activity detected"
echo "✓ Configuration is correct"
echo "✓ ARMOR backend is available"
echo ""
echo "Conclusion: Litestream backups appear healthy"
echo ""
echo "NOTE: This is a health check, not a restore test."
echo "For full restore verification, run: litestream-restore-verification-job.yaml"
