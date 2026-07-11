# bf-1dl3t: Decoded Value Corruption - Binary Data Detected

## Date
2026-07-11

## Finding
The decoded value in `/tmp/litestream_key_id.txt` is **corrupted binary data**, not human-readable text.

## Evidence

### Hex dump (first 48 bytes)
```
000000 f7 97 1b df 97 f6 6b af 34 69 e7 f9 6b 96 fa f7  >......k.4i..k...<
000010 66 df 75 ef 38 f5 fd 7a 6d a6 b6 eb b7 da d3 77  >f.u.8..zm......w<
000020 9d 6f bd 3a df 47 7a d7 9f 75 e9 df 5b 6f cd dd  >.o.:.Gz..u..[o..<
```

### File characteristics
- **Length:** 46 characters (but includes non-printable bytes)
- **Control characters:** None detected (but bytes are outside ASCII printable range)
- **Null bytes:** None detected
- **File type:** Binary data (not text/ASCII)

### Expected vs Actual
| Aspect | Expected | Actual |
|--------|----------|--------|
| Format | 20-character alphanumeric (e.g., `AKIAIOSFODNN7EXAMPLE`) | 46 bytes of binary garbage |
| Character set | Printable ASCII (0-9, A-Z) | High-byte values (0xF7, 0x97, 0xDF, etc.) |
| Readability | Human-readable | Unreadable binary corruption |

## Root Cause Analysis

The corruption likely occurred during one of these stages:
1. **Base64 decoding** - Input may have been doubly-encoded or improperly encoded
2. **OpenBao secret storage** - Secret may have been corrupted at rest
3. **Transit decoding** - Character set or encoding mismatch during retrieval

## Next Steps

The parent bead needs to revisit the extraction process:
- Verify the OpenBao secret is stored correctly
- Check if double-encoding occurred (Base64-of-Base64)
- Re-extract with explicit encoding handling

## Status
**FAILED** - Value contains binary corruption, not valid AWS Access Key ID.
