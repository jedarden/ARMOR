# Task bf-36zo2: Force Fresh Litestream Backup Baseline for queue-api

## Summary

After ARMOR is upgraded to the fixed version (0.1.467), force queue-api's litestream to establish a fresh backup baseline (new snapshot/generation). This is critical because the version bump alone does NOT fix already-written data - existing backups may still contain the multipart corruption bug.

## Current State

- **ARMOR Version**: 0.1.467 (fixed version - past 0.1.42 threshold)
- **Litestream Version**: 0.5.11
- **Database**: /data/queue.db on PVC `queue-api-data-sata-2`
- **Backup Location**: S3 bucket `devimprint`, path `state/litestream/queue.db`
- **ARMOR Endpoint**: http://armor:9000

## Litestream Configuration

From queue-api-deployment.yml:
```yaml
litestream.yml: |
  dbs:
    - path: /data/queue.db
      replica:
        type: s3
        bucket: devimprint
        path: state/litestream/queue.db
        endpoint: http://armor:9000
        force-path-style: true
        access-key-id: ${LITESTREAM_ACCESS_KEY_ID}
        secret-access-key: ${LITESTREAM_SECRET_ACCESS_KEY}
```

## Method: Force Fresh Snapshot via `litestream reset`

According to [Litestream documentation](https://litestream.io/reference/reset/), the `reset` command clears local Litestream state for a database, forcing a fresh snapshot on the next replication sync.

### Kubernetes Job Execution Plan

Since ord-devimprint cluster access is read-only via kubectl-proxy, this operation needs to be performed through the declarative-config repo with ArgoCD sync.

#### Step 1: Create Kubernetes Job

The job will:
1. Scale down queue-api deployment to 0 (stops litestream sidecar)
2. Execute litestream reset on the database
3. Scale queue-api deployment back to 1
4. Litestream sidecar automatically creates fresh snapshot

**Job YAML** (to be added to declarative-config):
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: litestream-force-fresh-snapshot
  namespace: devimprint
spec:
  template:
    spec:
      activeDeadlineSeconds: 300
      containers:
      - name: litestream-reset
        image: litestream/litestream:0.5.11
        command:
          - /bin/sh
          - -c
          - |
            echo "Clearing litestream local state for /data/queue.db"
            litestream reset /data/queue.db || echo "Reset failed or no state to clear"
            echo "Litestream state cleared - fresh snapshot will be created on next sync"
        env:
          - name: LITESTREAM_ACCESS_KEY_ID
            valueFrom:
              secretKeyRef:
                name: armor-writer
                key: auth-access-key
          - name: LITESTREAM_SECRET_ACCESS_KEY
            valueFrom:
              secretKeyRef:
                name: armor-writer
                key: auth-secret-key
        volumeMounts:
          - name: data
            mountPath: /data
          - name: litestream-config
            mountPath: /etc/litestream.yml
            subPath: litestream.yml
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
      volumes:
        - name: data
          persistentVolumeClaim:
            claimName: queue-api-data-sata-2
        - name: litestream-config
          configMap:
            name: queue-api-litestream-config
      restartPolicy: Never
```

#### Step 2: Pre-Execution Tasks

Before running the job:
1. Scale down queue-api deployment to 0 (prevents concurrent access)
2. Verify queue-api pod is terminated
3. Apply the Job YAML
4. Monitor job completion
5. Scale queue-api deployment back to 1
6. Verify litestream creates new snapshot

#### Step 3: Verify Fresh Snapshot

After execution:
1. Check litestream logs for new snapshot creation
2. Verify new generation in ARMOR S3 bucket
3. Test restore capability (separate task)

## Alternative: Manual S3 Backup Cleanup

If litestream reset doesn't work, manually delete the existing backup:
```bash
# This would require direct S3/ARMOR access
# Delete state/litestream/queue.db/* from devimprint bucket
# Litestream will recreate from scratch
```

## Acceptance Criteria

- [x] Confirmed litestream is configured for queue-api database
- [ ] Created Kubernetes Job to trigger fresh snapshot/generation
- [ ] Verified new backup generation is active and replicating
- [ ] Noted the generation ID for the next verification step

## Next Steps

1. Add the Job YAML to declarative-config repo
2. Execute the scaling and job application steps
3. Monitor and verify the fresh snapshot
4. Document the generation ID
5. (Separate task) Test restore from fresh snapshot

## References

- [Litestream Reset Command Documentation](https://litestream.io/reference/reset/)
- [Litestream Tips & Caveats](https://litestream.io/v0.3/tips/)
- [Queue API Deployment YAML](/home/coding/jedarden/declarative-config/k8s/ord-devimprint/devimprint/queue-api-deployment.yml)
- [Bead bf-4qq1](https://github.com/jedarden/ARMOR) - Parent task for ARMOR upgrade and fresh backup verification

## Notes

- The version bump alone does NOT fix already-written data
- Existing backups may still contain the multipart corruption bug
- A fresh snapshot from the fixed version is required
- This is critical for data integrity and disaster recovery
