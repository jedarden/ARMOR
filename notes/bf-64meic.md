# Multipart Download and Verification - Already Implemented

**Date:** 2026-07-16
**Bead:** bf-64meic

## Summary

The multipart download and verification logic requested in this bead was already fully implemented in commit `8ead837a` on 2026-07-14 as part of bead `bf-4595`.

## Implementation Details

The `checkMultipart()` function in `internal/canary/canary.go` (lines 470-700) implements all required verification:

### 1. Download the multipart canary object
```go
body, headers, err := m.backend.GetRangeWithHeaders(ctx, m.bucket, key, 0, int64(len(envelope)))
downloadedEnvelope, err := io.ReadAll(body)
```
(lines 603-610)

### 2. Byte-for-byte verification
```go
// Verify size matches
if len(downloadedEnvelope) != len(envelope) {
    return nil, fmt.Errorf("multipart download size mismatch: got %d, expected %d", len(downloadedEnvelope), len(envelope))
}

// Byte-for-byte verification (critical for multipart integrity)
if !bytes.Equal(downloadedEnvelope, envelope) {
    return nil, fmt.Errorf("multipart content byte-for-byte verification failed")
}
```
(lines 622-630)

### 3. HMAC verification (reusing existing Monitor logic)
```go
// Verify HMACs
if err := decryptor.VerifyHMACs(downloadedEncrypted, downloadedHMAC); err != nil {
    result.HMACVerified = false
    return nil, fmt.Errorf("HMAC verification failed: %w", err)
}
result.HMACVerified = true
```
(lines 664-668)

### 4. Plaintext SHA verification
```go
// Verify plaintext SHA
if err := downloadedHeader.VerifyPlaintextSHA(decrypted); err != nil {
    return nil, fmt.Errorf("plaintext SHA verification failed: %w", err)
}
```
(lines 683-686)

### 5. Clear pass/fail result
```go
result.Status = StatusHealthy
result.MultipartHealthy = StatusHealthy
result.MultipartHealthyBool = true

return result, nil
```
(lines 695-700)

## Test Coverage

All tests pass:
- `TestMonitorMultipartCheck` - Verifies end-to-end multipart check
- `TestMonitorMultipartIntegration` - Verifies metrics integration
- `TestMultipartHealthyBoolField` - Verifies status reporting
- `TestMultipartHealthyBoolFieldFailure` - Verifies failure handling
- `TestCanaryHealthResponseJSON` - Verifies JSON serialization
- `TestMultipartCanaryMetricsEmission` - Verifies Prometheus metrics

## Related

- ADR-002: "Close detection gaps that let the multipart-upload corruption bug run 40 days undetected"
- Bead bf-4595: Original implementation bead
- Commit 8ead837a: Implementation commit
