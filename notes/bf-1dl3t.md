# Bead bf-1dl3t: Validation Result - FAILED

## Task
Verify decoded value is human-readable and not corrupted

## Result
**FAILED** - Binary corruption detected

## Evidence

### Hex dump (first 40 bytes)
```
00000000: f797 1bdf 97f6 6baf 3469 e7f9 6b96 faf7  ......k.4i..k...
00000010: 66df 75ef 38f5 fd7a 6da6 b6eb b7da d377  f.u.8..zm......w
00000020: 9d6f bd3a df47 7ad7                      .o.:.Gz.
```

### Analysis
- Bytes like `0xf7`, `0x97`, `0xdf`, `0xe7` are outside printable ASCII range (0x20-0x7E)
- This is binary data, not human-readable text
- No control characters (0x00-0x1F) were detected by grep, but binary data is present
- File length: 46 bytes of raw binary data

## Acceptance Criteria Status
- ❌ Value contains only printable ASCII characters - **FAILED**
- ✅ No control characters detected (passed grep test)
- ✅ No null bytes detected
- ❌ Value is not garbled or corrupted - **FAILED**
- ❌ Hex dump shows clean ASCII - **FAILED**

## Conclusion
The decoded value in `/tmp/litestream_key_id.txt` contains **corrupted binary data**, not clean ASCII text. The secret appears to be corrupted or improperly encoded.

## Next Steps
This bead cannot be closed successfully. The root cause of the binary corruption needs to be investigated in the encoding/decoding chain.
