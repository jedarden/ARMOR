# MEK Rotation Runbook

Operational procedure for rotating the ARMOR Master Encryption Key (MEK) in a
running deployment. Rotation re-wraps every object's wrapped DEK from the old
MEK to a new MEK in place via B2 `CopyObject` (`MetadataDirective=REPLACE`),
and is resumable through `.armor/rotation-state.json`.

This runbook is the **happy path** ordering. For failure recovery and resume
semantics, see [disaster-recovery.md — Key Rotation Failure Recovery](disaster-recovery.md#key-rotation-failure-recovery).

## Required ordering

The four steps **must** happen in this order. The invariant is:
**the MEK used to re-wrap DEKs (the rotate request body) must be byte-identical
to the MEK pods ultimately boot with (the OpenBao value).** If they diverge,
every read of a rotated object fails — DEKs are wrapped to one key while pods
serve another.

```
1. New MEK written to OpenBao
        │
        ▼
2. External Secrets Operator syncs it into the armor-secrets Secret
        │
        ▼
3. POST /admin/key/rotate  (running pod re-wraps DEKs old → new)
        │
        ▼
4. Restart pods so they boot with the new env MEK
```

### Step 1 — Write the new MEK to OpenBao

Generate and store the new MEK in OpenBao at the same path the
`ExternalSecret` reads (the value that lands in `armor-secrets` key
`master-encryption-key`). Nothing in the cluster changes yet — running pods
still hold the old MEK in memory.

```bash
NEW_MEK=$(openssl rand -hex 32)
# Store $NEW_MEK in OpenBao at the ARMOR MEK path.
# Escrow it (see disaster-recovery.md — MEK Backup and Escrow) BEFORE rotating.
```

### Step 2 — Let ESO sync the Secret

External Secrets Operator reconciles the OpenBao value into the
`armor-secrets` Kubernetes Secret (`master-encryption-key`). Confirm the Secret
has the new value. Pods do **not** pick it up yet — env vars are read at pod
start, not live — so all running pods continue to serve with the old MEK.

```bash
# Confirm ESO has synced (value is base64; compare to echo -n $NEW_MEK | base64)
kubectl get secret armor-secrets -o jsonpath='{.data.master-encryption-key}'
```

### Step 3 — Call the rotate endpoint

The running pod's key manager still holds the **old** MEK (its `oldMEK`); the
request body supplies the **new** MEK. Rotation unwraps each DEK with the old
MEK and re-wraps it with the new MEK via in-place `CopyObject`. The body
(ciphertext) and ETag are untouched; only `x-amz-meta-armor-wrapped-dek`
changes. On success the serving pod updates its in-memory default key to the
new MEK and continues serving.

```bash
curl -s -X POST http://localhost:9001/admin/key/rotate \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" \
  -H "Content-Type: application/octet-stream" \
  --data-binary "$NEW_MEK" | jq .
```

> **Auth (bf-5m9nde):** every `/admin/*` route requires
> `Authorization: Bearer <ARMOR_ADMIN_TOKEN>` (constant-time compare). With no
> token configured, the admin API is **disabled fail-closed** — the MEK cannot
> be exported or rotated. Never rotate over an unauthenticated admin surface.

> **The new MEK in the request MUST equal the value written to OpenBao in
> Step 1.** After restart (Step 4), pods read the OpenBao-sourced MEK; the
> re-wrapped DEKs are bound to the Step-3 request MEK. A mismatch makes every
> rotated object unreadable. Rotation via the admin API must never diverge
> from OpenBao.

### Step 4 — Restart pods with the new env MEK

Restart all ARMOR pods so they boot reading the now-synced `armor-secrets`
value (the new MEK). Until every replica has restarted, mixed MEKs may be
serving across replicas — roll promptly and completely.

```bash
kubectl rollout restart deployment/armor -n <namespace>
kubectl rollout status  deployment/armor -n <namespace>
```

## What rotation preserves (and must never drop)

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

## B2 CopyObject size ceiling

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

## Resume after interruption

Rotation checkpoints progress to `.armor/rotation-state.json` every 100 objects
with `status: "in_progress"` and `LastKey`. A pod killed mid-rotation (SIGKILL,
OOMKill, node failure) leaves that `in_progress` state on disk; re-POSTing
`/admin/key/rotate` with the **same** new MEK resumes from `LastKey`, skipping
objects already re-wrapped, and completes idempotently — no object is left
old-wrapped. See [Key Rotation Failure Recovery](disaster-recovery.md#key-rotation-failure-recovery).

> The `new MEK` submitted on resume must match the SHA-256 recorded as
> `new_mek_hash` in the state file, and must match the OpenBao value. A
> different new MEK starts a fresh rotation from the beginning.

## Format Migration

Format migration is a distinct operation from key rotation. While **key rotation**
re-wraps DEKs (changing only the `x-amz-meta-armor-wrapped-dek` metadata),
**format migration** decrypts and re-encrypts the entire object body to upgrade
the encryption format version.

### Why format migration is needed

ARMOR Version1 (`0x01`) has a **CTR keystream reuse vulnerability**: adjacent
blocks in the encrypted stream share keystream, which can allow plaintext
recovery in certain scenarios. Version2 (`0x02`) fixes this by striding the CTR
counter by `blockSize/16` instead of incrementing by 1.

All new objects are encrypted with Version2 by default. However, existing V1
objects remain vulnerable until migrated. Format migration re-encrypts V1 objects
with V2 in place.

### How it differs from key rotation

| Aspect | Key Rotation | Format Migration |
|--------|-------------|------------------|
| **What changes** | Only `x-amz-meta-armor-wrapped-dek` | Entire object body + metadata |
| **Method** | B2 `CopyObject` (metadata replace) | Decrypt → Re-encrypt → PUT |
| **Speed** | Fast (metadata-only copy) | Slow (full re-encryption) |
| **Size limit** | 5 GiB (CopyObject ceiling) | No limit (multipart for >5 MiB) |
| **Use case** | Rotate MEK after compromise | Fix V1 keystream reuse |

### Starting a format migration

```bash
# Dry run (counts objects, doesn't migrate)
curl -s -X POST "http://localhost:9001/admin/format/migrate?dry_run=true&include=v1" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .

# Live migration of V1 objects to V2
curl -s -X POST "http://localhost:9001/admin/format/migrate?dry_run=false&include=v1&concurrency=4" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .
```

**Query parameters:**

- `dry_run=true|false` — Count objects without actually migrating (default: `false`)
- `include=v1|v1,v2` — Comma-separated list of source versions to migrate (default: `v1`)
  - `include=v1` — Migrate only V1 objects (skip V2)
  - `include=v1,v2` — Migrate both V1 and V2 objects (re-encrypt everything)
- `concurrency=N` — Number of parallel migration workers (default: `4`, range: `1-32`)

### Migration behavior

For each object whose version matches `include`:

1. **List every object** (paginated, skipping `.armor/*` internal objects)
2. **Decrypt through the normal read path** (unwraps DEK, decrypts ciphertext)
3. **Re-encrypt with Version2** (current write format, fixes keystream reuse)
4. **PUT to the same key**:
   - **Single PUT** if ≤ 5 MiB (faster, one request)
   - **Multipart upload** if > 5 MiB (handles large objects)
5. **Read back and verify** SHA-256 matches the pre-migration plaintext digest
6. **Advance cursor** and continue

**State persistence:** `.armor/migration-state.json` tracks:
- `started`, `cursor`, `done`, `failed:[{key,reason}]`
- Resumable and idempotent (any instance can resume)
- Failures are recorded and skipped, never retried in a loop

### Checking migration status

```bash
# Get current migration progress
curl -s -X GET "http://localhost:9001/admin/format/migrate" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq .
```

**Response:**
```jsonc
{
  "id": "v1-to-v2-1693100000",
  "target_versions": "v1",
  "write_version": 2,
  "start_time": "2026-08-28T12:34:56Z",
  "last_updated": "2026-08-28T12:45:00Z",
  "status": "in_progress",  // "in_progress", "completed", "failed", "interrupted"
  "total_objects": 1234,
  "processed_objects": 567,
  "skipped_objects": 89,
  "failed_objects": 2,
  "last_key": "data/warehouse/part-00567.parquet",
  "failures": [
    {
      "key": "data/warehouse/corrupted.parquet",
      "reason": "failed to decrypt object: HMAC verification failed",
      "version": 1,
      "skipped": true
    }
  ],
  "dry_run": false
}
```

### Resume after interruption

If a migration is interrupted (pod crash, OOMKill, node failure), re-run the
same `POST /admin/format/migrate` with identical parameters. The migration:

1. Loads `.armor/migration-state.json`
2. Verifies `target_versions` and `write_version` match
3. Resumes from `last_key`, skipping already-processed objects
4. Completes idempotently

**Resumption is automatic** — no manual intervention or state cleanup is needed.

### Failure handling

Format migration records failures and **never retries in a loop**. Failed
objects are:

1. Logged with the reason
2. Added to `failures[]` in the state
3. Skipped on subsequent runs

**Common failures:**
- **HMAC verification failed** — Object corrupted before migration
- **Failed to decrypt object** — Invalid metadata or corrupted ciphertext
- **Failed to re-encrypt object** — Transient backend error (retry manually)
- **Plaintext SHA-256 mismatch** — Verification failure (data integrity issue)

**After migration:** Review `failed_objects` and `failures[]`. Corrupted objects
should be restored from backup and re-migrated individually.

### Metadata and provenance

Migration updates all ARMOR metadata exactly like a normal PUT:

- `x-amz-meta-armor-version` — Set to `2` (Version2)
- `x-amz-meta-armor-block-size` — Preserved from original
- `x-amz-meta-armor-plaintext-size` — Preserved from original
- `x-amz-meta-armor-iv` — New IV for Version2 encryption
- `x-amz-meta-armor-wrapped-dek` — New wrapped DEK (same MEK, new DEK)
- `x-amz-meta-armor-plaintext-sha256` — Verified and preserved
- `x-amz-meta-armor-etag` — Preserved from original

**Manifest:** Updated with new wrapped DEK, version, and metadata.
**Provenance:** Records the migration as an upload event in the chain.

### Performance considerations

Format migration is **slow and I/O intensive**:

- **Large objects** are decrypted and re-encrypted in full
- **Concurrency** of 4 workers balances speed and backend load
- **Dry run first** to count objects and estimate time
- **Run during low-traffic periods** to minimize impact

**Example timing** (4 workers, B2 backend, 64 KB block size):
- 10,000 objects × 1 MB each ≈ 2-3 hours
- 100 objects × 100 GB each ≈ 12-24 hours (multipart, slower)

### After migration: verify V2 adoption

```bash
# List objects by version (requires manifest)
# V1 objects should be zero after successful migration
curl -s "http://localhost:9001/admin/format/migrate" \
  -H "Authorization: Bearer $ARMOR_ADMIN_TOKEN" | jq '.processed_objects'

# Verify no V1 objects remain
# (Use dashboard or backend list with metadata inspection)
```
