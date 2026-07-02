# ARMOR Disaster Recovery Runbook

This document covers disaster recovery procedures for ARMOR deployments, including MEK backup/escrow, restore drills, key rotation failure recovery, and data recovery limitations.

## TL;DR Critical Points

1. **The MEK (Master Encryption Key) is the single point of failure** — losing it means losing all data. Never rotate the MEK without first exporting and escrowing the current MEK.
2. **All operational state lives in B2** — a fresh ARMOR instance with the same config (MEK + B2 creds + Cloudflare domain) can recover all data.
3. **The `.armor/` prefix is reserved and must be preserved** — losing sidecar files (`.armor/hmac/*`, `.armor/rotation-state.json`, `.armor/multipart/*.state`) makes corresponding objects unrecoverable.
4. **Per-file DEKs are wrapped by the MEK** — if you lose the MEK, every object's wrapped DEK becomes useless, even though the ciphertext is intact.

## Table of Contents

1. [MEK Backup and Escrow](#mek-backup-and-escrow)
2. [Restore Drill: Recovering from Complete Deployment Loss](#restore-drill-recovering-from-complete-deployment-loss)
3. [Key Rotation Failure Recovery](#key-rotation-failure-recovery)
4. [Multipart Upload Recovery](#multipart-upload-recovery)
5. [What is NOT Recoverable](#what-is-not-recoverable)
6. [Verification and Testing](#verification-and-testing)

---

## MEK Backup and Escrow

The MEK is the cryptographic root of trust for all encrypted objects. Without it, all wrapped DEKs are useless and all data is permanently lost.

### Exporting the MEK

The ARMOR admin API provides a `/admin/key/export` endpoint that returns the current MEK in hex format.

```bash
# Export the default MEK
curl -s "http://localhost:9001/admin/key/export?confirm=yes"
```

**Response:**
```json
{
  "mek": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "format": "hex",
  "warning": "This key is the single point of failure for all encrypted data. Store it securely and never lose it."
}
```

### Exporting Named MEKs (Multi-Key Deployments)

If your deployment uses `ARMOR_KEY_ROUTES` for multi-key routing, each key must be exported separately. The admin API only exports the default MEK. For named keys, retrieve them from your secret store:

```bash
# Example: Retrieve named keys from Kubernetes secrets
kubectl get secret armor-secrets -o jsonpath='{.data.sensitive-mek}' | base64 -d
kubectl get secret armor-secrets -o jsonpath='{.data.archive-mek}' | base64 -d
```

### Escrow Requirements

Escrow copies must satisfy these properties:

| Property | Description | Examples of Valid Storage |
|----------|-------------|---------------------------|
| **Offline storage** | Not accessible via network or API | Hardware security module (HSM), encrypted USB drive, paper backup in safe deposit box |
| **Access control** | Strict authorization required to retrieve | Corporate secret manager with approval workflows, physical safe with access log |
| **Durability** | Survives disasters, personnel changes | Multiple geographic locations, redundancy across providers |
| **Auditability** | All access attempts are logged | Secret manager with audit logs, physical access logs for safe |
| **Versioning** | Retains history of MEK versions | Timestamped exports, version-numbered backups |

**Valid escrow locations (examples, not endorsements):**
- Cloud KMS (AWS KMS, GCP KMS, Azure Key Vault) with audit logging
- Hardware security modules (HSM) with FIPS 140-2 Level 3+
- Encrypted backups stored in separate physical locations
- Paper copies stored in secure physical vaults (fireproof, access-controlled)

**Invalid escrow locations:**
- Unencrypted files on disk
- Public cloud storage without encryption
- Shared documents without access controls
- Environment variables in CI/CD systems

### MEK Rotation Pre-Flight Checklist

Before rotating the MEK, you MUST:

1. **Export the current MEK** and verify the export completes successfully:
   ```bash
   curl -s "http://localhost:9001/admin/key/export?confirm=yes" | jq -r '.mek' > /secure/path/mek-backup-$(date +%Y%m%d).hex
   sha256sum /secure/path/mek-backup-*.hex  # Verify checksum
   ```

2. **Escrow the current MEK** in your secure location of choice.

3. **Verify the escrow** by retrieving it and comparing checksums:
   ```bash
   # After escrow, retrieve and verify
   escrowed_mek=$(retrieve-from-escrow)
   current_mek=$(curl -s "http://localhost:9001/admin/key/export?confirm=yes" | jq -r '.mek')
   if [ "$escrowed_mek" != "$current_mek" ]; then
     echo "ERROR: Escrowed MEK does not match current MEK"
     exit 1
   fi
   ```

4. **Verify canary health** to ensure current MEK is valid:
   ```bash
   curl -s http://localhost:9001/admin/key/verify | jq .
   ```

**Only proceed with rotation after all four steps complete successfully.**

### Why This Order Matters

Per-file DEKs are wrapped with the MEK. If you rotate the MEK without escrowing the old MEK:

- All existing objects are re-wrapped with the new MEK
- The old MEK is discarded
- If the rotation is interrupted or fails partway through, some objects may still be wrapped with the old MEK
- Without the old MEK escrowed, those objects become permanently unreadable

**Rule:** A MEK must never be rotated without first verifying escrow of the old MEK.

---

## Restore Drill: Recovering from Complete Deployment Loss

This procedure covers recovery when the ARMOR deployment is completely lost (pod deleted, cluster destroyed, configuration lost) but the B2 bucket and escrowed MEK survive.

### Scenario

You've lost:
- The ARMOR pod/container
- Kubernetes Deployment configuration
- Environment variables
- Access to the original cluster

You still have:
- B2 bucket (all encrypted objects intact)
- Escrowed MEK
- B2 credentials
- Cloudflare domain

### Step 1: Retrieve Escrowed MEK

Retrieve the MEK from your escrow location. Example patterns:

```bash
# From a secret manager
aws secretsmanager get-secret-value --secret-id armor-mek-prod | jq -r '.SecretString'

# From a file on encrypted media
cryptsetup open /dev/sdX encrypted_backup
cp /mnt/backup/mek.hex ~/mek-recovered.hex
cryptsetup close encrypted_backup

# From a paper backup
# (Type in the hex value manually)
```

Verify the retrieved MEK is 64 hex characters (32 bytes):

```bash
mek=$(cat ~/mek-recovered.hex)
if [ ${#mek} -ne 64 ] || ! [[ $mek =~ ^[0-9a-fA-F]{64}$ ]]; then
  echo "ERROR: Invalid MEK format (must be 64 hex chars)"
  exit 1
fi
```

### Step 2: Deploy Fresh ARMOR Instance

Create a new ARMOR deployment with the recovered MEK and original B2 credentials.

#### Kubernetes Deployment

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-secrets
type: Opaque
stringData:
  b2-access-key-id: "your-b2-key-id"
  b2-secret-access-key: "your-b2-secret-key"
  master-encryption-key: "0123456789abcdef..." # From escrow
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: armor-config
data:
  ARMOR_B2_REGION: "us-east-005"
  ARMOR_BUCKET: "your-bucket-name"
  ARMOR_CF_DOMAIN: "armor-b2.example.com"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: armor
spec:
  replicas: 1
  selector:
    matchLabels:
      app: armor
  template:
    metadata:
      labels:
        app: armor
    spec:
      containers:
      - name: armor
        image: ronaldraygun/armor:0.1.43
        ports:
        - containerPort: 9000
        env:
        - name: ARMOR_B2_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: b2-access-key-id
        - name: ARMOR_B2_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: b2-secret-access-key
        - name: ARMOR_MEK
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: master-encryption-key
        - name: ARMOR_BUCKET
          valueFrom:
            configMapKeyRef:
              name: armor-config
              key: ARMOR_BUCKET
        - name: ARMOR_B2_REGION
          valueFrom:
            configMapKeyRef:
              name: armor-config
              key: ARMOR_B2_REGION
        - name: ARMOR_CF_DOMAIN
          valueFrom:
            configMapKeyRef:
              name: armor-config
              key: ARMOR_CF_DOMAIN
```

```bash
kubectl apply -f armor-recovery.yaml
kubectl wait --for=condition=available --timeout=60s deployment/armor
```

#### Docker Deployment

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e ARMOR_B2_REGION=us-east-005 \
  -e ARMOR_B2_ACCESS_KEY_ID=your-key-id \
  -e ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret \
  -e ARMOR_BUCKET=your-bucket \
  -e ARMOR_CF_DOMAIN=your-cf-domain.example.com \
  -e ARMOR_MEK=$(cat ~/mek-recovered.hex) \
  -e ARMOR_AUTH_ACCESS_KEY=my-access-key \
  -e ARMOR_AUTH_SECRET_KEY=my-secret-key \
  ronaldraygun/armor:0.1.43
```

### Step 3: Verify MEK Against Canary

The canary is a self-healing integrity monitor that ARMOR maintains in the bucket. Verifying the canary confirms the MEK is correct and the full pipeline (encryption → B2 → Cloudflare → decryption) is working.

```bash
# Via kubectl
kubectl exec deploy/armor -- curl -s http://localhost:9001/admin/key/verify | jq .

# Via Docker
docker exec armor-container curl -s http://localhost:9001/admin/key/verify | jq .

# Via port-forward
kubectl port-forward svc/armor 9001:9001
curl -s http://localhost:9001/admin/key/verify | jq .
```

**Expected response:**
```json
{
  "status": "verified",
  "message": "MEK is correct and canary decrypted successfully"
}
```

**Failure response:**
```json
{
  "status": "failed",
  "message": "MEK verification failed: HMAC mismatch"
}
```

If verification fails, the MEK is incorrect — retrieve it from escrow again and verify the escrow checksum.

### Step 4: Check Canary Health

```bash
curl -s http://localhost:9001/armor/canary | jq .
```

**Expected response:**
```json
{
  "status": "healthy",
  "last_check": "2026-07-02T12:00:00Z",
  "consecutive_successes": 42,
  "consecutive_failures": 0,
  "upload_latency_ms": 45,
  "download_latency_ms": 12
}
```

### Step 5: Spot-Check Decrypt a Few Objects

Verify that actual encrypted objects can be decrypted correctly. Use the S3 API to list and download some objects.

```bash
# Configure AWS CLI for ARMOR
export AWS_ACCESS_KEY_ID=my-access-key
export AWS_SECRET_ACCESS_KEY=my-secret-key
export AWS_ENDPOINT_URL=http://localhost:9000

# List objects
aws s3 ls --endpoint-url $AWS_ENDPOINT_URL s3://your-bucket/ | head -10

# Download and decrypt a few objects
aws s3 cp --endpoint-url $AWS_ENDPOINT_URL s3://your-bucket/data/sensor-readings.parquet /tmp/test-recovery.parquet

# Verify the downloaded file is valid (example: Parquet)
ducksql -c "SELECT COUNT(*) FROM '/tmp/test-recovery.parquet';"
# Or
parquet-tools schema /tmp/test-recovery.parquet
```

For a more thorough validation, use the offline decrypt CLI to verify a few objects directly from B2:

```bash
# Build the decrypt tool (if not already built)
go build -o armor-decrypt ./cmd/armor-decrypt

# Decrypt a specific object from B2
armor-decrypt \
  -mek $(cat ~/mek-recovered.hex) \
  -input b2://your-bucket/data/sensor-readings.parquet \
  -output /tmp/verify-decrypt.parquet

# Verify checksum matches
sha256sum /tmp/verify-decrypt.parquet
```

### Step 6: Verify Metadata Cache and Manifest (if enabled)

If your deployment uses the manifest index, verify it loads correctly on startup:

```bash
# Check logs for manifest loading
kubectl logs -f deploy/armor | grep -i manifest
```

Look for:
```
manifest index loaded: 1500 entries from snapshot + 3 deltas
```

### Step 7: Run Full System Verification

If you have integration tests, run them against the recovered deployment:

```bash
# From the tests/integration directory
go test -v -tags=integration ./...
```

---

## Key Rotation Failure Recovery

ARMOR tracks key rotation progress in `.armor/rotation-state.json` in the B2 bucket. This allows rotation to resume safely if interrupted.

### Detecting an Incomplete Rotation

Check for the presence of the rotation state file:

```bash
# List .armor/ prefix to find rotation state
aws s3 ls --endpoint-url http://localhost:9000 s3://your-bucket/.armor/
```

Look for `.armor/rotation-state.json`. If present, download and inspect it:

```bash
aws s3 cp --endpoint-url http://localhost:9000 s3://your-bucket/.armor/rotation-state.json - | jq .
```

**Rotation state format:**
```json
{
  "status": "in_progress",
  "old_mek_sha256": "abc123...",
  "new_mek_sha256": "def456...",
  "started_at": "2026-07-02T10:00:00Z",
  "last_object_processed": "data/file-5000.parquet",
  "total_objects": 10000,
  "processed_objects": 5000,
  "failed_objects": []
}
```

If `status` is `"in_progress"`, rotation was interrupted.

### Rotation Failure Modes

| Failure Mode | Detection | Recovery Action |
|--------------|-----------|-----------------|
| ARMOR pod crashed during rotation | `.armor/rotation-state.json` exists with `status: in_progress` | Restart rotation via `/admin/key/rotate` — it will resume automatically |
| B2 API rate limit | Rotation endpoint returns error with retry-after | Wait and retry — rotation is idempotent, already-processed objects are skipped |
| Network timeout on CopyObject | Failed objects listed in rotation state | Retry rotation — only failed objects are reprocessed |
| Old MEK lost before rotation completed | `old_mek_sha256` in state doesn't match any escrowed MEK | **Data loss for objects not yet re-wrapped** — restore old MEK from backup or use offline decrypt with old MEK |

### Resuming an Interrupted Rotation

To resume rotation, simply POST to the `/admin/key/rotate` endpoint again with the same new MEK:

```bash
# Rotation will automatically resume from .armor/rotation-state.json
curl -s -X POST http://localhost:9001/admin/key/rotate \
  -H "Content-Type: application/json" \
  -d '{"new_mek": "0123456789abcdef..."}' | jq .
```

ARMOR reads `.armor/rotation-state.json`, determines which objects were successfully processed, and continues from where it left off.

**Important:** The `new_mek` in the request body must match the SHA256 hash in `rotation-state.json.new_mek_sha256`. If you submit a different new MEK, rotation will fail with an error.

### Bucket Versioning Implications

**ARMOR buckets do NOT have versioning enabled** in the default configuration. This means:

- `CopyObject` during rotation overwrites objects in place
- The old wrapped DEK (in `x-amz-meta-armor-wrapped-dek`) is replaced
- There is no rollback mechanism if rotation completes but the new MEK is lost

**If you enable bucket versioning:**

- Each CopyObject creates a new version
- Non-current versions accumulate (cost impact)
- After successful rotation, you must expire non-current versions to avoid data leakage (old wrapped DEKs remain accessible)

**Recommendation:** Keep bucket versioning disabled for simpler operations. Enable it only if you have explicit rollback requirements and a lifecycle rule to expire old versions.

---

## Multipart Upload Recovery

Multipart uploads in ARMOR store HMAC tables as sidecar objects at `.armor/hmac/<sha256-of-key>`. These sidecars are essential for decryption — without them, multipart-uploaded objects are unreadable.

### Detecting Orphaned Multipart State

When a multipart upload is interrupted (client crash, network failure), ARMOR leaves behind:

1. `.armor/multipart/<upload-id>.state` — Encrypted state object
2. Potential incomplete parts in B2
3. No `.armor/hmac/<sha256>` sidecar (because CompleteMultipartUpload never ran)

**Detection:**

```bash
# List multipart state objects
aws s3 ls --endpoint-url http://localhost:9000 s3://your-bucket/.armor/multipart/ | wc -l

# If count > 0, you have incomplete uploads
```

### Recovering or Cleaning Up Incomplete Uploads

ARMOR provides an automatic cleanup mechanism:

```bash
# List and abort incomplete multipart uploads
aws s3api list-multipart-uploads --endpoint-url http://localhost:9000 --bucket your-bucket

# Abort a specific upload
aws s3api abort-multipart-upload \
  --endpoint-url http://localhost:9000 \
  --bucket your-bucket \
  --key data/large-file.bin \
  --upload-id <upload-id>
```

After aborting, the `.armor/multipart/<upload-id>.state` object is deleted automatically by ARMOR's state manager.

### Preserving `.armor/hmac/` Sidecars

**Never delete objects under `.armor/hmac/`** — these are the HMAC tables for multipart-uploaded objects. Deleting a sidecar makes the corresponding object permanently unreadable.

**The `.armor/` prefix is protected by ARMOR's S3 handler:**

```bash
# This returns 403 AccessDenied
aws s3 rm --endpoint-url http://localhost:9000 s3://your-bucket/.armor/hmac/abc123...
```

However, direct B2 API calls can bypass ARMOR and delete these objects. **Never use the B2 native API to delete `.armor/` objects.**

---

## What is NOT Recoverable

Even with perfect MEK escrow and B2 durability, some data loss scenarios are unrecoverable.

### 1. Data Written After Last MEK Escrow

If you rotate the MEK without escrowing the old MEK, and then lose the new MEK, all data encrypted with the new MEK is permanently lost.

**Mitigation:** Always escrow the current MEK before rotation.

### 2. Objects with Destroyed Metadata

If an object's `x-amz-meta-armor-*` headers are deleted or corrupted, the wrapped DEK is lost. Without the wrapped DEK, the object cannot be decrypted even with the correct MEK.

**How this happens:**
- Direct B2 API calls that modify object metadata
- Third-party tools that touch B2 objects outside ARMOR
- Accidental `CopyObject` with `MetadataDirective: REPLACE` that strips ARMOR metadata

**Mitigation:**
- Never use non-ARMOR tools to modify objects in the bucket
- Set B2 bucket lifecycle rules to prevent accidental metadata mutation
- Enable provenance chain auditing (`/admin/audit`) to detect tampering

### 3. Lost `.armor/hmac/` Sidecars

If a multipart-uploaded object's HMAC sidecar (`.armor/hmac/<sha256>`) is deleted, the object cannot be verified during decryption.

**Detection:** Decryption fails with "HMAC table missing" error.

**Mitigation:**
- Never use the B2 native API to delete `.armor/` objects
- The ARMOR S3 handler blocks `.armor/` deletions (returns 403)
- If you must use B2 native tools, explicitly exclude `.armor/` prefix

### 4. MEK Lost Without Escrow

If the MEK is lost and no escrow exists, all data in the bucket is permanently unrecoverable. Ciphertext without the MEK is cryptographically indistinguishable from random bytes.

**This is the single catastrophic failure mode.**

**Mitigation:**
- Escrow the MEK before first deployment
- Escrow the MEK before every rotation
- Store escrow in multiple durable locations
- Test escrow retrieval regularly

### 5. B2 Bucket Deletion

If the B2 bucket itself is deleted, all objects are gone. B2 does not provide undelete or bucket recovery.

**Mitigation:**
- Enable B2 bucket versioning (if not using ARMOR rotation)
- Use B2 lifecycle rules to archive to a separate bucket
- Cross-region replicate to a separate B2 account
- Regular backups to cold storage (Glacier, B2 Cold Storage)

---

## Verification and Testing

### Testing MEK Escrow Retrieval

Regularly test that you can retrieve and use the escrowed MEK:

```bash
# 1. Retrieve from escrow
mek=$(retrieve-from-escrow)

# 2. Deploy test ARMOR instance with retrieved MEK
docker run -d --name armor-test \
  -e ARMOR_MEK=$mek \
  -e ARMOR_BUCKET=test-bucket \
  ... (other config)

# 3. Verify against canary
docker exec armor-test curl -s http://localhost:9001/admin/key/verify | jq .

# 4. Clean up
docker stop armor-test && docker rm armor-test
```

Run this test quarterly and after any escrow system changes.

### Running a Full Restore Drill

Once per year, perform a full restore drill in a non-production environment:

1. **Simulate complete deployment loss** — Delete the ARMOR pod and configuration
2. **Retrieve MEK from escrow** — Use your documented escrow retrieval procedure
3. **Deploy fresh ARMOR instance** — Follow the restore drill exactly
4. **Verify decryption of sample objects** — Download and decrypt 10-100 random objects
5. **Verify canary health** — Confirm `/armor/canary` returns `healthy`
6. **Run integrity audit** — Execute `/admin/audit` and verify no chain gaps

Document the drill results and any issues encountered.

### Testing Key Rotation Failure Recovery

Before deploying ARMOR to production, test rotation interruption:

```bash
# 1. Upload test data
for i in {1..100}; do
  aws s3 cp --endpoint-url http://localhost:9000 test-file.bin s3://test-bucket/file-$i.bin
done

# 2. Start rotation
curl -X POST http://localhost:9001/admin/key/rotate \
  -H "Content-Type: application/json" \
  -d '{"new_mek": "fedcba9876543210..."}' &

# 3. Kill ARMOR after 5 seconds (simulating crash)
sleep 5
kubectl delete pod armor-xxx-yyy

# 4. Restart ARMOR and verify rotation resumes
kubectl wait --for=condition=available deployment/armor
curl -s http://localhost:9001/admin/key/rotate \
  -H "Content-Type: application/json" \
  -d '{"new_mek": "fedcba9876543210..."}' | jq .

# 5. Verify all files still decrypt
for i in {1..100}; do
  aws s3 cp --endpoint-url http://localhost:9000 s3://test-bucket/file-$i.bin - | wc -c
done
```

---

## Appendix: Quick Reference Commands

### Export and Escrow MEK

```bash
# Export MEK
curl -s "http://localhost:9001/admin/key/export?confirm=yes" | jq -r '.mek' > mek-backup-$(date +%Y%m%d).hex

# Verify export
sha256sum mek-backup-*.hex

# Escrow (example: AWS Secrets Manager)
aws secretsmanager create-secret \
  --name armor-mek-prod-$(date +%Y%m%d) \
  --secret-string file://mek-backup-$(date +%Y%m%d).hex
```

### Verify MEK

```bash
# Via admin API
curl -s http://localhost:9001/admin/key/verify | jq .

# Via canary status
curl -s http://localhost:9001/armor/canary | jq .
```

### Check Rotation Status

```bash
# Download rotation state
aws s3 cp --endpoint-url http://localhost:9000 s3://bucket/.armor/rotation-state.json - | jq .

# Resume rotation (if interrupted)
curl -X POST http://localhost:9001/admin/key/rotate \
  -H "Content-Type: application/json" \
  -d '{"new_mek": "..."}' | jq .
```

### Offline Decrypt

```bash
# Build decrypt tool
go build -o armor-decrypt ./cmd/armor-decrypt

# Decrypt from B2
armor-decrypt \
  -mek $(cat mek-backup.hex) \
  -input b2://bucket/object-key \
  -output recovered-file.bin

# Decrypt from local file with wrapped DEK
armor-decrypt \
  -mek $(cat mek-backup.hex) \
  -input encrypted.bin \
  -wrapped-dek WWF...base64... \
  -output plaintext.bin
```

---

## References

- [ARMOR README](../README.md) — Project overview and quick start
- [ARMOR Plan](plan/plan.md) — Comprehensive implementation details
- [Admin API Endpoints](../README.md#admin-api) — Full admin API reference
- [Offline Decrypt CLI](../README.md#disaster-recovery--offline-decryption) — Decrypt tool documentation
- [Envelope Encryption Format](plan/plan.md#encryption-scheme) — Cryptographic design
