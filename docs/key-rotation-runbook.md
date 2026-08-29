# MEK Rotation Runbook

Operational procedure for rotating the ARMOR Master Encryption Key (MEK) in a
running deployment using the **MEK key ring** (introduced in version 0.1.1922).

Rotation re-wraps every object's wrapped DEK from the old MEK to a new MEK in
place via B2 `CopyObject` (`MetadataDirective=REPLACE`), and is resumable through
`.armor/rotation-state.json`.

This runbook is the **happy path**. For failure recovery and resume semantics,
see [disaster-recovery.md — Key Rotation Failure Recovery](disaster-recovery.md#key-rotation-failure-recovery).

## What Changed: The Key Ring Approach (v0.1.1922)

**Before (v0.1.1921 and earlier):** Rotation required strict ordering and byte-identical
values. A mismatch between the rotate-request MEK and the OpenBao MEK, or a restart
mid-rotation, made every rotated object permanently unreadable.

**After (v0.1.1922+):** The MEK key ring allows multiple MEKs to coexist safely.
Every replica loads the entire ring, so:

- **No byte-identical value requirement** — the ring makes mismatches impossible
- **Order no longer matters** — all replicas serve correctly throughout rotation
- **Restart mid-rotation is harmless** — replicas always have all valid keys
- **Mixed MEKs serving is safe** — every replica can decrypt objects wrapped with any ring key

The ring is configured via two environment variables:
- `ARMOR_MEK` — The active key (wraps every new DEK)
- `ARMOR_MEK_RING` — Comma-separated list of retired-but-valid hex MEKs

## Rotation Procedure

The rotation procedure has four steps. Unlike the pre-ring runbook, **steps 1-2 can
happen in any order**, and a restart during or after rotation is always safe.

```
1. Generate new MEK and add to OpenBao
   │
   ▼
2. Update OpenBao secret: set ARMOR_MEK to new key,
   append old key to ARMOR_MEK_RING
   │
   ▼
3. External Secrets Operator syncs the updated Secret
   │
   ▼
4. Rolling restart (every replica loads the ring)
   │
   ▼
5. POST /admin/key/rotate (optional, can run in background)
   │
   ▼
6. Retire old key (remove from ARMOR_MEK_RING) when
   GET /admin/key/ring shows 0 objects under old fingerprint
```

### Step 1 — Generate the new MEK

Generate a new 32-byte (64 hex char) MEK. **Never let ARMOR generate its own key** —
a self-minted key is an un-escrowed key. Always generate into OpenBao first:

```bash
# Generate and store directly in OpenBao (value never enters your terminal)
NEW_MEK=$(openssl rand -hex 32)

# Store in OpenBao at the ARMOR MEK path
# Example for OpenBao:
bao kv put secret/<cluster>/<app>/master-encryption-key value="$NEW_MEK"

# Escrow BEFORE making it active (see disaster-recovery.md — MEK Backup and Escrow)
```

The value in your shell (`$NEW_MEK`) is a convenience handle — **the canonical copy
is the OpenBao value, which you should escrow before using.**

### Step 2 — Update OpenBao Secret with Ring Configuration

Update the OpenBao secret to set `ARMOR_MEK` to the new key and append the old key
to `ARMOR_MEK_RING`. The exact secret structure depends on your ExternalSecret
configuration; the example below assumes a flat secret with `master-encryption-key`
as the active key and `mek_ring` as the ring.

```bash
# Get the old (current) MEK from OpenBao before overwriting
OLD_MEK=$(bao kv get -field=master-encryption-key secret/<cluster>/<app>)

# Update the secret with the new active key and the ring
# Example OpenBao secret structure (adjust for your ExternalSecret):
bao kv put secret/<cluster>/<app>/armor \
  master-encryption-key="$NEW_MEK" \
  mek_ring="$OLD_MEK"
```

**Why this order matters:** You retrieve the old MEK *before* overwriting it.
Escrowing the ring (see disaster-recovery.md) captures both keys as a unit.

### Step 3 — Let ESO Sync the Secret

External Secrets Operator reconciles the OpenBao value into the
`armor-secrets` Kubernetes Secret. Confirm the Secret has the new values:

```bash
# Confirm ESO has synced (values are base64)
kubectl get secret armor-secrets -o jsonpath='{.data.master-encryption-key}' | base64 -d
kubectl get secret armor-secrets -o jsonpath='{.data.mek_ring}' | base64 -d

# Both should match your OpenBao values
```

Pods do **not** pick up the new values yet — env vars are read at pod start,
not live — so all running pods continue serving with the old MEK alone.

### Step 4 — Rolling Restart (All Replicas Load the Ring)

Restart all ARMOR pods so they boot reading the updated `armor-secrets` values.
Every replica loads **both** `ARMOR_MEK` (the new active key) and
`ARMOR_MEK_RING` (the old key), so:

- New writes use the new active key
- Reads of objects wrapped with the old key succeed (trial unwrap finds it in the ring)
- Reads of objects wrapped with the new key succeed (direct unwrap with active key)
- A restart mid-rotation is harmless — all replicas always have all valid keys

```bash
kubectl rollout restart deployment/armor -n <namespace>
kubectl rollout status deployment/armor -n <namespace>
```

**Unlike the pre-ring runbook:** You do **not** need to wait for the restart to
complete before proceeding. Once a single replica has the new ring, rotation can
proceed safely.

### Step 5 — Call the Rotate Endpoint (Optional, Background)

The rotation endpoint no longer takes a secret request body. It re-wraps every
object whose wrapped DEK fingerprint does not match the active key's fingerprint,
using the active key already loaded from config.

```bash
# Via kubectl exec (uses the pod's own ARMOR_ADMIN_TOKEN env var)
kubectl exec deploy/armor -n <namespace> -- \
  curl -s -X POST http://localhost:9001/admin/key/rotate \
    -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .

# Via kubectl proxy (plan §8.6 credential-file pattern)
kubectl proxy --port=8001
curl -s -X POST http://127.0.0.1:8001/api/v1/namespaces/<namespace>/services/armor:9001/proxy/admin/key/rotate \
  -H "Authorization: Bearer $(bao kv get -field=admin_token secret/<cluster>/<app>/armor)" | jq .
```

**Auth (bf-5m9nde):** every `/admin/*` route requires
`Authorization: Bearer <ARMOR_ADMIN_TOKEN>` (constant-time compare). With no
token configured, the admin API is **disabled fail-closed** — the MEK cannot be
exported or rotated. Never rotate over an unauthenticated admin surface.

**Why the request body is gone:** The rotate endpoint reads the active key from
`ARMOR_MEK` (already loaded), not from a request body. This eliminates the
pre-ring failure mode where a mismatch between the request-body MEK and the
OpenBao MEK made every rotated object unreadable.

**Rotation is idempotent and resumable.** If interrupted, re-POST to the same
endpoint resumes from `.armor/rotation-state.json`. Already-processed objects
are skipped.

**Optional:** You can skip this step entirely. The ring allows old keys to work
indefinitely, so rotation can be deferred to a convenient maintenance window
or run as a background task.

### Step 6 — Verify and Retire the Old Key

Before removing the old key from the ring, verify that no objects still depend
on it. The admin API provides a histogram of object counts by key fingerprint:

```bash
# Via kubectl exec
kubectl exec deploy/armor -n <namespace> -- \
  curl -s http://localhost:9001/admin/key/ring \
    -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .

# Via kubectl proxy
curl -s http://127.0.0.1:8001/api/v1/namespaces/<namespace>/services/armor:9001/proxy/admin/key/ring \
  -H "Authorization: Bearer $(bao kv get -field=admin_token secret/<cluster>/<app>/armor)" | jq .
```

**Response:**
```json
{
  "active_key_fingerprint": "a1b2c3d4e5f6a7b8",
  "ring_keys": [
    {
      "fingerprint": "f1e2d3c4b5a69788",
      "object_count": 0
    }
  ],
  "total_objects": 12345
}
```

**Retire the old key only when `object_count` is 0.** To retire:

```bash
# Remove the old key from ARMOR_MEK_RING in OpenBao
bao kv patch secret/<cluster>/<app>/armor \
  mek_ring=""  # Empty string if no other retired keys, or remove just the old key

# ESO syncs → rolling restart → done
kubectl rollout restart deployment/armor -n <namespace>
```

`armor check` warns if you attempt to remove a key that still has objects
depending on it. **Retiring a key with non-zero object count makes those objects
permanently unreadable.**

**What about CopyObject exceptions?** Objects above the 5 GiB `CopyObject` ceiling
are enumerated as exceptions and stay on their ring key indefinitely. When you
run a format migration that re-uploads them (full read → PUT cycle), they're
re-wrapped to the active key automatically.

## What Rotation Preserves (and Must Never Drop)

Rotation clones the object's **full raw metadata** and overwrites only
`x-amz-meta-armor-wrapped-dek`. It does **not** rebuild metadata from
`ARMORMetadata.ToMetadata()` (which omits the multipart markers). This is
load-bearing for multipart objects, whose HMAC table lives in a sidecar at
`.armor/hmac/<sha256(key)>` rather than embedded in the byte stream:

- `x-amz-meta-armor-multipart` / `x-amz-meta-armor-part-size` are preserved.
- The `.armor/hmac/*` sidecar is skipped (under `.armor/`) and never touched.
- Plaintext body, ETag, plaintext-sha256, IV, content-type, key-id, and any
  non-ARMOR user metadata all survive the `REPLACE` copy.

Dropping the multipart marker would make the read path treat the object as
single-PUT and look for an embedded HMAC table that isn't there — every
rotated multipart object would 500. That is the [bf-24sxh7](https://github.com/jedarden/ARMOR) failure mode; the test
`TestKeyRotationMixedPrefixPreservesMultipart` guards against its return.

## B2 CopyObject Size Ceiling

B2/S3 `CopyObject` rejects objects above **5 GiB** (same as AWS S3). Rotation
cannot re-wrap such objects via `CopyObject`; it enumerates them as
**exceptions** (`RotationResult.Exceptions` / `ExceptionKeys`) rather than
silently skipping them. An operator must re-wrap each exception with a
multipart copy:

```jsonc
// rotation response
{
  "status": "completed",
  "processed_objects": 4321,
  "exceptions": 2,
  "exception_keys": ["data/warehouse/immense-1.parquet", "data/warehouse/immense-2.parquet"]
}
```

The typed error is `ErrCopyObjectTooLarge`; the ceiling constant is
`B2CopyObjectSizeCeiling` (5 GiB). Objects at exactly 5 GiB are still copyable
(the bound is exclusive).

**With the ring:** Exception objects remain readable indefinitely because they
stay on their ring key. You can defer their migration to a convenient window or
run it as a background task.

## Resume After Interruption

Rotation checkpoints progress to `.armor/rotation-state.json` every 100 objects
with `status: "in_progress"` and `LastKey`. A pod killed mid-rotation (SIGKILL,
OOMKill, node failure) leaves that `in_progress` state on disk; re-POSTing
`/admin/key/rotate` resumes from `LastKey`, skipping objects already re-wrapped,
and completes idempotently — no object is left old-wrapped. See [Key Rotation
Failure Recovery](disaster-recovery.md#key-rotation-failure-recovery).

**With the ring:** Interruption is safer because all replicas load the ring.
Even if rotation stops partway through, all objects remain readable:
- Already-re-wrapped objects use the new active key
- Not-yet-re-wrapped objects use the old key (still in the ring)

## Key Fingerprint Selection

ARMOR identifies MEKs by fingerprint — the first 8 bytes of `SHA-256(MEK)`
encoded as 16 hex characters. The fingerprint is stored **inside** the wrapped
DEK value in B2 metadata:

```
x-amz-meta-armor-wrapped-dek: v2:a1b2c3d4e5f6a7b8:WWF...base64...
                          ^^^^^^^^^^^^^^^^^^^^
                          fingerprint (16 hex chars)
```

**Unwrap logic:**
1. Parse the `v2:<fp16>:<base64>` format
2. If the fingerprint matches the active key, unwrap directly
3. If the fingerprint doesn't match, try each ring key in order
4. If no key in the ring matches, fail with `ErrFingerprintNotFound`
5. Legacy values without the prefix are trial-unwrapped across active + ring

This design works around B2's 10-user-metadata-slot limit (a multipart named-key
object uses all slots for HMAC table references).

**Format Migration**

Format migration re-encrypts objects to the current write format (e.g., V1 → V2).
Unlike key rotation (which only re-wraps DEKs), format migration performs a full
decrypt → re-encrypt cycle, updating the object's ciphertext and encryption format.

### When to migrate formats

Migrate when:
- A new encryption format version is released (e.g., V1 → V2)
- Security vulnerabilities are discovered in the old format
- Compliance requirements mandate stronger encryption

### Migration endpoint

```bash
# Start a migration (dry run by default - counts objects without re-encrypting)
curl -s -X POST "http://localhost:9001/admin/format/migrate?dry_run=true&include=1&concurrency=4" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .

# Actual migration (re-encrypts V1 objects to V2)
curl -s -X POST "http://localhost:9001/admin/format/migrate?dry_run=false&include=1&concurrency=4" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .

# Check migration progress
curl -s -X GET "http://localhost:9001/admin/format/migrate" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .
```

### Using the `armor migrate` CLI

The `armor migrate` command provides a convenient CLI wrapper around the admin API:

```bash
# Dry run (counts objects without re-encrypting)
ARMOR_ADMIN_TOKEN=<token> armor migrate \
  --admin-url http://127.0.0.1:9001 \
  --dry-run \
  --include v1 \
  --concurrency 4

# Actual migration with progress watching
ARMOR_ADMIN_TOKEN=<token> armor migrate \
  --admin-url http://127.0.0.1:9001 \
  --include v1,v2 \
  --concurrency 8 \
  --watch

# Migrate from a pod
kubectl exec deploy/armor -n <namespace> -- armor migrate \
  --admin-url http://localhost:9001 \
  --include v1 \
  --watch
```

**CLI flags:**
- `--admin-url`: Admin API endpoint (required). Example: `http://127.0.0.1:9001`
- `--dry-run`: Verify objects can be migrated without making changes (default: false)
- `--include`: Comma-separated source versions to migrate (default: `v1`). Examples: `v1`, `v1,v2`
- `--concurrency`: Number of concurrent workers (default: server-side default of 4)
- `--watch`: Poll progress until completion and print progress lines (default: false)

**Authentication:**
The admin token must be provided via the `ARMOR_ADMIN_TOKEN` environment variable. The token is **never accepted as a flag** to prevent accidental exposure in shell history or process listings.

**Watch mode:**
With `--watch`, the command polls the migration progress endpoint every 2 seconds and prints:
```
Progress: 10/1000 processed (in_progress)
Progress: 20/1000 processed, 5 skipped (in_progress)
Progress: 30/1000 processed, 5 skipped, 1 failed (in_progress)
Migration completed successfully.
Total: 1000, Processed: 994, Skipped: 5, Failed: 1
```

The command exits with code 1 if any objects failed migration, 0 on success.

### Query parameters

- `dry_run`: `true` (count only) or `false` (actual migration). Default: `false`
- `include`: Comma-separated list of source versions to migrate (e.g., `1` or `1,2`). Default: `1`
- `concurrency`: Number of concurrent workers (1-50). Default: `4`

### Migration state

Migration progress is tracked in `.armor/migration-state.json`:
```jsonc
{
  "id": "format-migration-1693123456",
  "start_time": "2024-08-28T12:34:56Z",
  "last_updated": "2024-08-28T12:45:12Z",
  "status": "in_progress",
  "total_objects": 1234,
  "processed_objects": 567,
  "skipped_objects": 89,
  "failed_objects": 2,
  "last_key": "data/warehouse/object-567.parquet",
  "include_versions": ["1"],
  "current_write_version": 2,
  "dry_run": false,
  "concurrency": 4,
  "failures": [
    {
      "key": "data/corrupted/object.dat",
      "reason": "decryption failed: invalid ciphertext",
      "time": "2024-08-28T12:42:30Z"
    }
  ]
}
```

### Resume after interruption

Like key rotation, format migration is resumable. If interrupted:
- State file persists with `status: "in_progress"` and `last_key`
- Re-POST to `/admin/format/migrate` resumes from `last_key`
- Already-processed objects are skipped (based on key ordering)
- Failed objects are recorded and never retried automatically

### Failure handling

Failed objects are recorded in the `failures` array with:
- Object key
- Failure reason
- Timestamp

Failures are **not retried automatically** to avoid infinite loops. Operators should:
1. Review failure reasons
2. Fix underlying issues (corruption, permission problems)
3. Re-run migration for specific failed objects if needed

### Verification

Migration verifies each re-encrypted object:
1. Calculates SHA-256 of pre-migration plaintext
2. Re-encrypts with current write format
3. Reads back the migrated object
4. Verifies SHA-256 matches pre-migration digest

If verification fails, the object is marked as failed and skipped.

### Limitations

- **Multipart objects**: Not yet supported (complex HMAC sidecar handling required)
- **Large objects**: Objects exceeding multipart threshold (5 MB default) require special handling
- **Concurrent migrations**: Only one migration can run at a time per bucket

### Differences from key rotation

| Aspect | Key Rotation | Format Migration |
|--------|--------------|------------------|
| Operation | Re-wrap DEK only | Full decrypt → re-encrypt |
| Object body | Untouched | Re-encrypted |
| Metadata | Wrapped DEK only | Version, IV, block size, wrapped DEK |
| Copy method | CopyObject (REPLACE) | Full read → PUT cycle |
| Verification | None required | SHA-256 verification required |
| Multipart | Supported (preserves markers) | Not yet supported |
| State file | `.armor/rotation-state.json` | `.armor/migration-state.json` |
