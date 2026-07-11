#!/bin/bash
# Litestream Restore Verification Test Script
# Task: bf-5aqh0
#
# This script tests the restore procedure for queue-api database from ARMOR backup.
# It simulates what would happen in a real disaster recovery scenario.
#
# PREREQUISITES:
# - litestream binary available (install from https://litestream.io)
# - Access to ARMOR service credentials
# - Network access to ARMOR endpoint

set -e

echo "=== Queue-API Restore Verification Test ==="
echo "Task: bf-5aqh0"
echo "Timestamp: $(date -Iseconds)"
echo ""

# Configuration from queue-api-litestream-config ConfigMap
LITESTREAM_CONFIG="
dbs:
  - path: /data/queue.db
    replica:
      type: s3
      bucket: devimprint
      path: state/litestream/queue.db
      endpoint: http://armor:9000
      force-path-style: true
      access-key-id: \${LITESTREAM_ACCESS_KEY_ID}
      secret-access-key: \${LITESTREAM_SECRET_ACCESS_KEY}
"

echo "CONFIGURATION:"
echo "==============="
echo "Database path: /data/queue.db"
echo "Backup location: S3 bucket 'devimprint', path 'state/litestream/queue.db'"
echo "Endpoint: http://armor:9000"
echo "Force path style: true"
echo ""

echo "EXPECTED BEHAVIOR:"
echo "=================="
echo "1. Litestream will connect to ARMOR at http://armor:9000"
echo "2. It will list available generations in devimprint/state/litestream/queue.db/"
echo "3. It will download the latest generation"
echo "4. It will restore the database to the specified output path"
echo ""

echo "RESTORE PROCEDURE:"
echo "=================="
echo ""
echo "Step 1: Create scratch directory"
echo "  mkdir -p /tmp/restore_test"
echo ""
echo "Step 2: Run litestream restore command"
echo "  litestream restore -v -o /tmp/restore_test/queue_restored.db /data/queue.db"
echo ""
echo "  Expected output:"
echo "    - Connecting to replica s3://devimprint/state/litestream/queue.db"
echo "    - Listing generations..."
echo "    - Found generation: <generation-id>"
echo "    - Downloading snapshot..."
echo "    - Applying WAL files..."
echo "    - Restore complete"
echo ""
echo "Step 3: Verify restored database"
echo "  ls -lh /tmp/restore_test/queue_restored.db"
echo "  sqlite3 /tmp/restore_test/queue_restored.db 'PRAGMA integrity_check;'"
echo "  sqlite3 /tmp/restore_test/queue_restored.db '.schema'"
echo "  sqlite3 /tmp/restore_test/queue_restored.db 'SELECT COUNT(*) FROM sqlite_master WHERE type=\"table\";'"
echo ""

echo "WHAT THIS TEST VERIFIES:"
echo "========================="
echo "✓ ARMOR backend is accessible and serving backup data"
echo "✓ Litestream can list available backup generations"
echo "✓ Complete generation chain exists (snapshot + WAL files)"
echo "✓ Data can be successfully downloaded and reassembled"
echo "✓ Restored database is not corrupted (SQLite integrity check)"
echo "✓ Database schema and data are intact"
echo "✓ File sizes are reasonable (no data loss)"
echo ""

echo "DISASTER RECOVERY IMPLICATIONS:"
echo "=================================="
echo "If this test succeeds, it confirms that:"
echo "  1. The backup generation created in bf-36zo2 is complete and valid"
echo "  2. ARMOR can successfully serve the backup data for restore"
echo "  3. In a real disaster, the restore procedure would work"
echo "  4. No data corruption exists in the backup chain"
echo "  5. The fresh snapshot fixed the multipart corruption bug"
echo ""

echo "KUBERNETES EXECUTION:"
echo "====================="
echo "To run this test in the cluster (requires write access):"
echo ""
echo "  kubectl apply -f ~/declarative-config/k8s/ord-devimprint/devimprint/litestream-restore-verification-job.yaml"
echo ""
echo "  # Monitor job execution"
echo "  kubectl get job litestream-restore-verification -n devimprint -w"
echo ""
echo "  # Check logs"
echo "  kubectl logs job/litestream-restore-verification -n devimprint"
echo ""
echo "  # Cleanup after successful test"
echo "  kubectl delete job litestream-restore-verification -n devimprint"
echo ""

echo "MANUAL VERIFICATION IN CLUSTER:"
echo "================================"
echo "If you have pod exec access, you can run this directly in the litestream sidecar:"
echo ""
echo "  kubectl exec -n devimprint queue-api-XXX -c litestream -- /bin/sh -c '"
echo "    mkdir -p /data/restore_test && \\"
echo "    litestream restore -o /data/restore_test/queue_restored.db /data/queue.db && \\"
echo "    sqlite3 /data/restore_test/queue_restored.db \"PRAGMA integrity_check;\""
echo "  '"
echo ""

echo "LOCAL TESTING (with credentials):"
echo "=================================="
echo "To test locally, you need ARMOR credentials:"
echo ""
echo "  export LITESTREAM_ACCESS_KEY_ID=<from-armor-writer-secret>"
echo "  export LITESTREAM_SECRET_ACCESS_KEY=<from-armor-writer-secret>"
echo "  # Port-forward to ARMOR service"
echo "  kubectl port-forward -n devimprint svc/armor 9000:9000"
echo "  # Run restore"
echo "  litestream restore -v -o /tmp/queue_restored.db <path-to-original-db-if-exists>"
echo ""

echo "SUCCESS CRITERIA:"
echo "=================="
echo "The test is successful when ALL of these pass:"
echo "  ✓ Restore command completes without errors"
echo "  ✓ Restored database file exists with reasonable size (> 1KB)"
echo "  ✓ SQLite PRAGMA integrity_check returns 'ok'"
echo "  ✓ Database contains expected tables (> 0 tables)"
echo "  ✓ File size is comparable to expected size"
echo "  ✓ No corruption or data loss detected"
echo ""

echo "DOCUMENTATION RESULTS:"
echo "======================="
echo "After successful verification:"
echo "  1. Document the generation ID that was tested"
echo "  2. Note the restore time and file size"
echo "  3. Update disaster-recovery.md with verified generation"
echo "  4. Schedule quarterly restore tests"
echo ""

echo "NEXT STEPS:"
echo "==========="
echo "Since this environment has read-only cluster access:"
echo "  1. The verification job YAML is already in declarative-config"
echo "  2. ArgoCD will sync it automatically (if not already synced)"
echo "  3. Monitor for job execution"
echo "  4. Document results in this directory"
echo "  5. Update runbooks with verified generation ID"
echo ""

echo "=== Script Complete ==="
echo "This script documents the restore verification procedure."
echo "Execute the commands above to perform the actual test."
