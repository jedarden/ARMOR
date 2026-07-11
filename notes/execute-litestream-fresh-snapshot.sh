#!/bin/bash
# Execution script for bf-36zo2: Force Fresh Litestream Backup Baseline
# This script requires write access to ord-devimprint cluster
# Usage: ./execute-litestream-fresh-snapshot.sh

set -e

NAMESPACE="devimprint"
DEPLOYMENT="queue-api"
JOB_NAME="litestream-force-fresh-snapshot"
JOB_FILE="/home/coding/ARMOR/notes/litestream-force-fresh-snapshot-job.yaml"
KUBECTL_CMD="kubectl"

echo "=========================================="
echo "Litestream Fresh Snapshot Execution (bf-36zo2)"
echo "Date: $(date)"
echo "=========================================="
echo ""

# Check if job file exists
if [ ! -f "$JOB_FILE" ]; then
    echo "ERROR: Job file not found: $JOB_FILE"
    exit 1
fi

# Step 1: Scale down deployment
echo "Step 1: Scaling down $DEPLOYMENT to 0 replicas..."
$KUBECTL_CMD scale deployment $DEPLOYMENT --replicas=0 -n $NAMESPACE
echo "✓ Deployment scaled down"
echo ""

# Step 2: Wait for pod termination
echo "Step 2: Waiting for pod termination..."
$KUBECTL_CMD wait --for=delete pod -l app=$DEPLOYMENT -n $NAMESPACE --timeout=60s
echo "✓ Pods terminated"
echo ""

# Step 3: Apply the job
echo "Step 3: Applying litestream reset job..."
$KUBECTL_CMD apply -f $JOB_FILE
echo "✓ Job applied"
echo ""

# Step 4: Monitor job completion
echo "Step 4: Waiting for job completion (max 5 minutes)..."
$KUBECTL_CMD wait --for=condition=complete job/$JOB_NAME -n $NAMESPACE --timeout=300s
echo "✓ Job completed successfully"
echo ""

# Step 5: Check job logs
echo "Step 5: Job logs:"
echo "------------------------------------------"
$KUBECTL_CMD logs job/$JOB_NAME -n $NAMESPACE
echo "------------------------------------------"
echo ""

# Step 6: Scale up deployment
echo "Step 6: Scaling $DEPLOYMENT back to 1 replica..."
$KUBECTL_CMD scale deployment $DEPLOYMENT --replicas=1 -n $NAMESPACE
echo "✓ Deployment scaled up"
echo ""

# Step 7: Wait for pod ready
echo "Step 7: Waiting for pod to be ready..."
$KUBECTL_CMD wait --for=condition=ready pod -l app=$DEPLOYMENT -n $NAMESPACE --timeout=120s
echo "✓ Pod is ready"
echo ""

# Step 8: Verify fresh snapshot creation
echo "Step 8: Checking litestream logs for fresh snapshot..."
echo "Looking for 'snapshot' or 'generation' messages..."
echo "------------------------------------------"
$KUBECTL_CMD logs deployment/$DEPLOYMENT -c litestream -n $NAMESPACE --tail=50 | grep -E "(snapshot|generation)" || echo "No explicit snapshot messages found in recent logs"
echo "------------------------------------------"
echo ""

# Step 9: Cleanup prompt
echo "Step 9: Cleanup"
echo "Job completed. You can now clean up the job:"
echo "  $KUBECTL_CMD delete job $JOB_NAME -n $NAMESPACE"
echo ""

echo "=========================================="
echo "Execution Complete!"
echo "=========================================="
echo ""
echo "Next Steps:"
echo "1. Verify fresh snapshot in litestream logs (above)"
echo "2. Note the generation ID for verification (bf-5uehq)"
echo "3. Monitor ongoing replication for any errors"
echo "4. Clean up the job (optional)"
echo ""
echo "Documentation:"
echo "  - Status: /home/coding/ARMOR/notes/bf-36zo2-execution-status.md"
echo "  - Guide: /home/coding/ARMOR/notes/bf-36zo2-litestream-fresh-snapshot-guide.md"
echo ""
