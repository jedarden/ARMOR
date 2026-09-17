# ARMOR Disaster Recovery Runbook

This document covers disaster recovery procedures for ARMOR deployments, including MEK backup/escrow, restore drills, key rotation failure recovery, and data recovery limitations.

## TL;DR Critical Points

1. **The MEK (Master Encryption Key) is the single point of failure** — losing it means losing all data. Never rotate the MEK without first exporting and escrowing the current MEK.
2. **All operational state lives in B2** — a fresh ARMOR instance with the same config (MEK + B2 creds + Cloudflare domain) can recover all data.
3. **The `.armor/` prefix is reserved and must be preserved** — losing sidecar files (`.armor/hmac/*`, `.armor/rotation-state.json`, `.armor/multipart/*.state`) makes corresponding objects unrecoverable.
4. **Per-file DEKs are wrapped by the MEK** — if you lose the MEK, every object's wrapped DEK becomes useless, even though the ciphertext is intact.
5. **The MEK key ring (v0.1.1922+) makes rotation safer** — multiple MEKs can coexist, eliminating the strict ordering and byte-identical value requirements of the old rotation procedure.
6. **The secondary backend ([ADR-006](adr/006-dual-backend-replication.md)) is the only hedge against B2 itself being gone**, and only for deployments that opted in before the loss. Replication is asynchronous and best-effort: a write acknowledged before its secondary copy completes may be absent after failover. See [B2 Account or Bucket Gone: Provider-Outage Recovery](#b2-account-or-bucket-gone-provider-outage-recovery).

## Table of Contents

1. [MEK Backup and Escrow](#mek-backup-and-escrow)
2. [Restore Drill: Recovering from Complete Deployment Loss](#restore-drill-recovering-from-complete-deployment-loss)
3. [Key Rotation Failure Recovery](#key-rotation-failure-recovery)
4. [Multipart Upload Recovery](#multipart-upload-recovery)
5. [B2 Account or Bucket Gone: Provider-Outage Recovery](#b2-account-or-bucket-gone-provider-outage-recovery)
6. [What is NOT Recoverable](#what-is-not-recoverable)
7. [Verification and Testing](#verification-and-testing)

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

### Exporting the MEK Ring (v0.1.1922+)

For deployments using the MEK key ring, you must export **both** the active key and the ring as a unit.

```bash
# Export the active MEK and ring
kubectl exec deploy/armor -n <namespace> -- \
  curl -s "http://localhost:9001/admin/key/ring?confirm=yes" \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq .
```

**Response:**
```json
{
  "active_key_fingerprint": "a1b2c3d4e5f6a7b8",
  "active_mek": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
  "ring_keys": [
    {
      "fingerprint": "f1e2d3c4b5a69788",
      "mek": "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
    }
  ],
  "warning": "This key ring is the single point of failure for all encrypted data. Store it securely and never lose it."
}
```

**Escrow format:** Store the entire JSON response as the escrow unit. The `mek_ring` field
is the canonical source — if you need a flat file format, use:

```json
{
  "active_mek": "0123456789abcdef...",
  "mek_ring": ["fedcba9876543210...", "9876543210abcdef..."]
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

### MEK Rotation Pre-Flight Checklist (v0.1.1922+)

Before rotating the MEK, you MUST:

1. **Export the current MEK and ring** and verify the export completes successfully:
   ```bash
   # Export the ring as a unit
   kubectl exec deploy/armor -n <namespace> -- \
     curl -s "http://localhost:9001/admin/key/ring?confirm=yes" \
       -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq . > /secure/path/mek-ring-backup-$(date +%Y%m%d).json

   # Verify the export is valid JSON
   jq . /secure/path/mek-ring-backup-*.json

   # Verify checksums
   sha256sum /secure/path/mek-ring-backup-*.json
   ```

2. **Escrow the MEK ring as a unit** in your secure location of choice.
   The escrow must include both the active MEK and all ring keys.

3. **Verify the escrow** by retrieving it and comparing checksums:
   ```bash
   # After escrow, retrieve and verify
   escrowed_ring=$(retrieve-from-escrow)
   current_ring=$(kubectl exec deploy/armor -n <namespace> -- \
     curl -s "http://localhost:9001/admin/key/ring?confirm=yes" \
       -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE
   if [ "$escrowed_ring" != "$current_ring" ]; then
     echo "ERROR: Escrowed ring does not match current ring"
     exit 1
   fi
   ```

4. **Verify canary health** to ensure current MEK is valid:
   ```bash
   kubectl exec deploy/armor -n <namespace> -- \
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

### Key Ring Failure Modes: Then vs. Now (v0.1.1922)

The MEK key ring eliminates several critical failure modes that existed in the pre-ring rotation procedure:

| Failure Mode | Pre-Ring (v0.1.1921) | With Ring (v0.1.1922+) |
|--------------|---------------------|----------------------|
| **MEK mismatch between rotate call and OpenBao** | Every rotated object becomes permanently unreadable | **Impossible** — rotate endpoint reads active key from config, not request body |
| **Restart mid-rotation** | Data loss — mixed MEKs serving, some replicas can't decrypt re-wrapped objects | **Non-event** — all replicas load the ring, can decrypt any object |
| **Operator error in rotate request** | Wrong MEK in request body → unreadable objects | **Impossible** — no secret in request body |
| **Interrupted rotation** | Partially re-wrapped objects, old MEK may be lost | **Safe** — old key remains in ring, all objects remain readable |
| **Missed ESO sync** | Pods boot with wrong MEK → unreadable objects | **Impossible** — ESO syncs both active key and ring atomically |

**The key insight:** The ring makes rotation a **read-only operation** from the perspective of
running pods. Adding a key to the ring never makes existing objects unreadable, because
all replicas always have all valid keys loaded.

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
- Escrowed MEK (or MEK ring, for v0.1.1922+)
- B2 credentials
- Cloudflare domain

### Step 1: Retrieve Escrowed MEK (or Ring)

Retrieve the MEK from your escrow location. For v0.1.1922+ deployments, retrieve the entire ring as a unit.

```bash
# From a secret manager (single MEK)
aws secretsmanager get-secret-value --secret-id armor-mek-prod | jq -r '.SecretString'

# From a secret manager (MEK ring - v0.1.1922+)
aws secretsmanager get-secret-value --secret-id armor-mek-ring-prod | jq -r '.SecretString'

# From a file on encrypted media
cryptsetup open /dev/sdX encrypted_backup
cp /mnt/backup/mek.hex ~/mek-recovered.hex
# Or for ring:
cp /mnt/backup/mek-ring.json ~/mek-ring-recovered.json
cryptsetup close encrypted_backup

# From a paper backup
# (Type in the hex value manually)
```

Verify the retrieved MEK is 64 hex characters (32 bytes):

```bash
# Single MEK
mek=$(cat ~/mek-recovered.hex)
if [ ${#mek} -ne 64 ] || ! [[ $mek =~ ^[0-9a-fA-F]{64}$ ]]; then
  echo "ERROR: Invalid MEK format (must be 64 hex chars)"
  exit 1
fi

# MEK ring - verify JSON structure
jq -e '.active_mek and .mek_ring' ~/mek-ring-recovered.json
```

### Step 2: Deploy Fresh ARMOR Instance

Create a new ARMOR deployment with the recovered MEK (or ring) and original B2 credentials.

#### Kubernetes Deployment (Single MEK)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-secrets
type: Opaque
stringData:
  b2-access-key-id: "your-b2-key-id"
  b2-secret-access-key: "your-b2-secret-key"
  master-encryption-key: "REMOVED-NOT-A-SECRET-VALUE" # From escrow
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: armor-config
data:
  ARMOR_B2_REGION: "us-west-002"
  ARMOR_BUCKET: "your-bucket-name"
  ARMOR_CF_DOMAIN: "b2-us-west-002.ardenone.com"
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
        image: ronaldraygun/armor:<version>  # Use tag from VERSION file or Docker Hub
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

#### Kubernetes Deployment (MEK Ring - v0.1.1922+)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: armor-secrets
type: Opaque
stringData:
  b2-access-key-id: "your-b2-key-id"
  b2-secret-access-key: "your-b2-secret-key"
  master-encryption-key: "REMOVED-NOT-A-SECRET-VALUE" # Active key from escrow
  mek_ring: "fedcba9876543210...,9876543210abcdef..." # Ring keys (comma-separated)
---
# Rest of deployment same as above, with ARMOR_MEK_RING env var added:
        - name: ARMOR_MEK_RING
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: mek_ring
```

```bash
kubectl apply -f armor-recovery.yaml
kubectl wait --for=condition=available --timeout=60s deployment/armor
```

#### Docker Deployment (Single MEK)

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e ARMOR_B2_REGION=us-west-002 \
  -e ARMOR_B2_ACCESS_KEY_ID=your-key-id \
  -e ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret \
  -e ARMOR_BUCKET=your-bucket \
  -e ARMOR_CF_DOMAIN=b2-us-west-002.ardenone.com \
  -e ARMOR_MEK=$(cat ~/mek-recovered.hex) \
  -e ARMOR_AUTH_ACCESS_KEY=my-access-key \
  -e ARMOR_AUTH_SECRET_KEY=my-secret-key \
  ronaldraygun/armor:<version>  # Use tag from VERSION file or Docker Hub
```

#### Docker Deployment (MEK Ring - v0.1.1922+)

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e ARMOR_B2_REGION=us-west-002 \
  -e ARMOR_B2_ACCESS_KEY_ID=your-key-id \
  -e ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret \
  -e ARMOR_BUCKET=your-bucket \
  -e ARMOR_CF_DOMAIN=b2-us-west-002.ardenone.com \
  -e ARMOR_MEK=$(jq -r '.active_mek' ~/mek-ring-recovered.json) \
  -e ARMOR_MEK_RING=$(jq -r '.mek_ring | join(",")' ~/mek-ring-recovered.json) \
  -e ARMOR_AUTH_ACCESS_KEY=my-access-key \
  -e ARMOR_AUTH_SECRET_KEY=my-secret-key \
  ronaldraygun/armor:<version>
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

For a more thorough validation, use the offline decrypt CLI to verify a few objects directly from B2.

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

## Offline Decryption Without ARMOR (verified 2026-08-08)

The fastest recovery path does **not** involve redeploying ARMOR. `armor decrypt`
reads objects straight from B2 and decrypts them locally, needing only the MEK,
B2 credentials, and the binary. Use this when you need data back now, or when
ARMOR itself is what is broken.

> The `armor decrypt` examples elsewhere in this runbook show only `-mek` and
> `-input`. **That is not sufficient** — the tool also requires four B2
> environment variables, and it ignores the standard `AWS_*` names. Following
> those examples literally fails with
> `B2 credentials not set: set ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY`.

### Procedure (executed end to end, not theoretical)

```bash
# MEK — take it from escrow, NOT from the cluster you are recovering.
export ARMOR_MEK=$(vault kv get -field=MASTER_ENCRYPTION_KEY \
                     secret/rs-manager/<cluster>/armor)

# B2 credentials. These names are mandatory; AWS_* is ignored.
export ARMOR_B2_ACCESS_KEY_ID=<b2 key id>
export ARMOR_B2_SECRET_ACCESS_KEY=<b2 application key>
export ARMOR_B2_REGION=us-west-002
export ARMOR_B2_ENDPOINT=https://s3.us-west-002.backblazeb2.com

# Optional: route ciphertext reads through the B2 download URL proxied by
# Cloudflare (free Bandwidth Alliance egress). Keep the S3 endpoint above:
# armor decrypt still uses it for authenticated metadata requests.
export ARMOR_CF_DOMAIN=b2-us-west-002.ardenone.com

armor decrypt -v \
  -input  b2://<bucket>/<key> \
  -output /tmp/recovered.bin
```

**MEK Ring support (v0.1.1922+):** For deployments using the key ring, provide the ring
via the `--mek-ring` flag or a JSON escrow file:

```bash
# From JSON escrow file
armor decrypt -v \
  -input  b2://<bucket>/<key> \
  -output /tmp/recovered.bin \
  --mek-ring-file ~/mek-ring-recovered.json

# From command-line (comma-separated)
armor decrypt -v \
  -input  b2://<bucket>/<key> \
  -output /tmp/recovered.bin \
  --mek-ring fedcba9876543210...,9876543210abcdef...
```

**If you do not know the region or endpoint**, B2 will tell you — they are not
recorded in the credential store, which holds only `key_id` and
`application_key`:

```bash
curl -s -u "$KEY_ID:$APP_KEY" \
  https://api.backblazeb2.com/b2api/v3/b2_authorize_account \
  | jq -r '.apiInfo.storageApi | "endpoint=\(.s3ApiUrl)  bucket=\(.bucketName)"'
```

### Expected output

```
Loaded MEK from env
Reading from B2: <bucket>/<key>
ARMOR version: 1, Block size: 65536, Plaintext size: 12225178
Successfully unwrapped DEK
Read 12225178 encrypted bytes and 187 HMAC entries
Verified plaintext SHA-256: 5328674abb1558b8…
Decryption successful
```

Every object carries its own wrapped DEK, IV, block size, plaintext size and
plaintext SHA-256 as S3 object metadata, plus per-block HMACs — so decryption is
self-verifying. No external manifest is required for a single object.

### V3 Format Support

ARMOR v3 introduces several enhancements to the encryption format while maintaining
backward compatibility with v1 and v2:

**V3 Single-PUT Format:**
- Block table trailer replaces inline HMAC table (per-block HMACs + ciphertext lengths)
- Per-block compression with zstd (compression flag in block table entry)
- Counter format: `IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock)`
- Header byte `0x03` identifies v3 objects

**V3 Multipart Format:**
- Gzip-compressed JSON sidecar at `.armor/hmac/<sha256(key)>`
- Per-part block tables with independent part counters (no uniform part size constraint)
- Same per-block compression as single-PUT

**Automatic format detection:**
`armor decrypt` automatically detects v3 format from object metadata and handles:
- Block table trailer parsing for single-PUT objects
- Gzip-compressed sidecar loading for multipart objects  
- Per-block decompression (transparent to the caller)
- Part-aware counter construction (multipart only)

No special flags are required — v3 decryption works the same as v1/v2:

```bash
# V3 objects decrypt identically to v1/v2
armor decrypt -v \
  -input  b2://<bucket>/<v3-key> \
  -output /tmp/recovered-v3.bin
```

**Expected output for v3 objects:**
```
ARMOR version: 3, Block size: 65536, Plaintext size: 12225178
V3 envelope: 187 blocks, ciphertext size 12225178, trailer 6732 bytes
Decrypted with block table (187 blocks)
Verified plaintext SHA-256: 5328674abb1558b8…
```

**Local file recovery with v3 sidecars:**
For v3 multipart objects downloaded locally, use the `-sidecar` flag with the
gzip-compressed sidecar file:

```bash
armor decrypt -v \
  -input  /tmp/downloaded-object.bin \
  -sidecar /tmp/object.hmac.json.gz \
  -iv <hex-iv-from-metadata> \
  -output /tmp/recovered.bin
```

The sidecar file is the gzip-compressed JSON stored at `.armor/hmac/<sha256(key)>`
in B2. Download it separately and provide it to `armor decrypt`.

### Verification performed

Run on a host outside the protected cluster, with the MEK read from a
*different* cluster's secret store:

- 12,225,178-byte object decrypted in **3 seconds**, plaintext SHA-256 verified
- Output was a valid gzip tarball; `git fsck --connectivity-only` passed
- Recovered repository HEAD matched live production **exactly** (`af0e37cbac8d`)
- MEK in escrow confirmed identical to the live cluster's by SHA-256 comparison

### Two network paths — know which one you are on

| Path | Throughput (measured, 154 MB object) | Egress cost |
|---|---|---|
| Cloudflare (`ARMOR_CF_DOMAIN`, e.g. `b2-us-west-002.ardenone.com`) | **~7.9 MB/s** | **free** (Bandwidth Alliance) |
| Direct B2 S3 (`ARMOR_B2_ENDPOINT`) | **~38 MB/s** | billed |

`armor decrypt` uses the same B2 range-read path as the ARMOR service. Set
`ARMOR_CF_DOMAIN` to fetch ciphertext through Cloudflare's native B2 download
URL (`/file/<bucket>/<key>`) at free Bandwidth Alliance egress. Leave it empty
to use the direct B2 S3 path instead, which is faster but billed. The S3
endpoint and credentials remain required in both cases for authenticated
metadata reads; the Cloudflare domain is not an S3 endpoint and is deliberately
used only for object range reads.

### Why the service was slower than either raw path

The measurements above predate [ADR-013](adr/013-read-throughput-unpipelined-block-fetches.md).
Before that fix, the CF read path issued a **ranged GET per 64 KiB block**
(`bytes=<off>-<off+len-1>`) serially. Against a cross-country round trip
(measured: 74 ms TCP connect to `us-west-002`) that capped throughput near
`65536 / RTT` — around 0.9 MB/s serial, ~1.5 MB/s observed.

`GetRangeWithHeaders` now pipelines those requests with a bounded worker pool,
preserves block order, and fails the whole read if any block fails or is
truncated. `ARMOR_READ_CONCURRENCY` controls the maximum number of ranged GETs
in flight and defaults to 16. `armor decrypt` remains faster in a direct-B2
configuration because it bypasses the Cloudflare path, at billed egress.

## Key Rotation Failure Recovery

> For the planned rotation procedure with the MEK key ring (v0.1.1922+), see [key-rotation-runbook.md](key-rotation-runbook.md). This section covers recovering from an interrupted or failed rotation.

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

### Rotation Failure Modes (v0.1.1922+)

| Failure Mode | Detection | Recovery Action |
|--------------|-----------|-----------------|
| ARMOR pod crashed during rotation | `.armor/rotation-state.json` exists with `status: in_progress` | Restart rotation via `/admin/key/rotate` — it will resume automatically. **Safe** — all replicas load the ring, so all objects remain readable. |
| B2 API rate limit | Rotation endpoint returns error with retry-after | Wait and retry — rotation is idempotent, already-processed objects are skipped |
| Network timeout on CopyObject | Failed objects listed in rotation state | Retry rotation — only failed objects are reprocessed |
| Old MEK lost before rotation completed | Old MEK not in escrow | **Data loss for objects not yet re-wrapped** — restore old MEK from escrow. **With ring:** safer because old key remains in ring until explicitly retired. |

### Resuming an Interrupted Rotation

To resume rotation, simply POST to the `/admin/key/rotate` endpoint again:

```bash
# Rotation will automatically resume from .armor/rotation-state.json
kubectl exec deploy/armor -n <namespace> -- \
  curl -s -X POST http://localhost:9001/admin/key/rotate \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq .
```

ARMOR reads `.armor/rotation-state.json`, determines which objects were successfully processed, and continues from where it left off.

**With the ring:** Resuming is safer because all replicas load the ring. Even if rotation
stopped partway through, all objects remain readable:
- Already-re-wrapped objects use the new active key
- Not-yet-re-wrapped objects use the old key (still in the ring)

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

## B2 Account or Bucket Gone: Provider-Outage Recovery

Every procedure above this one recovers ARMOR **from** B2. This one covers the
failure where B2 itself is what is lost: the account is suspended (billing or
ToS action), a regional outage outlasts your tolerance, or the bucket is
destroyed (accidental deletion, lifecycle-rule misfire). The canary, the
provenance chain, and restore verification
([ADR-004](adr/004-continuous-restore-verification.md)) cannot detect or
survive this class of failure — they all read their known-good answer from the
same provider that just went away.

[ADR-006](adr/006-dual-backend-replication.md) added an opt-in secondary
backend as insurance for exactly this scenario, and deliberately deferred the
failover steps to this section. They apply **only** to deployments that
configured `ARMOR_SECONDARY_BACKEND` before the loss; for everything else,
"B2 account or bucket gone" is total loss (see
[What is NOT Recoverable](#what-is-not-recoverable)).

### The replication model and its data-loss window

The secondary is a **best-effort, non-blocking mirror**; ADR-006 explicitly
rejected making it transactional. Internalize these properties before you
need them:

- **A client write is acknowledged when the primary write succeeds.** The copy
  to the secondary is enqueued and performed in the background.
  **Any write acknowledged before its secondary copy completed may be absent
  after failover.** This is the documented consistency tradeoff (ADR-006,
  Decision #6) — the secondary is not zero-RPO and must not be presented as
  such.
- **The queue is in memory** (capacity 4096) and is lost when the ARMOR
  process stops. Objects still queued at that moment were acknowledged to
  clients but never copied.
- **A full queue drops items rather than blocking writes.** Every drop is
  counted in `armor_replication_dropped_total`; each count is an acknowledged
  write that never reached the mirror.
- **Copy failures retry while the worker lives.** Transient errors (timeouts,
  rate limits, 5xx) retry indefinitely; permanent errors (not found, access
  denied, authentication failure) are dropped and counted in
  `armor_replication_errors_total`.
- **Only completed data objects are mirrored.** The reserved `.armor/`
  namespace — manifest snapshot, provenance chain, rotation state, multipart
  upload state, multipart HMAC sidecars — is internal primary state and is
  never copied. A multipart object's ciphertext is mirrored after
  `CompleteMultipartUpload`, but without its `.armor/hmac/` sidecar it cannot
  be decrypted from the mirror. Single-PUT objects carry their HMAC/block
  table inline and decrypt fine.
- **Failover is never automatic.** Replication does not change the read path,
  and read traffic does not move on its own. Promotion is the manual procedure
  below.

### Prerequisites (arrange these before the outage)

1. **Opt in, and pick an independent failure domain.** The filesystem
   secondary on a different host/network than any B2-dependent component is
   the ADR-006-recommended first choice. A B2 secondary is supported but
   shares fate with its account — point it at a **different account**
   (ideally a different provider), or it does not survive an account-level
   suspension:

   ```bash
   # Filesystem secondary — inline form
   ARMOR_SECONDARY_BACKEND=filesystem:/offsite/armor
   # …or the split form, useful when the path is mounted/injected separately
   ARMOR_SECONDARY_BACKEND=filesystem
   ARMOR_SECONDARY_BACKEND_PATH=/offsite/armor
   ```

   For a B2 secondary use `ARMOR_SECONDARY_BACKEND=b2` with
   `ARMOR_SECONDARY_B2_ENDPOINT`, `ARMOR_SECONDARY_B2_KEY_ID`,
   `ARMOR_SECONDARY_B2_KEY`, and `ARMOR_SECONDARY_B2_BUCKET`. Those credential
   values must come from the deployment secret store and are never printed in
   redacted configuration.

2. **Match the prefix.** Replication copies keys verbatim and filters the
   target with the primary's `ARMOR_PREFIX`. A mismatched value does not
   error — it silently mis-filters listings. Configure the secondary's prefix
   to exactly the primary's.

3. **Make the volume real.** The secondary initializer fails ARMOR startup if
   the configured path does not exist or is not a directory — a deliberately
   loud check that the backing volume actually mounted. Do not "fix" that
   startup failure by pre-creating an empty directory; fix the mount.

4. **Monitor replication health continuously** — see
   [metrics](metrics.md) and [dashboard](dashboard.md):

   | Signal | Healthy | Investigate/page when |
   |---|---|---|
   | `armor_replication_queue_depth` | ~0 | sustained growth (example alert: `> 1000`) |
   | `armor_replication_lag_seconds` | ~0 | rising — the oldest unreplicated object is aging |
   | `armor_replication_dropped_total` | flat | **any increase** — acknowledged writes with no mirror copy |
   | `armor_replication_errors_total` | flat | any increase — copies failing permanently |
   | `/armor/canary` `secondary_healthy` | `healthy` | repeated `secondary_consecutive_fails` |

   Note: the canary's secondary check re-reads the canary object from the
   primary to compare envelopes, so when the primary is unreachable
   `secondary_healthy` fails too — that is expected and does not by itself
   mean the secondary copy is damaged.

5. **Drill the procedure** (see
   [Verification and Testing](#verification-and-testing)) so these steps are
   familiar before they are urgent.

### Route A — Promote the replica and serve reads from it

#### Step 1: Freeze writes and capture the replication state

Stop client writes first: every acknowledged write that has not yet
replicated widens the loss window. Then record the replication state
**before restarting anything** — the queue is in memory and a restart
destroys it.

```bash
# Replication state at the moment of failure (capture BEFORE any restart)
curl -s http://localhost:9001/metrics | grep '^armor_replication'
curl -s http://localhost:9001/armor/canary | jq '{
  secondary_healthy,
  secondary_replication_lag_ms,
  secondary_queue_depth,
  secondary_last_error
}'
```

Reading it:

- last `armor_replication_queue_depth` — objects acknowledged-but-uncopied at
  that instant; the tightest upper bound on your loss window;
- `armor_replication_dropped_total` (delta over the deployment's life) —
  acknowledged writes that were never queued at all;
- `armor_replication_lag_seconds` — age of the oldest unreplicated object.

If ARMOR must restart during triage, everything still in the queue at that
moment is gone. Capture the numbers first.

#### Step 2: Verify the secondary copy before trusting it

```bash
# Mount the replica volume read-only — it is now the only copy.
mount -o ro <device> /offsite/armor   # or your volume manager's equivalent

# Layout is <root>/<bucket>/<stored-key> with .metadata sidecars.
replica_root=/offsite/armor
find "$replica_root" -name '*.metadata' | wc -l   # ≈ mirrored object count
```

Compare the count against the primary's last known object inventory (your
records, or the manifest snapshot if preserved) to size the gap. If the
newest mirrored objects predate your last acknowledged writes by more than
the lag recorded in Step 1, assume everything in between is lost.

#### Step 3: Point a fresh ARMOR instance at the replica

The filesystem mirror **is** a valid filesystem-primary tree:
`ARMOR_BACKEND=filesystem` selects the filesystem backend as a supported
primary, with the B2 credentials and Cloudflare domain optional in that mode
(plan.md §8.5). Start a fresh instance against it:

```bash
export ARMOR_BACKEND=filesystem
export ARMOR_FS_PATH=/offsite/armor   # the replica ROOT; objects live at <path>/<bucket>/<key>
export ARMOR_PREFIX=<primary's ARMOR_PREFIX>   # must match the mirrored keys
export ARMOR_MEK=$(cat /secure/escrow/armor-mek.hex)   # from escrow, never the failed deployment
# plus ARMOR_MEK_RING / ARMOR_AUTH_FILE (or auth env) as in the restore drill above
```

Validation and caveats for this instance:

- **Watch for the silent empty-directory trap.** Unlike the secondary
  initializer, a filesystem *primary* creates its base directory on demand.
  If the volume did not mount, ARMOR starts happily against an empty path and
  serves zero objects. Confirm the S3 list count matches the `find` inventory
  from Step 2 before anything else.
- Multipart objects fail to read — their `.armor/hmac/` sidecars were never
  mirrored. Single-PUT objects read normally.
- There is no Cloudflare read path in filesystem mode; reads are served from
  the mounted volume.
- `/admin/key/verify` and `/armor/canary` prove the MEK and the
  encrypt/decrypt pipeline. The canary **self-heals** — it writes a fresh
  canary object on the promoted instance — so a green canary does **not**
  prove the mirrored data is readable. That is Step 4's job.
- Writes on the promoted instance land only on this volume: single-copy
  durability until Step 5 is done.

#### Step 4: Validate reads (restore validation)

```bash
export AWS_ACCESS_KEY_ID=<access key>     # from ARMOR_AUTH_FILE / credentials
export AWS_SECRET_ACCESS_KEY=<secret key>
export AWS_ENDPOINT_URL=http://localhost:9000

# Count and compare with the Step 2 inventory
aws s3 ls --endpoint-url $AWS_ENDPOINT_URL s3://<bucket>/ | wc -l

# Spot-read a sample across size and age ranges — GET decrypts transparently
aws s3 cp --endpoint-url $AWS_ENDPOINT_URL s3://<bucket>/<key> /tmp/verify.bin

# Application-level validity, then checksum
duckdb -c "SELECT COUNT(*) FROM '/tmp/verify.parquet';"
sha256sum /tmp/verify.bin
```

For strong per-object validation, decrypt straight from the replica file
(Route B below) and compare the tool's `Verified plaintext SHA-256` output
against `x-amz-meta-armor-plaintext-sha` in the object's `.metadata` sidecar.
Every object that fails here joins the loss list from Step 1 — validate
before declaring the failover complete.

#### Step 5: Return to a durable primary

The promoted instance is insurance made permanent — do not leave it that way.

1. Provision the replacement primary (new B2 account/bucket, or another
   provider) and verify it with the
   [restore drill](#restore-drill-recovering-from-complete-deployment-loss)
   and the restore verifier
   ([ADR-004](adr/004-continuous-restore-verification.md),
   [deployment guide](restore-verifier-deployment-guide.md)).
2. Reprotect new writes immediately: configure the replacement as the
   **secondary** on the promoted instance (`ARMOR_SECONDARY_BACKEND=b2 …`).
   New writes replicate out as they land. **There is no bulk backfill** —
   replication is write-time only.
3. Reprotect pre-outage objects by reading each through the promoted instance
   and re-uploading it: a fresh PUT creates fresh ciphertext and metadata,
   which then replicate to the replacement.
4. When the replacement holds everything and has passed verification, migrate
   the primary back to it as a new deployment change. The manifest snapshot
   and the provenance/audit history do not survive the original provider's
   loss — they restart from the promoted instance's first write.

### Route B — Offline decryption from the replica

When you need specific objects rather than a promoted server, decrypt
directly from the replica tree. `armor decrypt` reads local files, taking the
wrapped DEK from the `.metadata` sidecar.

#### What the filesystem contains

The filesystem backend stores each replicated object at
`<root>/<bucket>/<stored-key>` and its metadata at the same path with a
`.metadata` suffix. `stored-key` includes `ARMOR_PREFIX` when that option is
enabled. The metadata JSON contains the original ARMOR headers, including the
wrapped DEK, IV, plaintext size, and multipart marker.

Only completed data-object keys are queued by the S3 handlers. The reserved
`.armor/` namespace (manifest, provenance, rotation state, multipart upload
state, and HMAC sidecars) is internal primary-backend state and is not copied
or exposed through public listings. Consequently, single-PUT envelope files
can be decrypted from this mirror, but a multipart ciphertext alone is not
recoverable: multipart decryption also requires its `.armor/hmac/` sidecar.
Keep a separate backup of ARMOR internal state if multipart recovery from the
secondary is a requirement.

#### Recover a single-PUT object

1. Preserve the secondary volume and mount it read-only if possible. Do not
   delete or rename the object while investigating it.
2. Resolve the bucket and stored key. With a prefix, use the prefixed path
   exactly as it appears below the configured filesystem root:

   ```bash
   replica_root=/offsite/armor
   bucket=<primary-bucket>
   stored_key=<stored-key-including-any-prefix>
   ciphertext="$replica_root/$bucket/$stored_key"
   metadata="$ciphertext.metadata"
   test -f "$ciphertext" && test -f "$metadata"
   ```

3. Read the wrapped DEK from the metadata sidecar. The value may be a plain
   base64 string or the newer `v2:<mek-fingerprint>:<base64>` form; pass only
   the final base64 component to the local decrypt command:

   ```bash
   wrapped_dek=$(jq -r '.Metadata["x-amz-meta-armor-wrapped-dek"]' "$metadata")
   case "$wrapped_dek" in
     v2:*:*) wrapped_dek="${wrapped_dek##*:}" ;;
   esac
   ```

4. Use the escrowed MEK, never a value copied from the failed deployment, to
   decrypt the copied envelope. The envelope carries its own IV and inline
   HMAC/block table, so no secondary B2 credentials are needed:

   ```bash
   armor decrypt \
     -mek-file /secure/escrow/armor-mek.hex \
     -input "$ciphertext" \
     -wrapped-dek "$wrapped_dek" \
     -output /secure/recovered/<object-name>
   ```

5. Verify the recovered plaintext against `x-amz-meta-armor-plaintext-sha` in
   the `.metadata` sidecar and the application-level checks appropriate to
   the artifact (for example SQLite, Parquet, or tar/gzip validation). Repeat
   for each required object. To resume service, provision a replacement
   primary and upload the recovered plaintext through ARMOR; this creates
   fresh encrypted objects and does not recreate the lost B2-side
   manifest/provenance history.

### What the replica cannot recover

| Data | Why it is lost |
|---|---|
| Writes acknowledged before their secondary copy completed | ADR-006's asynchronous window — the ack reflects the primary only |
| Objects still queued when the ARMOR process stopped | The queue is in memory and dies with the process |
| Items dropped on a full queue (`armor_replication_dropped_total`) | Never queued; the primary remained authoritative |
| Multipart objects | Ciphertext mirrors, `.armor/hmac/` sidecars do not — undecryptable without a separate ARMOR internal-state backup |
| Manifest snapshot, provenance chain, audit history | The `.armor/` internal namespace is never mirrored |
| Buckets that never opted in | Replication is opt-in per deployment |

This procedure is intentionally manual: the secondary is insurance against
provider loss, not an automatic read replica. If the filesystem is also gone,
or the object was acknowledged during the asynchronous replication window,
the secondary cannot provide recovery.

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
- Configure the ADR-006 secondary backend — see [B2 Account or Bucket Gone: Provider-Outage Recovery](#b2-account-or-bucket-gone-provider-outage-recovery) for the only recovery path that survives losing the provider entirely

---

## Verification and Testing

### Testing MEK Escrow Retrieval

Regularly test that you can retrieve and use the escrowed MEK (or MEK ring, for v0.1.1922+):

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

# 2. Start rotation (v0.1.1922+ - no request body needed)
kubectl exec deploy/armor -- \
  curl -X POST http://localhost:9001/admin/key/rotate \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE &

# 3. Kill ARMOR after 5 seconds (simulating crash)
sleep 5
kubectl delete pod armor-xxx-yyy

# 4. Restart ARMOR and verify rotation resumes
kubectl wait --for=condition=available deployment/armor
kubectl exec deploy/armor -- \
  curl -s -X POST http://localhost:9001/admin/key/rotate \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq .

# 5. Verify all files still decrypt (they should - ring makes this safe)
for i in {1..100}; do
  aws s3 cp --endpoint-url http://localhost:9000 s3://test-bucket/file-$i.bin - | wc -c
done
```

---

## Appendix: Quick Reference Commands

### Export and Escrow MEK (Single MEK)

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

### Export and Escrow MEK Ring (v0.1.1922+)

```bash
# Export ring (includes active key and all ring keys)
kubectl exec deploy/armor -n <namespace> -- \
  curl -s "http://localhost:9001/admin/key/ring?confirm=yes" \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq . > mek-ring-backup-$(date +%Y%m%d).json

# Verify export
jq . mek-ring-backup-*.json
sha256sum mek-ring-backup-*.json

# Escrow (example: AWS Secrets Manager)
aws secretsmanager create-secret \
  --name armor-mek-ring-prod-$(date +%Y%m%d) \
  --secret-string file://mek-ring-backup-$(date +%Y%m%d).json
```

### Verify MEK

```bash
# Via admin API
kubectl exec deploy/armor -- curl -s http://localhost:9001/admin/key/verify | jq .

# Via canary status
kubectl exec deploy/armor -- curl -s http://localhost:9001/armor/canary | jq .
```

### Check Rotation Status (v0.1.1922+)

```bash
# Get ring status (includes object count histogram by key fingerprint)
kubectl exec deploy/armor -n <namespace> -- \
  curl -s http://localhost:9001/admin/key/ring \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq .

# Resume rotation (if interrupted)
kubectl exec deploy/armor -n <namespace> -- \
  curl -s -X POST http://localhost:9001/admin/key/rotate \
    -H "Authorization: Bearer REMOVED-NOT-A-SECRET-VALUE | jq .
```

### Offline Decrypt (Single MEK)

```bash
# Decrypt from B2
# B2 reads REQUIRE these four; the tool ignores AWS_* names entirely.
export ARMOR_B2_ACCESS_KEY_ID=<b2 key id>
export ARMOR_B2_SECRET_ACCESS_KEY=<b2 application key>
export ARMOR_B2_REGION=us-west-002
export ARMOR_B2_ENDPOINT=https://s3.us-west-002.backblazeb2.com

armor decrypt \
  -mek $(cat mek-backup.hex) \
  -input b2://bucket/object-key \
  -output recovered-file.bin
```

### Offline Decrypt (MEK Ring - v0.1.1922+)

```bash
# Decrypt from B2 with ring support
export ARMOR_B2_ACCESS_KEY_ID=<b2 key id>
export ARMOR_B2_SECRET_ACCESS_KEY=<b2 application key>
export ARMOR_B2_REGION=us-west-002
export ARMOR_B2_ENDPOINT=https://s3.us-west-002.backblazeb2.com

# From JSON escrow file
armor decrypt \
  -input b2://bucket/object-key \
  -output recovered-file.bin \
  --mek-ring-file ~/mek-ring-backup-20260829.json

# From command-line (comma-separated ring keys)
armor decrypt \
  -input b2://bucket/object-key \
  -output recovered-file.bin \
  --mek-ring fedcba9876543210...,9876543210abcdef...
```

---

## References

- [ARMOR README](../README.md) — Project overview and quick start
- [ARMOR Plan](plan/plan.md) — Comprehensive implementation details (see §8.13 for MEK key ring)
- [Admin API Endpoints](../README.md#admin-api) — Full admin API reference
- [Offline Decrypt CLI](../README.md#disaster-recovery--offline-decryption) — Decrypt tool documentation
- [Envelope Encryption Format](plan/plan.md#encryption-scheme) — Cryptographic design
- [Key Rotation Runbook](key-rotation-runbook.md) — Rotation procedure with MEK key ring (v0.1.1922+)
- [ADR-006: Dual-Backend Async Replication](adr/006-dual-backend-replication.md) — Design and tradeoffs behind the secondary backend and this section's recovery procedure
- [ADR-004: Continuous Restore Verification](adr/004-continuous-restore-verification.md) — Restore verifier (proves the primary's contents; does not cover provider loss)
- [Replication Metrics](metrics.md) — `armor_replication_*` queue depth, lag, drop, and error counters
- [Dashboard](dashboard.md) — Replication queue depth and enqueue panels
