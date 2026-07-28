# bf-1v6skf Blocked State (2026-07-28)

## Current Status: ARMOR Code Fix CONFIRMED, but Acceptance Criterion BLOCKED

### What's Confirmed (✓)

The ARMOR multipart/HMAC decrypt code defect is **FIXED and VERIFIED**:

- Comment 94 (2026-07-22): Successfully GET+decrypted 56.6MB production object
  - `kalshi-tape/parquet/2026-07-22/14/orderbook_delta.parquet`
  - 56,612,636 bytes, written hours after 0.1.1901 fleet deploy
  - Byte-exact download (matches ContentLength)
  - Spans ~864 blocks at 64KiB block size (3x past historical block-256 failure point)
  - Structurally valid Parquet (1,868,214 rows, correct schema)
  - Semantically sane content (real Kalshi market tickers)

This definitively proves the ARMOR read-path fix is working on real large-scale production data.

### What's Blocking Acceptance Criterion (✗)

This bead's acceptance criterion requires:
> "queue-api backup chain re-verified end-to-end restorable after the fix"
> -- "the literal restored file path and a real litestream/sqlite3 command transcript"
> (per bf-2sq7gf's acceptance criteria, cross-linked in comment 88)

**BLOCKED by separate litestream bug** (Comment 93, 2026-07-22):

```
litestream on queue-api (commitgraph ns, ord-devimprint) has written ZERO new 
backup objects to B2 since 2026-07-18 10:37

Cause: local .queue.db-litestream/ltx/0 dir is missing its first segment 
(0000000000000001-...ltx), litestream is stuck retrying from txid 1 while 
local files start at 0x26

Effect: continuous 'LTX file is missing' errors since pod started 
2026-07-21T18:27:47Z (234+ consecutive errors)

Remediation needed: 'litestream reset <db>' or delete .queue.db-litestream 
+ restart
```

### Consequences

1. **No fresh post-fix snapshot exists** - Last snapshot (2026-07-18) is from pre-fix ARMOR (corrupt at block 256/400/512 per comment 84)
2. **Cannot test DR restore** - Even with ARMOR fixed, there's no valid post-fix queue-api backup object to restore
3. **DR path unproven on this cluster** - While ARMOR works on kalshi-tape data, the queue-api specific DR path remains unverified

### What's Needed to Complete This Bead

1. Fix litestream local state loss on queue-api (separate from ARMOR)
2. Wait for fresh snapshot to be created (post-0.1.1901 ARMOR deploy)
3. Run litestream restore against that snapshot
4. Verify with sqlite3 PRAGMA integrity_check
5. Provide the literal restored file path and command transcript

### Recommendation

This bead should remain **open/blocked** until:
- The litestream state issue is resolved, AND
- A fresh post-fix snapshot is created, AND
- A successful restore transcript is produced

The ARMOR code defect IS resolved (separate verification completed), but the 
queue-api DR acceptance criterion cannot be met until litestream is unblocked.
