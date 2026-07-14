# Malformed Signature Rejection Test Coverage

## Task: Verify that requests with malformed signatures are rejected with 403

### Test Results
All malformed signature rejection tests are passing. The test suite is located in `internal/server/malformed_signature_test.go`.

### Acceptance Criteria Coverage

#### ✅ 1. Invalid signature format returns 403 Forbidden
Test cases verify:
- Missing algorithm prefix (e.g., `Credential=...` without `AWS4-HMAC-SHA256`)
- Wrong algorithm (e.g., `AWS4-HMAC-SHA1` instead of `AWS4-HMAC-SHA256`)
- Missing signature component (valid format but missing `Signature=`)
- Missing signed headers
- Missing credential
- Malformed credential with insufficient parts
- Empty signature value (`Signature=`)

All return `403 Forbidden` with appropriate error codes (`InvalidAlgorithm`, `IncompleteSignature`, `InvalidCredential`).

#### ✅ 2. Garbage signature string returns 403 Forbidden
Test cases verify:
- Non-hex signature (`this-is-not-a-valid-hex-signature-at-all!!!`)
- Too short signature (`abc123`)
- Empty signature (``)
- Random characters (`!@#$%^&*()_+-=[]{}|;':,.<>?/`)

All return `403 Forbidden` with `SignatureDoesNotMatch` or `IncompleteSignature` error codes.

#### ✅ 3. Partial signature (missing components) returns 403 Forbidden
Test cases verify:
- Only algorithm (`AWS4-HMAC-SHA256`)
- Algorithm and credential only (missing signature)
- Missing signature in otherwise valid format

All return `403 Forbidden` with appropriate error codes (`InvalidAlgorithm`, `IncompleteSignature`).

#### ✅ 4. Error responses include meaningful error messages
Test cases verify that error messages:
- Are non-empty
- Describe the authentication problem
- Contain relevant terms (authentication, signature, credential, algorithm, header, aws4)

#### ✅ 5. Rejection happens quickly (no long timeouts)
Test cases verify that malformed signatures are rejected in under 50ms for local tests, ensuring no unnecessary delays or timeouts in the authentication validation logic.

### Test Structure
The main test function `TestMalformedSignatureRejection` is organized into subtests:
1. `Garbage signature string returns 403 Forbidden` - Tests invalid hex strings
2. `Invalid signature format returns 403 Forbidden` - Tests malformed auth headers
3. `Partial signature (missing components) returns 403 Forbidden` - Tests incomplete auth headers
4. `Error responses include meaningful error messages` - Validates error message quality
5. `Rejection happens quickly (no long timeouts)` - Performance validation
6. `Valid signature is still accepted` - Ensures valid auth still works

### Implementation Notes
- Tests use `httptest.NewRecorder()` for HTTP response capture
- Error responses are parsed as XML S3 error responses
- Helper function `createMalformedSigRequest` creates test requests with malformed signatures
- All test cases use valid credentials except where testing invalid formats

### Test Execution
```bash
go test -v ./internal/server -run TestMalformedSignatureRejection
```

All 22 subtests pass consistently.
