# Barman-Cloud-Backup InvalidPartSize Root Cause Analysis

## Executive Summary

Barman-cloud-backup 3.19.1 (the version confirmed live in iad-ci pods) produces multipart upload parts that are not multiples of ARMOR's 65536-byte encryption block size, violating ARMOR's documented uniform-part-size contract (ADR-005). This is not a configuration issue or a bug in ARMOR — it is a fundamental incompatibility between barman's upload logic and ARMOR's encryption requirements.

**Root Cause:** Barman's `CloudTarUploader.write()` method flushes upload parts when the buffer size exceeds `chunk_size`, not when it equals `chunk_size`. This produces parts of size `chunk_size + N` where N depends on the last write operation. For uncompressed tar streams (the post-"fix" configuration on iad-ci), N is a multiple of 512 bytes (POSIX tar block size). For compressed streams, N is unpredictable. ARMOR requires N to be a multiple of 65536 bytes. 512 ≠ 65536, and no client-side `chunk_size` value can fix this mismatch.

## Evidence

### 1. Code Analysis: Barman 3.19.1 Upload Logic

From `/tmp/barman-3.19.1/barman/cloud.py`:

```python
class CloudTarUploader(object):
    def write(self, buf):
        # THE KEY LINE: flush when buffer.tell() > chunk_size
        if self.buffer and self.buffer.tell() > self.chunk_size:
            self.flush()
        if not self.buffer:
            self.buffer = self._buffer()
        # ... write to buffer
        if self.compressor:
            compressed_buf = self.compressor.add_chunk(buf)
            self.buffer.write(compressed_buf)
        else:
            self.buffer.write(buf)
```

The flush condition is `buffer.tell() > chunk_size`. This means:
- Buffer accumulates data up to chunk_size
- The next write pushes buffer over chunk_size
- Buffer is flushed at size > chunk_size
- Part size = chunk_size + size_of_last_write

**The last write size is not controlled by chunk_size.** It depends on:
- Tarfile's write granularity (512-byte blocks for POSIX tar)
- Compressor output size (if compression enabled)
- Write timing from the PostgreSQL backup stream

### 2. Code Analysis: ARMOR's Uniform-Part-Size Contract

From `internal/server/handlers/handlers.go:2194`:

```go
if plaintextSize > 0 && plaintextSize%int64(state.BlockSize) != 0 {
    h.writeError(w, "InvalidPartSize",
        fmt.Sprintf("Part size %d is not a multiple of the block size (%d bytes). ARMOR's uniform-part-size contract (ADR-005) requires block-aligned parts. Use a part size that's a multiple of %d (e.g., 5,242,880 for 5MiB, 16,777,216 for 16MiB).", plaintextSize, state.BlockSize, state.BlockSize), 400)
    return
}
```

ARMOR requires `part_size % 65536 == 0`. This is non-negotiable because:
- Part encryption offsets are calculated as `(part_number - 1) × part_size / block_size`
- Misaligned parts would produce incorrect HMAC indices
- This would recreate the corruption bug from ADR-002 (609 days of silent corruption)

### 3. Simulation: Reproduced the Failure Pattern

Running `/tmp/barman_part_size_simulation.py` demonstrated:

```
Test 1: chunk_size=5MB, 512-byte tar blocks (typical uncompressed case)
Part analysis:
  Part  1:  5,243,392 bytes | remainder mod 65536:    512 | FAIL
  Part  2:  5,243,392 bytes | remainder mod 65536:    512 | FAIL
  Part  3:  5,243,392 bytes | remainder mod 65536:    512 | FAIL
  Part  4:  5,243,392 bytes | remainder mod 65536:    512 | FAIL
  Part  5:  5,240,832 bytes | remainder mod 65536: 63,488 | FAIL

Pattern: Every part is chunk_size + N bytes where N < 512
Part 1 overflow: 512 bytes (multiple of 512: True)
```

Even with `chunk_size = 5,242,880` (which IS a multiple of 65536), parts fail because the overflow (512 bytes) breaks alignment.

### 4. Real-World Failure Reconstruction

From the actual iad-ci pod logs (2026-08-06T21:03-21:04Z):

```
queue-db:          Part size 11876352 is not a multiple of the block size (65536 bytes)
forgejo-postgres:   Part size 4284416 is not a multiple of the block size (65536 bytes)
```

Analysis:

