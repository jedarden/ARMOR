# Restore-Verifier Deployment Guide

## Overview

The restore-verifier is a standalone service for continuous backup verification that runs dual-path verification (ARMOR read path + armor decrypt direct) to prove that backups are restorable through both the normal server path and disaster recovery.

## Deployment Configuration

### Standard Deployment

For standard deployments where all objects are encrypted with a single MEK:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: restore-verifier-config
data:
  ARMOR_B2_REGION: "us-west-004"
  ARMOR_B2_ENDPOINT: "https://s3.us-west-004.backblazeb2.com"
  ARMOR_BUCKET: "my-bucket"
  ARMOR_MEK: "<64-char-hex-mek>"  # Active MEK only
  VERIFIER_CHECK_INTERVAL: "6h"
  VERIFIER_SAMPLE_SIZE: "10"
  VERIFIER_HTTP_LISTEN: ":9002"
```

### Key Rotation Deployment (MEK Ring)

During key rotation (Plan §8.13), the restore-verifier requires access to both the active MEK and any retired MEKs that may still have objects encrypted with them. This ensures the verifier can successfully decrypt and verify all objects in the bucket, regardless of which key was used to encrypt them.

#### Configuration with MEK Ring

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: restore-verifier-config
data:
  ARMOR_B2_REGION: "us-west-004"
  ARMOR_B2_ENDPOINT: "https://s3.us-west-004.backblazeb2.com"
  ARMOR_BUCKET: "my-bucket"
  ARMOR_MEK: "<64-char-hex-active-mek>"          # Active MEK (current)
  VERIFIER_MEK_RING: "<old-key-hex>,<another-old-key-hex>"  # Retired MEKs (comma-separated)
  VERIFIER_CHECK_INTERVAL: "6h"
  VERIFIER_SAMPLE_SIZE: "10"
  VERIFIER_HTTP_LISTEN: ":9002"
```

#### Key Rotation Procedure

1. **Before Rotation**: Deploy restore-verifier with only the active MEK
   ```yaml
   ARMOR_MEK: "<current-mek>"
   VERIFIER_MEK_RING: ""  # Empty - no retired keys
   ```

2. **During Rotation**: Add retired keys to the ring
   ```yaml
   ARMOR_MEK: "<new-active-mek>"
   VERIFIER_MEK_RING: "<old-mek-1>,<old-mek-2>"  # All retired keys
   ```

3. **After Rotation Complete**: Remove retired keys from the ring
   ```yaml
   ARMOR_MEK: "<current-mek>"
   VERIFIER_MEK_RING: ""  # Empty when no objects remain on old keys
   ```

#### Verification Strategy

The restore-verifier uses fingerprint-based key selection (Plan §8.13):
- **V2 format objects** (wrapped DEK includes fingerprint): Direct key selection by fingerprint from active key or ring
- **Legacy format objects** (no fingerprint): Trial unwrapping with active key, then each ring key in order

This means:
- Objects encrypted with the active key always work
- Objects encrypted with retired keys in the ring always work
- Objects encrypted with unknown keys fail with `ErrFingerprintNotFound`

## Environment Variables

### Required
- `ARMOR_B2_REGION`: B2 region (e.g., `us-west-004`)
- `ARMOR_B2_ENDPOINT`: B2 S3 API endpoint
- `ARMOR_B2_ACCESS_KEY_ID`: B2 application key ID
- `ARMOR_B2_SECRET_ACCESS_KEY`: B2 application key
- `ARMOR_MEK`: Master encryption key (hex, 64 chars)

### Optional
- `VERIFIER_MEK_RING`: Comma-separated list of retired MEKs (hex, 64 chars each) for key rotation support
- `VERIFIER_CHECK_INTERVAL`: Verification check interval (default: `6h`)
- `VERIFIER_SAMPLE_SIZE`: Historical sample size (default: `10`)
- `VERIFIER_HTTP_LISTEN`: HTTP listen address (default: `:9002`)
- `VERIFIER_DR_DRILL_INTERVAL`: Direct-only DR drill interval (default: disabled)

## HTTP Endpoints

- `GET /status`: Verification status for all buckets
- `GET /bucket?bucket=X`: Status for specific bucket
- `POST /trigger`: Trigger immediate verification run (dual path)
- `POST /trigger?mode=dr-drill`: Trigger direct-only DR drill
- `GET /healthz`: Liveness check
- `GET /readyz`: Readiness check
- `GET /metrics`: Prometheus metrics

## Metrics

The restore-verifier exposes Prometheus metrics:

- `armor_restore_verification_status_total`: Total verification runs by status
- `armor_restore_verification_duration_seconds`: Verification run duration
- `armor_restore_verification_failures_total`: Total verification failures
- `armor_restore_path_comparison_total`: Dual-path comparison results

## Operational Notes

### Key Rotation Transition

When rotating MEKs (Plan §8.13):

1. **Add new key to OpenBao** as the active key
2. **Update restore-verifier ConfigMap** to include the old key in `VERIFIER_MEK_RING`
3. **Rolling restart** the restore-verifier deployment
4. **Monitor metrics** - verification failures should not increase
5. **Run POST /admin/key/rotate** on ARMOR server to re-wrap objects
6. **Remove old key from `VERIFIER_MEK_RING`** once all objects are re-wrapped
7. **Final rolling restart** to clean up the ring

### Verification During Rotation

- **Before rotation**: All objects encrypted with current key → 100% success
- **During rotation**: Mixed objects (current + old keys) → 100% success with ring configured
- **After rotation**: All objects encrypted with new key → 100% success

### Troubleshooting

**Symptom**: `restore-verifier` reports checksum conflicts on all objects

**Cause**: MEK ring not configured during key rotation

**Solution**: Add retired MEKs to `VERIFIER_MEK_RING` environment variable

**Symptom**: `ErrFingerprintNotFound` in logs

**Cause**: Object encrypted with unknown MEK not in active key or ring

**Solution**: Verify the retired MEK is included in `VERIFIER_MEK_RING` and the fingerprint matches

## Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restore-verifier
  labels:
    app: restore-verifier
spec:
  replicas: 1
  selector:
    matchLabels:
      app: restore-verifier
  template:
    metadata:
      labels:
        app: restore-verifier
    spec:
      containers:
      - name: restore-verifier
        image: ronaldraygun/armor-restore-verifier:0.1.1913
        envFrom:
        - configMapRef:
            name: restore-verifier-config
        - secretRef:
            name: armor-b2-credentials
        ports:
        - containerPort: 9002
          name: http
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
        readinessProbe:
          httpGet:
            path: /readyz
            port: http
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

## References

- Plan §8.13: MEK key ring — multiple concurrent MEKs
- ADR-004: Continuous restore verification
- ADR-009: Restore-verifier ARMOR path never decrypts
- ADR-014: Restore-verifier discovery reliability
