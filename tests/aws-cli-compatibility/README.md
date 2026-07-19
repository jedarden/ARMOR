# AWS CLI / rclone Compatibility Tests

Implements the **Compatibility Tests** section of `docs/plan/plan.md`: it
verifies that the real `aws` CLI (`s3 cp` / `ls` / `rm` / `sync`, plus
`head-object` and server-side copy) and `rclone copy` round-trip byte-identical
objects through ARMOR.

This is the plan's third pillar of multipart/transfer coverage alongside the
boto3-driven `tests/test_s3_basic_operations.py` and the Go-level
`internal/server/handlers/handlers_test.go` / `internal/dashboard/*_test.go`
suites.

## How it works

Each test spins up an in-process ARMOR HTTP server — the **real** request
pipeline (SigV4 auth, `aws-chunked` streaming decode, ACL enforcement, and the
handlers in `internal/server/handlers`) — backed by a thread-safe in-memory
mock backend (`harness_test.go`). The server is built by
`internal/server.NewWithBackend`, an exported test helper that injects the
backend so no B2 or Cloudflare credentials are needed. The CLI is then pointed
at `httptest.NewServer(srv.Handler())` via `--endpoint-url` (AWS CLI) or an
`rclone.conf` S3 remote (`force_path_style = true`). Every round-tripped
object is checked byte-for-byte (SHA-256) against the original.

## Two layers

| File | What | When it runs |
|------|------|--------------|
| `awscli_compat_test.go` | `TestAWSCLI_*` / `TestRclone_*` — shells out to the **real** `aws` and `rclone` binaries | Only when the binaries are on `PATH` **and** not under `-short` |
| `zz_verify_sdk_test.go` | `TestVerify_*` — drives the identical request paths via `aws-sdk-go-v2` (multipart, out-of-order completion, concurrent transfers) | **Always**, including CI's `-short` gate — it needs no external binaries |

The `TestVerify_*` smoke tests are the suite's teeth on machines without the
CLIs: they exercise the same in-process server and handlers the CLI tests use,
so if they pass, the CLI tests will pass once the CLIs are installed.

## Skipping cleanly (does not break CI)

Neither `aws` nor `rclone` is installed in the current dev/CI image. The
`TestAWSCLI_*` / `TestRclone_*` tests detect this and skip with a clear reason
rather than fail:

```
--- SKIP: TestAWSCLI_PutGetRoundTrip (0.00s)
    aws CLI not installed on PATH — skipping AWS CLI compatibility test
    (install with e.g. `pip install awscli` or the official AWS CLI v2 bundle)
```

They also short-circuit under `testing.Short()`, so the Dockerfile test gate
(`CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short`) stays green
regardless of CLI presence.

## Running the full suite for real

Install both CLIs, then run without `-short`:

```bash
pip install awscli            # or the official AWS CLI v2 bundle
curl https://rclone.org/install.sh | sudo bash   # see https://rclone.org/install/

go test -race ./tests/aws-cli-compatibility/
```

Adding `aws-cli` and `rclone` to the CI image so the `TestAWSCLI_*` /
`TestRclone_*` tests run there too is a **separate infra decision** and is not
done by this suite — see `Dockerfile`. The `TestVerify_*` smoke tests already
run in CI today.

## Files

- `harness_test.go` — mock `backend.Backend`, in-process server factory
  (`startArmorServer`), AWS/rclone config builders, file-equality and CLI
  helpers, and the binary-presence / `-short` skip guards.
- `awscli_compat_test.go` — the CLI-gated compatibility tests.
- `zz_verify_sdk_test.go` — the always-runs SDK smoke tests.
