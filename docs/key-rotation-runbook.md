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