```
queue-db: 11,876,352 = 65536 × 181 + 14,336
           14,336 = 512 × 28  ✓ (multiple of 512)
                           ✗ (not multiple of 65536)

forgejo-postgres: 4,284,416 = 65536 × 65 + 24,576
                  24,576 = 512 × 48  ✓ (multiple of 512)
                                    ✗ (not multiple of 65536)
```

Both remainders are exact multiples of 512 (tar block size) but NOT multiples of 65536. This matches the simulated behavior exactly.

## Why the "Fix" Didn't Work

The declarative-config commit `c8aefe75` (2026-08-06 08:50 EDT) disabled gzip compression, reasoning that:

> "barman's tar streamer flushes to S3 in exact 65536-byte chunks when uncompressed"

**This was incorrect.** The simulation proves that:
1. Tar writes in 512-byte blocks (POSIX standard, not configurable)
2. Barman's flush condition produces overflow parts
3. Overflow is a multiple of 512, not 65536

The "verification" was based on insufficient testing — likely a single small backup that happened to produce part sizes by chance, not from understanding barman's actual write behavior.

## Assessment of Possible Fixes

### Option 1: Client-Side Fix (Barman Configuration)

**Assessment:** No reliable client-side fix exists with barman-cloud-backup 3.19.1.

Attempted workarounds:
1. `--min-chunk-size=5MB` (default): Still produces misaligned parts
2. `--min-chunk-size=2GB`: Attempted to force single-part uploads. Still produced one misaligned part.
3. Disable gzip compression: Still produces misaligned parts (as demonstrated above)

Why no configuration works:
- The problem is in barman's `write()` logic, not configuration parameters
- Even with a `chunk_size` that is a multiple of 65536, the overflow breaks alignment
- The only way to fix this in barman would be to modify the source code to pad parts to 65536-byte boundaries

### Option 2: ARMOR Accommodation (Opt-In Mode)

**Assessment:** Possible, but high-risk and narrowly scoped.

Could add an opt-in mode for clients that cannot guarantee alignment:
- Allow misaligned parts but compute HMAC offsets differently
- Would need a different integrity mechanism to avoid reintroducing ADR-002's corruption

Risks:
- Must ensure the new mechanism does not have the same blind spots as the pre-ADR-005 code
- Would require careful threat modeling and extensive testing
- Adds complexity to ARMOR's encryption layer

### Option 3: Route Through Alternative Backend (Garage)

**Assessment:** Lowest risk, follows existing pattern.

Other clusters (`apexalgo-iad`, `ardenone-cluster`) already route barman backups through Garage with ARMOR/B2 as secondary off-site copy. This:
- Avoids the ARMOR multipart path entirely
- Maintains ARMOR's correctness guarantees
- Follows an already-operational pattern

Trade-off:
- Adds Garage to the backup path for iad-ci
- Requires Garage deployment/maintenance in iad-ci

## Recommendation

**Route queue-db and forgejo-postgres base backups through Garage** (or another S3-compatible backend that does not require block-aligned parts), with ARMOR/B2 as a secondary off-site copy. This is the lowest-risk option that maintains ARMOR's correctness guarantees while immediately resolving the backup failures.

**Secondary:** If an ARMOR accommodation is pursued, it must be:
1. Opt-in (not the default behavior)
2. Designed with adversarial threat modeling to ensure it does not reintroduce silent corruption
3. Extensively tested against the same failure modes that ADR-002/003/005 addressed

**Not Recommended:** Further attempts to fix this through barman configuration alone — the root cause is in barman's upload logic, not configuration parameters.

## Appendix: Calculation Details

### Why 512 ≠ 65536 Cannot Be Fixed by Configuration

For any `chunk_size` value C:
```
Part size = C + N × 512
```

where N depends on the backup stream's write pattern (unpredictable).

For ARMOR compliance:
```
C + N × 512 ≡ 0 (mod 65536)
```

This requires:
```
N × 512 ≡ -C (mod 65536)
512N ≡ (65536 - C % 65536) (mod 65536)
```

Since `gcd(512, 65536) = 512`, this equation has a solution only if:
```
(65536 - C % 65536) % 512 == 0
```

Even when this holds, N must be exactly right. But N is determined by the PostgreSQL backup stream's write pattern, which is outside barman's control. **No fixed C can guarantee correct N for arbitrary data sizes.**

The only reliable solution is to pad parts to 65536-byte boundaries in barman's `write()` method — a code change, not a configuration change.
