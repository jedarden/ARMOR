# Provenance Audit Walker

## Overview

The audit walker verifies the integrity of ARMOR's provenance chain by walking three sources of chain entries in order:

1. **Legacy chain objects** (`.armor/chain/<writer>/*.json`) - Individual chain entries
2. **Chain segments** (`.armor/chain-segments/<writer>/<from>-<to>.jsonl`) - Compacted legacy entries
3. **Delta-embedded entries** (`.armor/manifest/<writer>/delta-*.jsonl`) - Chain entries embedded in manifest delta files

## Chain Head Formats

The audit walker supports two chain head formats:

### Legacy Format (Manifest Disabled)

```json
{
  "writer_id": "armor-instance-1",
  "sequence": 123,
  "chain_hash": "0123456789abcdef...",
  "updated": "2026-08-28T12:00:00Z"
}
```

### Manifest Format (Manifest Enabled)

```json
{
  "delta_file": ".armor/manifest/writer/delta-0000000001.jsonl",
  "sequence": 123,
  "chain_hash": "0123456789abcdef..."
}
```

## Audit Walk Process

For each writer, the audit walker:

1. **Determines the chain head format** by checking for `WriterID` (legacy) or `DeltaFile` (manifest)

2. **For manifest format**, walks delta-embedded entries:
   - Lists all delta files for the writer
   - Parses each JSONL file to extract chain entries from `put` operations
   - Verifies each chain link (hash matching, sequence ordering)

3. **For remaining entries**, walks legacy chain objects:
   - Fetches each `.armor/chain/<writer>/<seq>.json` entry
   - Verifies both `Entry` (uploads) and `KeyEvent` (key operations) objects
   - Checks cryptographic links between entries

4. **Cross-references tracked objects**:
   - Compares tracked objects against all objects in the bucket
   - Reports untracked ARMOR-encrypted objects as potential bypasses

5. **Returns detailed results**:
   - Overall status: `valid`, `invalid`, or `incomplete`
   - Per-writer verification results
   - List of gaps, untracked objects, and errors

## Verification

The audit walker verifies:

- **Cryptographic integrity**: Each entry's `chain_hash` matches the computed hash of its content
- **Chain continuity**: Each entry's `prev_chain_hash` matches the previous entry's `chain_hash`
- **Genesis linkage**: The final `prev_chain_hash` equals the initial hash (`InitialChainHash`)
- **Sequence ordering**: No gaps or duplicate sequence numbers
- **Writer consistency**: All entries have the correct `writer_id`
- **Object tracking**: Cross-reference the current encrypted objects against
  the provenance chain; a missing entry makes the audit `invalid`.

## API Endpoint

### GET /armor/audit

Requires authentication via `ARMOR_ADMIN_TOKEN` bearer token.

Without parameters the audit is unbounded: it walks every writer's chain and
lists every object in the bucket to cross-reference tracked objects. On a
bucket with months of writes that walk far outlasts any HTTP client timeout,
so the endpoint accepts optional bounds. Bounded walks report `status:
"incomplete"` with the progress verified so far when a bound stops them,
rather than hanging until the client gives up.

| Parameter | Meaning |
|---|---|
| `writer=<id>` | Audit only that writer's chain head and entries. Other writers are neither walked nor reported. A writer with no chain head is reported as an incomplete audit, not an empty valid one. |
| `untracked=false` | Skip the full-bucket cross-reference (the `total_objects` / `untracked_objects` fields stay empty). Bounded walks (`writer` or `max_entries` set) skip it by default, because a bounded walk cannot vouch for the whole bucket; pass `untracked=true` to force it on. |
| `max_entries=<n>` | Bound the total chain entries verified across the walk. When the budget is exhausted the walk stops mid-chain, the affected writer is reported with the stopping error, and the overall status is `incomplete`. The delta walk reads manifest files newest-first and stops once the covered sequences suffice for the window, so the bound also caps the number of delta reads. |
| `timeout=<duration>` | Server-side deadline (for example `timeout=25s`). When it fires, backend calls fail, and the walk returns the partial result with status `incomplete` instead of holding the connection open. |

**Bounded request example:**

```
GET /armor/audit?writer=armor-integration-ci&untracked=false&timeout=25s
```

**Response Example:**

```json
{
  "status": "valid",
  "total_entries": 150,
  "total_objects": 145,
  "writers": [
    {
      "writer_id": "armor-instance-1",
      "head_sequence": 100,
      "entries_verified": 100,
      "key_events": 2,
      "valid": true
    },
    {
      "writer_id": "armor-instance-2",
      "head_sequence": 50,
      "entries_verified": 50,
      "key_events": 1,
      "valid": true
    }
  ],
  "untracked_objects": [],
  "gaps": [],
  "errors": []
}
```

## Implementation Details

### Chain Segment Format

Chain segments are compacted legacy chain entries stored as JSONL files:

```json
{"sequence":1,"object_key":"file1.txt","plaintext_sha256":"...","chain_hash":"...","prev_chain_hash":"0000...","timestamp":"...","writer_id":"...","operation":"put"}
{"sequence":2,"object_key":"file2.txt","plaintext_sha256":"...","chain_hash":"...","prev_chain_hash":"...","timestamp":"...","writer_id":"...","operation":"put"}
```

### Delta-Embedded Entry Format

Chain entries embedded in manifest deltas contain minimal information:

```json
{"op":"put","key":"bucket/file.txt","entry":{...},"chain":{"sequence":123,"chain_hash":"...","prev_chain_hash":"..."},"ts":"2026-08-28T12:00:00Z"}
```

### Error Handling

The audit walker handles various failure modes:

- **Incomplete**: Backend failures prevent full audit (listing errors, fetch failures)
- **Invalid**: Cryptographic verification fails (hash mismatch, gap, tampering)
- **Gaps**: Missing sequence numbers indicate deleted entries
- **Untracked objects**: ARMOR-encrypted objects not in any chain

The synchronous `RecordUpload` calls run after the object has been stored.
If it fails, the S3 write still succeeds; ARMOR logs a WARN with the key and
error and increments `armor_provenance_record_failures_total`. A later audit
lists a surviving encrypted object with no prior chain entry under
`untracked_objects` and returns `invalid`. A chain can be cryptographically
continuous yet fail this object-tracking check. The audit compares keys rather
than object versions, so an overwrite can retain an older matching key entry
and still report `valid` after the new append fails. The counter counts all
append failures since process start, including failures for objects later
overwritten or removed; investigate the warning even after a valid audit.

## Testing

The implementation includes comprehensive tests:

- `TestAuditWalksAllThreeSources`: Verifies walking legacy chain, segments, and deltas
- `TestAuditHandlesManifestFormatChainHead`: Tests manifest-format chain head handling
- `TestWalkChainSegments`: Unit tests for chain segment walking
- `TestWalkDeltaEntries`: Unit tests for delta entry extraction
- `TestChainHeadFormatDetection`: Tests format detection logic

## Acceptance Criteria

✅ **Acceptance tests pass**: Tests with a chain spanning all three sources verify clean and detect tampered links in each source

✅ **Backwards compatible**: Existing legacy-format chains continue to work without migration

✅ **Boundary verification**: Chain links are verified across source boundaries (legacy → segment → delta)

✅ **Performance**: Audit completes in reasonable time even with large chains (uses streaming JSONL parsing)

## Future Enhancements

- Chain segment compaction (converts legacy chain to segments)
- Automatic migration from legacy to manifest format
- Chain segment deletion after successful compaction (currently an open question per plan §8.10)
