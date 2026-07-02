# Test Gate Verification - bf-68dl

## Implementation
The test gate was implemented in commit `7a8d89d`:
```dockerfile
# Test gate: run go vet and unit tests before building
# Integration tests (tests/integration/) require build tags and credentials,
# and are automatically skipped by this gate.
RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
```

## Acceptance Criteria Verification

### 1. Build fails when a unit test fails
**Status:** ✅ VERIFIED

**Method:** Temporarily broke `TestManifestPrefix` in `internal/config/config_test.go` by changing the expected value to `"this-will-fail"`. Ran `docker build` which failed at the test gate step with exit code 1.

**Output:**
```
ERROR: process "/bin/sh -c CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short" did not complete successfully: exit code: 1
```

### 2. Build succeeds on clean HEAD with no credentials
**Status:** ✅ VERIFIED

**Method:** Ran `docker build -t armor-test-gate .` with no B2/Cloudflare credentials available. All unit tests passed.

**Output:**
```
#12 [builder 7/8] RUN CGO_ENABLED=0 go vet ./... && CGO_ENABLED=0 go test ./... -short
#12 10.51 ?   	github.com/jedarden/armor/cmd/armor	[no test files]
#12 10.52 ok  	github.com/jedarden/armor/cmd/armor-decrypt	0.006s
#12 10.52 ok  	github.com/jedarden/armor/internal/b2keys	0.005s
#12 10.52 ok  	github.com/jedarden/armor/internal/backend	0.006s
#12 10.77 ok  	github.com/jedarden/armor/internal/canary	0.256s
#12 10.77 ok  	github.com/jedarden/armor/internal/config	0.003s
#12 10.77 ok  	github.com/jedarden/armor/internal/crypto	0.165s
#12 10.77 ok  	github.com/jedarden/armor/internal/dashboard	0.016s
#12 10.77 ok  	github.com/jedarden/armor/internal/keymanager	0.004s
#12 10.77 ok  	github.com/jedarden/armor/internal/logging	0.003s
#12 10.77 ok  	github.com/jedarden/armor/internal/manifest	0.091s
#12 10.77 ok  	github.com/jedarden/armor/internal/metrics	0.053s
#12 10.77 ok  	github.com/jedarden/armor/internal/presign	0.006s
#12 10.77 ok  	github.com/jedarden/armor/internal/provenance	0.004s
#12 10.77 ok  	github.com/jedarden/armor/internal/server	0.128s
#12 10.99 ok  	github.com/jedarden/armor/internal/server/handlers	0.476s
#12 DONE 11.2s
```

### 3. Integration tests are properly skipped
**Status:** ✅ VERIFIED

**Method:** Checked that `tests/integration/` tests require the `integration` build tag (see `tests/integration/README.md:49`). The gate runs `go test ./... -short` without `-tags=integration`, so those tests are never compiled or run. Verified with `CGO_ENABLED=0 go test ./... -short` which returned no integration test output.

### 4. Build time increase is acceptable
**Status:** ✅ VERIFIED

**Measurement:** Test gate took ~11 seconds on a cached build. Well under the 2-minute threshold.

## Implementation Details

- `go vet ./...` runs static analysis first
- `go test ./... -short` runs unit tests only (integration tests require build tags)
- CGO_ENABLED=0 ensures pure Go compilation for Linux target
- Integration tests in `tests/integration/` are guarded by the `integration` build tag and require environment variables (`ARMOR_INTEGRATION_TEST=1`, B2 credentials, Cloudflare domain). They self-skip when not tagged properly or when credentials are missing.
- The gate runs after `COPY . .` but before the binary build, ensuring no broken code reaches the image
