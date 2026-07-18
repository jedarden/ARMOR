# bf-28rb — Strengthen multipart integration test to verify actual content

## Status: VERIFIED COMPLETE — code delivered in prior commits; this session re-verified and confirmed the CI gate

## Why this bead was still open

The task brief described `tests/integration/integration_test.go TestMultipartUpload` as a
3-part 15MB upload/download cycle that "only asserts ContentLength matches expected size --
never reads back and compares actual bytes." That path does not exist in this repo. The
actual test lives at `internal/backend/multipart_integration_test.go`, and by the time this
bead was re-picked-up the code already implemented every acceptance criterion (delivered in
`5183cdb7`, format-specifier fix in `03b39d29`, documented by a prior bead `bf-16yc7k` in
`0346c956`). The original `bf-28rb` bead was simply never closed.

This session's contribution is **independent re-verification** that the acceptance criteria
are met, the tests are green, and — the part prior notes only asserted — that the test
actually gates the deployed-image pipeline.

## Acceptance criteria — all met

| # | Criterion | Status | Where |
|---|-----------|--------|-------|
| 1 | Seed each part with distinguishable, verifiable content | met | Per-part patterns: part 0 = sequential `i&0xFF`, part 1 = inverse `0xFF-(i&0xFF)`, part 2 = alternating `0xAA/0x55` (lines 40-68) |
| 2 | Compare full downloaded bytes against what was uploaded | met | `io.ReadAll(body)` + `bytes.Equal(downloadedContent, uploadContent)` with first-mismatch offset/byte/context diagnostics (lines 129-164) |
| 3 | Sibling: final part not a multiple of part size | met | `TestMultipartUploadNonAlignedFinalPart` and `TestMultipartUploadIrregularFinalPart` (5MB+5MB+3MB = 13MB) |
| 4 | Sibling: single-part multipart upload | met | `TestMultipartUploadSinglePart` (one 6MB part) |
| 5 | Test gates the image-build/CI pipeline | met | see "CI gating" below |

## Test results (this session, live)

```
go test -run 'TestMultipartUpload' -v ./internal/backend/
--- PASS: TestMultipartUpload (0.10s)                     # 15MB / 3 parts, byte-for-byte + per-part pattern
--- PASS: TestMultipartUploadNonAlignedFinalPart (0.10s)  # 13MB / 5MB+5MB+3MB
--- PASS: TestMultipartUploadIrregularFinalPart (0.08s)   # 13MB / 5MB+5MB+3MB, full diagnostics
--- PASS: TestMultipartUploadSinglePart (0.02s)           # 6MB single part
ok  github.com/jedarden/armor/internal/backend  0.310s
```

Full package also green: `go test ./internal/backend/` -> `ok`. `go vet ./internal/backend/` clean.
CI runs `go test -v -race ./...`, so all four tests execute under the race detector with no
`-short` flag (the in-test `testing.Short()` skip is never triggered in CI).

## CI gating — confirmed (the part prior notes only asserted)

The deployed image is produced by the `armor-build` Argo WorkflowTemplate in
`jedarden/declarative-config` (`k8s/iad-ci/argo-workflows/armor-workflowtemplate.yml`).
Its `build` entrypoint is a **sequential** step chain — each entry is its own step-group, so
a group does not start until the previous one succeeds:

```
resolve-version -> lint (golangci-lint) -> test (go-test) -> docker-build (kaniko push)
```

The `go-test` step runs `go test -v -race ./...` against the cloned repo. Because the groups
run sequentially, `docker-build` — which pushes `ronaldraygun/armor:<version>` and
`:latest` — **does not execute unless `test` passes**. A content-correctness regression caught
by any of the four multipart tests therefore blocks the image from shipping. This is the
mechanism that makes "a content-correctness regression should never be able to ship" hold.

## Known limitation (documented honestly; out of scope for this bead)

The four tests exercise a `mockBackendForMultipart` — an in-memory `map[string][]byte`
whose `CompleteMultipartUpload` concatenates part slices in order and whose `Get` returns
them verbatim. They verify the **multipart contract, part ordering, and content-stitching
logic** plus the test harness, not a live B2/S3 endpoint. The real backend
(`internal/backend/b2.go`, `internal/backend/multipart_helpers.go`) is covered separately by
the AWS-client-fake-based tests in `internal/backend/b2_multipart_test.go`, which are also
mock-based (no live network). The `backend` package is not tested against a live remote
endpoint anywhere today (that would need credentials/egress in CI). So the gate is on the
contract/concatenation path, not on remote round-trip fidelity — a real, but bounded, gap.

## Prior commits delivering this work
- `5183cdb7` — original implementation of distinguishable content + byte-for-byte comparison + sibling tests
- `03b39d29` — fix format specifier in multipart test error message
- `0346c956` — `docs(bf-16yc7k)`: prior bead documenting the work was already complete
