# ADR-005: AES-CTR Counter Stride Fix (Version 2)

## Status

**Accepted** - Implemented, pending deployment

## Context

### Vulnerability Discovery

On 2024-08-19, a critical security vulnerability was discovered in ARMOR's AES-CTR counter derivation for Version 1 envelopes. The bug causes **keystream reuse between adjacent 64KB blocks**, enabling two-time pad attacks where plaintext XOR can be recovered from ciphertext alone.

### Root Cause

The `makeCounter` function in `internal/crypto/encryptor.go` (line 160) derives the CTR counter as:
```
counter = IV[0:12] || uint32(blockIndex)
```

With `blockSize = 65536` (default):
- Each ARMOR block consumes 65536/16 = **4096 AES blocks** (counter increments)
- Block 0 uses counters: `[IV+0, IV+1, ..., IV+4095]`
- Block 1 starts at `IV+1`, reusing `[IV+1, ..., IV+4095]`
- **Overlap: 4095 out of 4096 counter values (99.98%)**

This is a catastrophic two-time pad vulnerability. An attacker who obtains two adjacent encrypted blocks can compute:
```
plaintext0 XOR plaintext1 = ciphertext0 XOR ciphertext1
```

### Empirical Verification

The bug was verified with a standalone test (`internal/crypto/counter_reuse_verify_test.go`):
- Generated keystream for two all-zero adjacent blocks
- Confirmed `keystream1[0:65520] == keystream0[16:65536]` (true)
- **4095 out of 4096 AES blocks share keystream**

### Impact

- **All existing Version 1 objects are vulnerable** to keystream reuse
- Zero-knowledge encryption claims in README are **false** for V1
- Plaintext relationships leak from ciphertext alone
- Attack applies to any multi-block V1 object (essentially all real-world objects)

### Duplicate Code

The vulnerable `makeCounter` pattern was duplicated in:
- `internal/crypto/encryptor.go:160`
- `internal/crypto/decryptor.go:249`
- `internal/server/server.go:1446`
- `internal/server/handlers/handlers.go:1123`

## Decision

### Version 2 Envelope Format

Introduce **Version 2** envelopes with fixed counter derivation:

```go
// Version2: stride by number of AES blocks per ARMOR block
aesBlocksPerArmorBlock := blockSize / 16  // 4096 for 64KB blocks
counterValue = blockIndex * aesBlocksPerArmorBlock
```

This ensures:
- Block 0 uses counters: `[IV+0, IV+1, ..., IV+4095]`
- Block 1 uses counters: `[IV+4096, IV+4097, ..., IV+8191]`
- **Zero overlap** between adjacent blocks

### Backward Compatibility

- **Version 1 objects continue to decrypt with legacy derivation**
- Version 1 encryption remains available for testing only
- Version 2 is **mandatory for all new objects**
- Version field in `ARMORMetadata` determines derivation at decrypt time

### API Changes

#### Encryption
```go
// Old (V1, vulnerable) - kept for backward compatibility
enc, err := crypto.NewEncryptor(dek, iv, blockSize)

// New (V2, fixed) - use for all new objects
enc, err := crypto.NewEncryptorV2(dek, iv, blockSize)

// Explicit version selection
enc, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, crypto.Version2)
```

#### Decryption
```go
// Version determined from envelope header
dec, err := crypto.NewDecryptorWithVersion(dek, iv, blockSize, header.Version)

// Or let the envelope system handle it automatically
```

### Envelope Header Changes

```go
const (
    Version1 = 0x01  // Legacy (vulnerable)
    Version2 = 0x02  // Fixed (2024-08-19)
)
```

Header validation updated to accept both versions.

## Migration Strategy

### Phase 1: Deployment (Immediate)
1. Deploy this change to production
2. Service automatically creates Version 2 envelopes for all new PUTs
3. Existing Version 1 objects remain readable (legacy decryption path)

### Phase 2: Object Migration (Required)
**All existing Version 1 objects MUST be re-encrypted to Version 2:**

```bash
# Migration tool (to be implemented)
# For each B2 object:
# 1. Download V1 envelope
# 2. Decrypt with legacy derivation
# 3. Encrypt with V2 derivation
# 4. Upload to new key
# 5. Update metadata
# 6. Delete old object
```

### Migration Priority
1. **High-value objects** (secrets, keys, sensitive data) - Immediate
2. **User data** - Within 7 days
3. **System data** - Within 30 days

### Rollout Procedure
1. Deploy V2 code to production (read-only for existing objects)
2. Run migration tool during low-traffic window
3. Verify migrated objects decrypt correctly
4. Delete old V1 objects after verification
5. Monitor for decryption failures (rollback trigger)

## Consequences

### Positive
- **Zero-knowledge encryption actually true** for V2 objects
- No keystream reuse between blocks
- Maintains backward compatibility (V1 objects still decrypt)
- Simple fix (counter stride calculation)

### Negative
- **All existing V1 objects must be re-encrypted** (non-trivial cost)
- Migration downtime during object re-upload
- Dual decryption paths in code indefinitely (maintenance burden)
- Cannot auto-detect V1 vs V2 from ciphertext alone (version in header only)

### Risks
- **Migration failure** could leave objects undecryptable (mitigated by testing)
- **Rollback complexity** if V2 has issues (V1 objects remain readable)
- **Performance impact** during migration window
- **Storage cost** during migration (old + new objects coexist temporarily)

## Alternatives Considered

### Alternative 1: Fix V1 In-Place (REJECTED)
Change V1 derivation without version bump.
- **Would make all existing objects undecryptable**
- No migration path
- Data loss guaranteed

### Alternative 2: Per-Object Nonce (REJECTED)
Generate unique nonce per block instead of IV.
- **Breaks random-access property** (cannot decrypt block N without reading 0..N-1)
- Increases envelope size significantly
- More complex implementation

### Alternative 3: Switch to GCM (DEFERRED)
AES-GCM provides authentication without separate HMAC table.
- **Major format change** (not just counter derivation)
- Would require V3 envelope format
- Can be evaluated separately

## Implementation

### Code Changes
1. `internal/crypto/envelope.go` - Add Version2 constant
2. `internal/crypto/encryptor.go` - Version-aware makeCounter
3. `internal/crypto/decryptor.go` - Version-aware makeCounter
4. `internal/server/server.go` - Update inline makeCounter
5. `internal/server/handlers/handlers.go` - Update inline makeCounter

### Tests
1. `counter_reuse_verify_test.go` - Proves V1 bug
2. `counter_reuse_fixed_test.go` - Validates V2 fix
3. All existing tests pass (backward compatibility)

### Verification
```bash
# Prove bug exists in V1
go test ./internal/crypto/ -run TestProveCounterReuseDirectly -v

# Validate fix in V2
go test ./internal/crypto/ -run TestCTRKeystreamNoReuseV2 -v

# Verify backward compatibility
go test ./internal/crypto/ -run TestCTRV1V2Compatibility -v
```

## References

- CVE-2001-1102 (similar two-time pad in SSL)
- "Why ctr fails when key repeats" - cryptographic literature
- Existing tests: `internal/crypto/crypto_test.go`
- Bug proof: `internal/crypto/counter_reuse_verify_test.go`
- Fix validation: `internal/crypto/counter_reuse_fixed_test.go`

## Implementation Date

2024-08-19

## Author

@jedarden
