# CI Gate Verification: Multipart Integration Tests

**Date:** 2026-07-14  
**Bead:** bf-180ohh  
**Task:** Verify multipart integration test gates CI pipeline

## Findings

### ✅ CI Workflow Identified
The `armor-build` WorkflowTemplate in the `iad-ci` cluster builds ARMOR images:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflowtemplate armor-build -n argo-workflows
```

### ✅ Tests Run Before Image Build
The workflow runs steps sequentially:
1. `resolve-version` - Bump VERSION
2. `lint` - golangci-lint  
3. **`test`** - `go test -v -race ./...`
4. `docker-build` - Kaniko builds `ronaldraygun/armor:{version}`

The `test` step runs **before** `docker-build`. If tests fail, the workflow stops and no image is built.

### ✅ Integration Tests Are Included
The multipart integration tests in `internal/backend/multipart_integration_test.go`:
- **TestMultipartUpload** - 3-part 15MB upload with distinguishable content
- **TestMultipartUploadNonAlignedFinalPart** - Tests non-standard final part size
- **TestMultipartUploadIrregularFinalPart** - Similar to above with different parameters
- **TestMultipartUploadSinglePart** - Single-part "multipart" upload

These tests:
- Have NO build tags (included in `go test ./...`)
- Are NOT skipped in CI (only skipped with `-short` flag, which CI does not use)
- Reside in the `backend` package which is part of the standard test path

### ✅ Content Verification is Comprehensive
The tests use distinguishable byte patterns for each part:
- Part 0: Sequential bytes (0x00, 0x01, 0x02, ...)
- Part 1: Reverse sequential (0xFF, 0xFE, 0xFD, ...)
- Part 2: Alternating pattern (0xAA, 0x55, 0xAA, 0x55, ...)

After upload, the tests:
1. Download the combined object
2. Verify ContentLength matches expected
3. Perform byte-by-byte comparison with `bytes.Equal()`
4. Report exact offset and expected/actual bytes on mismatch

### ✅ Mock Backend Exercises Real Logic
The `mockBackendForMultipart` implements:
- `CreateMultipartUpload` - Generates upload ID
- `UploadPart` - Stores each part separately with etag
- `CompleteMultipartUpload` - **Combines parts in order** (this is where corruption would occur)
- `Get` - Retrieves combined object for verification

The concatenation logic in `CompleteMultipartUpload` appends parts sequentially, which is exactly the pattern that could produce corruption if implemented incorrectly.

## Conclusion

**The multipart integration tests ARE gated in CI and would prevent content-correctness regressions from shipping.**

A bug in multipart part concatenation would cause `bytes.Equal()` to fail, which would fail the test, which would stop the workflow, which would prevent the Docker image from being built and shipped.

## Test Coverage Summary

| Test | Scenario | Content Pattern | Verification |
|------|----------|-----------------|--------------|
| TestMultipartUpload | 3×5MB parts | Sequential, reverse, alternating | Byte-by-byte |
| TestMultipartUploadNonAlignedFinalPart | 2×5MB + 3MB final | Same as above | Byte-by-byte |
| TestMultipartUploadIrregularFinalPart | 2×5MB + 3MB final | Same as above | Byte-by-byte |
| TestMultipartUploadSinglePart | 1×5MB part | Sequential pattern | Byte-by-byte |

## CI Command

The actual command run in CI:
```bash
go test -v -race ./...
```

From `internal/backend/` this expands to:
```bash
go test -v -race github.com/jedarden/armor/internal/backend
```

Which includes all `*_test.go` files with package `backend`.
