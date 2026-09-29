# S3 Client Compatibility Tests

Implements the **Compatibility Tests** section of `docs/plan/plan.md`: it
verifies that real S3 clients round-trip or read data through ARMOR: the AWS
CLI, rclone, boto3, DuckDB/httpfs, litestream, and barman-cloud. The fixtures
under `testdata/` are the checked-in, credential-free forms of the documented
client examples; each test renders a private copy with only ephemeral runtime
values.

This is the plan's third pillar of multipart/transfer coverage alongside the
production-image boto3 test in `tests/test_s3_basic_operations.py` and the Go-level
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

## Client legs and two layers

| File | What | When it runs |
|------|------|--------------|
| `awscli_compat_test.go` | `TestAWSCLI_*` — shells out to the **real** `aws` binary: put/get round-trip, list and delete, sync, multipart, server-side copy | Only when the binary is on `PATH` **and** not under `-short` |
| `rclone_compat_test.go` | `TestRclone_*` — the **real** `rclone` binary: credential accept plus wrong-secret / unknown-key rejections (`SignatureDoesNotMatch` / `InvalidAccessKeyId`), copy round-trip, single-PUT write and read, listing with sizes and prefix scoping, byte-range reads via `cat --head/--offset/--count`, overwrite and delete, and a low-cutoff multipart upload with a part-boundary range read (the multipart download passes `--ignore-checksum` and asserts bytes directly: ARMOR's completed-multipart ETag is a bare-hex ciphertext digest, which rclone would otherwise misread as a content MD5 — real S3 returns the `md5(md5s)-N` composite there) | Only when `rclone` is on `PATH` **and** not under `-short`; endpoint mode makes a missing binary fatal |
| `boto3_compat_test.go` + `testdata/boto3_leg.py` | botocore's real SigV4 signer: valid authentication plus wrong-secret / unknown-key rejection, single-PUT with metadata, full and bounded range reads, HEAD, listing, overwrite, delete with `NoSuchKey`, explicit multipart (`create_multipart_upload` / `upload_part` / `list_parts` / `complete_multipart_upload`, short final part, part-boundary range read) and the abort path | Full mode; endpoint mode makes missing boto3 fatal |
| `duckdb_compat_test.go` + `testdata/duckdb-httpfs.sql` | DuckDB/httpfs footer and column range reads over an encrypted Parquet object | Full mode; endpoint mode makes missing DuckDB fatal |
| `litestream_compat_test.go` + `testdata/litestream.yml` | `litestream replicate` followed by `litestream restore` and SQLite content verification | Full mode; endpoint mode makes missing Litestream fatal |
| `barman_compat_test.go` + `testdata/barman-cloud.env` | `barman-cloud-backup` with 5MB tar chunks, then `barman-cloud-restore` into a fresh PostgreSQL cluster | Full mode; endpoint mode makes missing Barman/PostgreSQL tools fatal |
| `zz_verify_sdk_test.go` | `TestVerify_*` — drives the identical request paths via `aws-sdk-go-v2` (multipart, out-of-order completion, concurrent transfers) | **Always**, including CI's `-short` gate — it needs no external binaries |
| `protocol_conformance_test.go` | `TestVerify_Authentication` / `_RangeReads` / `_List` / `_Overwrite` / `_Delete` — pins the wire behaviors behind README's compatibility claim: SigV4 accept plus wrong-secret / unknown-key / unsigned rejections with standard error codes, block-boundary byte-range reads, prefix listing with HEAD agreement, wholesale overwrite, idempotent delete | **Always**, including CI's `-short` gate — no external binaries |
| `client_matrix_test.go` | `TestClientMatrix_*` — the executable client matrix: binds every leg above to the operations it covers (authentication, reads, range reads, listing, overwrite/delete, multipart — where supported), pins the always-run conformance floor, and holds this table, the README intro's client list, and AGENTS.md's CI claim to the legs that actually exist | **Always**, including CI's `-short` gate and the armor-build compatibility gate against the published image — no external binaries |

The `TestVerify_*` smoke tests are the suite's teeth on machines without the
CLIs: they exercise the same in-process server and handlers the CLI tests use,
so if they pass, the CLI tests will pass once the CLIs are installed.

## Skipping cleanly (does not break CI)

A bare development machine typically has neither `aws` nor `rclone` on `PATH`
(the armor-build compatibility gate installs pinned versions of every client
before running the suite — see below). The `TestAWSCLI_*` / `TestRclone_*`
tests detect a missing binary and skip with a clear reason rather than fail:

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

The built-image client matrix runs in endpoint mode. The boto3 leg uses
botocore's real SigV4 signer rather than a hand-written request signer; the
DuckDB, Litestream, and Barman legs likewise invoke their real binaries:

```bash
ARMOR_COMPAT_ENDPOINT=http://127.0.0.1:9000 \
ARMOR_COMPAT_ACCESS_KEY=armor \
ARMOR_COMPAT_SECRET_KEY=armor-demo-secret \
ARMOR_BUCKET=demo-bucket \
python3 tests/test_s3_basic_operations.py
```

`make compat` runs the Go client matrix and the broader boto3 test when
`ARMOR_COMPAT_ENDPOINT` is set. The image compatibility gate starts the freshly
built image, exports these variables, installs the client versions pinned by
the workflow, and runs every matrix leg. Credentials are supplied by the
deployment's secret references; no credential value is stored in a fixture or
the repository.

The `TestShortFinalPart_*` multipart integration tests
(`short_final_part_test.go`) are gated behind the `awscli_integration` build
tag and are **not** included in the command above — their multipart handshake
against the in-process server is not green in the default run, so they are
excluded to keep `go test ./...` green. Run them explicitly on demand:

```bash
go test -tags awscli_integration ./tests/aws-cli-compatibility/
```

## The armor-build compatibility gate

CI exercises every matrix leg against the freshly built image: the
`compat-suite-test` step of `armor-build`
(`declarative-config/k8s/iad-ci/argo-workflows/armor-workflowtemplate.yml`)
installs pinned aws CLI, rclone, litestream, DuckDB, barman and boto3
versions, starts the new image with an ephemeral filesystem backend on the
production `serve` path, exports the endpoint-mode variables, and runs this
whole package plus `tests/test_s3_basic_operations.py`. In endpoint mode a
missing client is fatal rather than a skip, so a leg can never silently drop
out of the gate. The `TestVerify_*` smoke tests additionally run on every
plain `go test` via CI's `-short` gate. The `TestClientMatrix_*` inventory
pin runs in that same image gate, so a leg dropped from the package — or an
operation binding, or a documented client list — fails the gate against the
published image itself, not just a local run.

## Files

- `harness_test.go` — mock `backend.Backend`, in-process server factory
  (`startArmorServer`), AWS/rclone config builders, file-equality and CLI
  helpers, and the binary-presence / `-short` skip guards.
- `awscli_compat_test.go` — the CLI-gated compatibility tests.
- `zz_verify_sdk_test.go` — the always-runs SDK smoke tests.
- `client_matrix_test.go` — the executable client matrix and its parity
  pins (see "Client legs and two layers").
