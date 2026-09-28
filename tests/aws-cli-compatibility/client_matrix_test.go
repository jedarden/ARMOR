// The executable client-compatibility matrix.
//
// README.md and AGENTS.md claim CI exercises six real S3 clients — the AWS
// CLI, rclone, boto3, DuckDB/httpfs, litestream, and barman-cloud — against
// the published image. Nothing reded when that claim drifted: deleting a leg
// (or an operation inside one) left a package that still compiled and a gate
// that still installed clients nothing exercised, while the documentation
// kept naming coverage nobody executed (armor-e3772451).
//
// clientLegs below is the matrix: one row per real client, binding each
// operation the leg covers — authentication, reads, range reads, listing,
// overwrite/delete, and multipart, "where supported" — to its executable
// proof: a Go test function in the leg's file, or (for the driver-backed
// boto3 leg) a token in testdata/boto3_leg.py. conformanceFloor binds the
// same operation set to the always-run aws-sdk-go-v2 tests, so the wire
// contract is pinned on every `go test` even where a client leg does not
// cover the operation.
//
// The TestClientMatrix_* tests pin the matrix to reality in both directions
// — every binding must resolve (no phantom cell) and every leg-owned test
// function must be bound (no coverage silently outside the matrix) — and
// hold the documented client lists to the legs that actually exist. They
// parse sources and never execute binaries, so they run on every `go test`
// including CI's `-short` gate and the armor-build compatibility gate in
// ARMOR_COMPAT_ENDPOINT mode against the published image: the matrix is
// itself gated, not just documentation.
package awsclicompat

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// compatOperation names one column of the matrix. The set is the documented
// compatibility contract: the operations every named client covers where
// the client supports them.
type compatOperation string

const (
	opAuthentication  compatOperation = "authentication"
	opReads           compatOperation = "reads"
	opRangeReads      compatOperation = "range reads"
	opListing         compatOperation = "listing"
	opOverwriteDelete compatOperation = "overwrite/delete"
	opMultipart       compatOperation = "multipart"
)

// allOperations is the full column set; the union of the client legs and
// the conformance floor must cover every column.
var allOperations = []compatOperation{
	opAuthentication,
	opReads,
	opRangeReads,
	opListing,
	opOverwriteDelete,
	opMultipart,
}

// clientLeg is one row of the matrix: a real S3 client, the package file
// that gates its leg, the test-name prefix the leg owns, and the operation
// bindings. Bindings are Go test function names (must exist in file) or,
// when driver is set, tokens that must appear in the driver source.
type clientLeg struct {
	client string
	file   string
	prefix string
	// driver names a testdata/ script that carries the leg's operation
	// bodies (only boto3 today); its cells bind to source tokens instead
	// of Go test names.
	driver string
	// gate is the presence-guard call every leg test must route through,
	// so ARMOR_COMPAT_ENDPOINT mode fails rather than skips a missing
	// client — a leg can never silently drop out of the image gate.
	gate string
	ops  map[compatOperation][]string
}

