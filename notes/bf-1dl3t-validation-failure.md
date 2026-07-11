# Validation Failure - bf-1dl3t

## Date: 2026-07-11

## Finding: Binary data corruption in decoded value

### Expected Format
AWS Access Key ID should be:
- 20 alphanumeric characters (A-Z, 0-9)
- Typically starts with prefix like "AKIA"
- Printable ASCII only

### Actual Content
The file `/tmp/litestream_key_id.txt` contains:

**Hex dump:**
```
000000 f7 97 1b df 97 f6 6b af 34 69 e7 f9 6b 96 fa f7  >......k.4i..k...<
000010 66 df 75 ef 38 f5 fd 7a 6d a6 b6 eb b7 da d3 77  >f.u.8..zm......w<
000020 9d 6f bd 3a df 47 7a d7 9f 75 e9 df 5b 6f cd dd  >.o.:.Gz..u..[o..<
000030
```

**File size:** 48 bytes (46 characters after stripping whitespace)

### Analysis
- Bytes like `0xf7`, `0x97`, `0x1b`, `0xdf` are **not printable ASCII**
- This is clearly binary data, not a valid AWS Access Key ID
- The secret was corrupted either:
  1. During initial storage in OpenBao
  2. During ExternalSecret sync
  3. During base64 decoding

### Next Steps
The secret needs to be regenerated or re-imported from the source of truth.
The ExternalSecret and OpenBao secret may need to be recreated with clean data.

### Related Issues
- Parent bead: bf-1dl3t
- Previous validation bead: bf-1y0g6 (should have caught this format mismatch)