// clientLegs: every real client named in README.md and AGENTS.md. An
// operation absent from a leg's map is a declared "not supported by this
// leg" cell, not an oversight — the union check in
// TestClientMatrix_OperationsCovered holds the line that every column is
// covered somewhere (by a leg or the conformance floor).
var clientLegs = []clientLeg{
	{
		client: "AWS CLI",
		file:   "awscli_compat_test.go",
		prefix: "TestAWSCLI_",
		gate:   "requireAWSCLI(",
		ops: map[compatOperation][]string{
			// Every aws command is SigV4-signed, so any leg pins
			// credential acceptance; the negative cases belong to the
			// rclone leg and the SDK floor.
			opAuthentication:  {"TestAWSCLI_PutGetRoundTrip"},
			opReads:           {"TestAWSCLI_PutGetRoundTrip", "TestAWSCLI_Sync", "TestAWSCLI_CopyObject"},
			opListing:         {"TestAWSCLI_ListAndDelete", "TestAWSCLI_Sync"},
			opOverwriteDelete: {"TestAWSCLI_ListAndDelete"},
			opMultipart:       {"TestAWSCLI_MultipartUpload"},
			// Range reads: not exercised through the aws CLI in this
			// suite; rclone, boto3, DuckDB and the SDK floor carry the
			// column.
		},
	},
	{
		client: "rclone",
		file:   "rclone_compat_test.go",
		prefix: "TestRclone_",
		gate:   "requireRclone(",
		ops: map[compatOperation][]string{
			opAuthentication:  {"TestRclone_Authentication"},
			opReads:           {"TestRclone_CopyRoundTrip", "TestRclone_SinglePutGet"},
			opRangeReads:      {"TestRclone_RangeReads"},
			opListing:         {"TestRclone_Listing"},
			opOverwriteDelete: {"TestRclone_OverwriteDelete"},
			opMultipart:       {"TestRclone_MultipartUpload"},
		},
	},
	{
		client: "boto3",
		file:   "boto3_compat_test.go",
		prefix: "TestBoto3_",
		driver: "testdata/boto3_leg.py",
		gate:   "requirePythonModule(",
		ops: map[compatOperation][]string{
			// The Go entry runs the whole driver under botocore's real
			// SigV4 signer; the remaining cells bind to the driver's
			// own request shapes.
			opAuthentication:  {"TestBoto3_ClientLeg", `signature_version="s3v4"`},
			opReads:           {"get_object(Bucket=bucket, Key=key)"},
			opRangeReads:      {`Range="bytes=1000-1999"`},
			opListing:         {"list_objects_v2"},
			opOverwriteDelete: {"delete_object", "overwrite.bin"},
			opMultipart:       {"create_multipart_upload", "abort_multipart_upload"},
		},
	},
	{
		client: "DuckDB/httpfs",
		file:   "duckdb_compat_test.go",
		prefix: "TestDuckDB_",
		gate:   "requireClientBin(",
		ops: map[compatOperation][]string{
			// Reading Parquet over s3 is a range workload by
			// construction: footer GET plus per-column-chunk GETs, all
			// SigV4-signed by httpfs.
			opAuthentication: {"TestDuckDB_HTTPFSParquetRangeRead"},
			opReads:          {"TestDuckDB_HTTPFSParquetRangeRead"},
			opRangeReads:     {"TestDuckDB_HTTPFSParquetRangeRead"},
		},
	},
	{
		client: "litestream",
		file:   "litestream_compat_test.go",
		prefix: "TestLitestream_",
		gate:   "requireClientBin(",
		ops: map[compatOperation][]string{
			opAuthentication: {"TestLitestream_ReplicateRestoreRoundTrip"},
			opReads:          {"TestLitestream_ReplicateRestoreRoundTrip"},
		},
	},
	{
		client: "barman-cloud",
		file:   "barman_compat_test.go",
		prefix: "TestBarmanCloud_",
		gate:   "requireClientBin(",
		ops: map[compatOperation][]string{
			// 5 MB tar chunks above barman's --min-chunk-size make the
			// backup a multipart upload with non-block-aligned parts.
			opAuthentication: {"TestBarmanCloud_BackupRestoreNonUniformParts"},
			opReads:          {"TestBarmanCloud_BackupRestoreNonUniformParts"},
			opMultipart:      {"TestBarmanCloud_BackupRestoreNonUniformParts"},
		},
	},
}

// conformanceFloor binds the operation columns to the always-run SDK tests
// (a real SigV4 signer against the same in-process server): the suite's
// teeth on machines without the CLIs, per README's "Client legs and two
// layers".
var conformanceFloor = map[compatOperation][]string{
	opAuthentication: {"TestVerify_Authentication"},
	// Full-object GET byte equality, before and after overwrite.
	opReads:           {"TestVerify_Overwrite"},
	opRangeReads:      {"TestVerify_RangeReads"},
	opListing:         {"TestVerify_List"},
	opOverwriteDelete: {"TestVerify_Overwrite", "TestVerify_Delete"},
	opMultipart:       {"TestVerify_MultipartRoundTrip", "TestVerify_ConcurrentTransfers"},
}

// floorFiles hold the conformance floor. They must contain no presence
// guards: their tests run on every `go test` — including CI's `-short`
// gate — or the floor is not a floor.
var floorFiles = []string{"protocol_conformance_test.go", "zz_verify_sdk_test.go"}

// ---------------------------------------------------------------------------
// source access — the matrix pins package sources, and `go test` runs from
// the package directory, so relative names resolve everywhere the suite
// runs (locally, in CI's -short gate, and in the armor-build image gate).
// ---------------------------------------------------------------------------

func packageSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("client matrix: read %s: %v", name, err)
	}
	return string(data)
}

var testFuncRe = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]+)\(`)

func testFuncs(src string) map[string]bool {
	funcs := make(map[string]bool)
	for _, m := range testFuncRe.FindAllStringSubmatch(src, -1) {
		funcs[m[1]] = true
	}
	return funcs
}

// legBindingsClaimed reports whether name is bound in some cell of leg.
func legBindingsClaimed(leg clientLeg, name string) bool {
	for _, bindings := range leg.ops {
		if slices.Contains(bindings, name) {
			return true
		}
	}
	return false
}

// TestClientMatrix_EveryLegRemainsPresent is the parity pin at the heart of
// the matrix: every binding resolves to real code, every leg-owned test
// function is claimed by a cell, every leg routes through its presence
// guard, and every *_compat_test.go file in the package is a known leg.
func TestClientMatrix_EveryLegRemainsPresent(t *testing.T) {
	for _, leg := range clientLegs {
		src := packageSource(t, leg.file)
		funcs := testFuncs(src)

		var driverSrc string
		if leg.driver != "" {
			driverSrc = packageSource(t, leg.driver)
		}

		for _, op := range sortedOps(leg.ops) {
			for _, binding := range leg.ops[op] {
				if funcs[binding] {
					continue
				}
				if leg.driver != "" && strings.Contains(driverSrc, binding) {
					continue
				}
				t.Errorf("%s (%s): %s is bound to %q, which is neither a Test function in %s nor present in %s — the matrix claims coverage that no longer exists",
					leg.client, leg.file, op, binding, leg.file, leg.driverName())
			}
		}

		for name := range funcs {
			if !strings.HasPrefix(name, leg.prefix) {
				continue
			}
			if !legBindingsClaimed(leg, name) {
				t.Errorf("%s: %s exists but no matrix cell claims it — bind it under an operation (or add the operation) in clientLegs in the same change", leg.file, name)
			}
		}

		if !strings.Contains(src, leg.gate) {
			t.Errorf("%s never calls %s — every leg must route through its presence guard so ARMOR_COMPAT_ENDPOINT mode (the armor-build gate against the published image) fails instead of skipping a missing client", leg.file, leg.gate)
		}
	}

	// A leg file added to the package without a matrix row would be
	// executable-but-undocumented coverage — exactly the drift this
	// pin exists to catch.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("client matrix: read package directory: %v", err)
	}
	known := make(map[string]bool, len(clientLegs))
	for _, leg := range clientLegs {
		known[leg.file] = true
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, "_compat_test.go") || known[name] {
			continue
		}
		t.Errorf("%s is a client-leg file with no clientLegs row in client_matrix_test.go — add the leg (and its README row) in the same change", name)
	}
}

// TestClientMatrix_ConformanceFloorAlwaysRuns pins the SDK layer: every
// floor binding exists, every TestVerify_ test in the floor files is bound,
// and the floor files contain nothing that could skip them.
func TestClientMatrix_ConformanceFloorAlwaysRuns(t *testing.T) {
	funcs := make(map[string]bool)
	for _, file := range floorFiles {
		src := packageSource(t, file)

		for _, guard := range []string{
			"testing.Short()",
			"requireClientBin(",
			"requireAWSCLI(",
			"requireRclone(",
			"requirePythonModule(",
		} {
			if strings.Contains(src, guard) {
				t.Errorf("%s contains %s — the conformance floor must run on every go test, including -short and machines without the clients", file, guard)
			}
		}

		for name := range testFuncs(src) {
			funcs[name] = true
		}
	}

	for _, op := range sortedOps(conformanceFloor) {
		for _, name := range conformanceFloor[op] {
			if !funcs[name] {
				t.Errorf("conformance floor: %s is bound to %q, which does not exist in %v", op, name, floorFiles)
			}
		}
	}

	for name := range funcs {
		if !strings.HasPrefix(name, "TestVerify_") {
			continue
		}
		bound := false
		for _, names := range conformanceFloor {
			if slices.Contains(names, name) {
				bound = true
				break
			}
		}
		if !bound {
			t.Errorf("%s runs in the conformance-floor files but no conformanceFloor cell claims it — bind it in client_matrix_test.go or move it out of the floor files", name)
		}
	}
}

// TestClientMatrix_OperationsCovered holds the "where supported" contract:
// declared cells are known columns, and every column is covered by at least
// one client leg or the conformance floor.
func TestClientMatrix_OperationsCovered(t *testing.T) {
	covered := make(map[compatOperation][]string) // operation -> who covers it
	for op := range conformanceFloor {
		covered[op] = append(covered[op], "conformance floor")
	}
	for _, leg := range clientLegs {
		for _, op := range sortedOps(leg.ops) {
			if !slices.Contains(allOperations, op) {
				t.Errorf("%s binds unknown operation %q — extend allOperations deliberately, in the same change as the documentation", leg.client, op)
			}
			covered[op] = append(covered[op], leg.client)
		}
	}
	for _, op := range allOperations {
		if len(covered[op]) == 0 {
			t.Errorf("operation %q is covered by no client leg and not pinned by the conformance floor", op)
		}
	}

	// Human-readable render (shown on failure and under -v).
	var b strings.Builder
	b.WriteString("executable client matrix:\n")
	for _, leg := range clientLegs {
		var cells []string
		for _, op := range allOperations {
			if bindings, ok := leg.ops[op]; ok {
				cells = append(cells, fmt.Sprintf("%s=%s", op, strings.Join(bindings, "+")))
			}
		}
		fmt.Fprintf(&b, "  %-13s %s\n", leg.client+":", strings.Join(cells, ", "))
	}
	var cells []string
	for _, op := range allOperations {
		cells = append(cells, fmt.Sprintf("%s=%s", op, strings.Join(conformanceFloor[op], "+")))
	}
	fmt.Fprintf(&b, "  %-13s %s (always runs)\n", "sdk floor:", strings.Join(cells, ", "))
	t.Log(b.String())
}

// TestClientMatrix_DocumentedClientsMatch holds the documented client lists
// to the matrix: the README intro, the README client-leg table, and the
// AGENTS.md CI bullet must name exactly the legs that exist.
func TestClientMatrix_DocumentedClientsMatch(t *testing.T) {
	want := make([]string, 0, len(clientLegs))
	for _, leg := range clientLegs {
		want = append(want, leg.client)
	}
	slices.Sort(want)

	for _, doc := range []struct {
		name string
		got  []string
	}{
		{"tests/aws-cli-compatibility/README.md intro", readmeIntroClients(t)},
		{"AGENTS.md CI bullet", agentsClaimedClients(t)},
	} {
		got := slices.Clone(doc.got)
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s names clients %v, but the executable matrix holds %v — keep the documented list and clientLegs in the same change", doc.name, got, want)
		}
	}

	// The client-leg table is the README's per-file inventory: rows may
	// only name known layers, and every leg and floor file must have a
	// row.
	known := map[string]bool{"client_matrix_test.go": true}
	for _, leg := range clientLegs {
		known[leg.file] = true
	}
	for _, file := range floorFiles {
		known[file] = true
	}
	rows := readmeTableFiles(t)
	for _, file := range rows {
		if !known[file] {
			t.Errorf("README.md's client-leg table has a row for %q, which is neither a client leg, a conformance-floor file, nor client_matrix_test.go — extend clientLegs in the same change", file)
		}
	}
	for _, file := range slices.Concat(
		mapLegFiles(clientLegs),
		floorFiles,
		[]string{"client_matrix_test.go"},
	) {
		if !slices.Contains(rows, file) {
			t.Errorf("%s has no row in README.md's client-leg table — the documented inventory and the executable matrix must move together", file)
		}
	}
}

// ---------------------------------------------------------------------------
// documented-inventory parsers. Each fails loudly when its surface is
// reworded, so a doc change consciously updates the matrix (and vice
// versa) rather than drifting.
// ---------------------------------------------------------------------------

// documentedClientList parses "the AWS CLI, rclone, ..., and barman-cloud"
// into canonical client names. The documented sentences wrap across lines,
// so whitespace is collapsed first.
func documentedClientList(t *testing.T, fragment string) []string {
	t.Helper()
	fragment = strings.Join(strings.Fields(fragment), " ")
	fragment = strings.TrimSuffix(strings.TrimSpace(fragment), ".")
	var names []string
	for _, name := range strings.Split(fragment, ",") {
		name = strings.TrimSpace(name)
		name = strings.TrimPrefix(name, "the ")
		name = strings.TrimPrefix(name, "and ")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func readmeIntroClients(t *testing.T) []string {
	t.Helper()
	const marker = "through ARMOR: "
	readme := packageSource(t, "README.md")
	i := strings.Index(readme, marker)
	if i < 0 {
		t.Fatalf("README.md no longer introduces the client list after %q — rewording it must update the marker in client_matrix_test.go", marker)
	}
	rest := readme[i+len(marker):]
	end := strings.Index(rest, ".")
	if end < 0 {
		t.Fatalf("README.md's client-list sentence never terminates")
	}
	return documentedClientList(t, rest[:end])
}

// The AGENTS.md bullet wraps across lines, so tolerate the whitespace
// between "suite" and the parenthesized client list.
var agentsClientClaim = regexp.MustCompile(`real-client compatibility suite\s*\(([^)]+)\)`)

func agentsClaimedClients(t *testing.T) []string {
	t.Helper()
	agents, err := os.ReadFile("../../AGENTS.md")
	if err != nil {
		t.Fatalf("client matrix: read AGENTS.md: %v", err)
	}
	m := agentsClientClaim.FindSubmatch(agents)
	if m == nil {
		t.Fatalf("AGENTS.md no longer names the clients in \"real-client compatibility suite (...)\" — rewording it must update the pattern in client_matrix_test.go")
	}
	return documentedClientList(t, string(m[1]))
}

var readmeTableRowFile = regexp.MustCompile("(?m)^\\|.*?`([a-z0-9_]+\\.go)`")

func readmeTableFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, m := range readmeTableRowFile.FindAllStringSubmatch(packageSource(t, "README.md"), -1) {
		if !slices.Contains(files, m[1]) {
			files = append(files, m[1])
		}
	}
	return files
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

// driverName renders the leg's driver for error messages.
func (l clientLeg) driverName() string {
	if l.driver == "" {
		return "(no driver)"
	}
	return l.driver
}

func sortedOps(ops map[compatOperation][]string) []compatOperation {
	keys := make([]compatOperation, 0, len(ops))
	for op := range ops {
		keys = append(keys, op)
	}
	slices.Sort(keys)
	return keys
}

func mapLegFiles(legs []clientLeg) []string {
	files := make([]string, 0, len(legs))
	for _, leg := range legs {
		files = append(files, leg.file)
	}
	return files
}
