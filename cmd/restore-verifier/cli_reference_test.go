//go:build !integration
// +build !integration

// Smoke coverage for the companion-binary CLI reference
// (docs/companion-cli-reference.md), restore-verifier half. The document is
// the operator contract for this binary; these tests keep it honest against
// the binary itself: every registered flag is documented and nothing else,
// every environment variable the document names is read by the
// implementation, the documented endpoint derivation holds, and the built
// binary behaves as documented on the version, help, usage-error, startup-
// validation, HTTP-surface, and graceful-shutdown paths. Without this, a
// renamed flag or a changed exit code ships with a reference page that no
// longer describes the binary. The armor-fleet half lives in
// cmd/armor-fleet.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// companionReferencePath resolves docs/companion-cli-reference.md from this
// file's location, so the test finds the document regardless of the working
// directory go test was invoked from.
func companionReferencePath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	// thisFile is cmd/restore-verifier/cli_reference_test.go; the repo root
	// is two up.
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))),
		"docs", "companion-cli-reference.md")
}

// companionSectionRe matches the reference's per-binary headings:
// ## `restore-verifier` and ## `armor-fleet`.
var companionSectionRe = regexp.MustCompile("(?m)^## `(restore-verifier|armor-fleet)`\\s*$")

// companionBacktickSpan matches a `...` code span anywhere in a section.
var companionBacktickSpan = regexp.MustCompile("`([^`\n]*)`")

// companionEnvName matches an environment variable literal this reference
// documents. The fleet console's SEAM_TOKEN is covered too; the whole
// document is checked so a rename anywhere fails here, not in an operator's
// terminal.
var companionEnvName = regexp.MustCompile("^(ARMOR|VERIFIER|SEAM)_[A-Z0-9_]+$")

// loadCompanionReference reads the reference document.
func loadCompanionReference(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(companionReferencePath(t))
	if err != nil {
		t.Fatalf("read companion CLI reference: %v", err)
	}
	return string(raw)
}

// companionSection returns the document's section for the named binary,
// from its `## <name>` heading to the next binary heading (or EOF).
func companionSection(t *testing.T, doc, name string) string {
	t.Helper()
	matches := companionSectionRe.FindAllStringSubmatchIndex(doc, -1)
	for i, m := range matches {
		if doc[m[2]:m[3]] != name {
			continue
		}
		end := len(doc)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		return doc[m[1]:end]
	}
	t.Fatalf("docs/companion-cli-reference.md has no `## %s` section; the heading format the smoke tests match is `## <binary>`", name)
	return ""
}

// companionDocumentedFlags extracts the flag names a section documents:
// every backticked token that starts with a dash, dashes stripped. -h/--help
// are the universal help path and -v/--version is restore-verifier's
// pre-parse version path — none are registered flags — and tokens with
// interior spaces or slashes are prose, not flags.
func companionDocumentedFlags(section string) map[string]bool {
	names := make(map[string]bool)
	for _, m := range companionBacktickSpan.FindAllStringSubmatch(section, -1) {
		tok := strings.TrimSpace(m[1])
		if !strings.HasPrefix(tok, "-") {
			continue
		}
		name := strings.TrimLeft(tok, "-")
		if name == "" || name == "h" || name == "help" || name == "v" || name == "version" ||
			strings.ContainsAny(name, " \t/") {
			continue
		}
		names[name] = true
	}
	return names
}

// companionRegisteredFlags collects the flags this binary registers on the
// command line — the authoritative set its usage prints. The test binary's
// own test.* flags share the same FlagSet; they contain a dot and the
// binary's flags never do, so the dot is the filter.
func companionRegisteredFlags() map[string]bool {
	names := make(map[string]bool)
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if strings.Contains(f.Name, ".") {
			return
		}
		names[f.Name] = true
	})
	return names
}

// TestCLIReferenceFlagsMatchRegistry pins the documented flag list to the
// binary's registry, in both directions: a flag the binary registers must be
// documented, and a flag the document names must exist in the registry
// (renames and typo'd flags fail here, not in an operator's terminal).
func TestCLIReferenceFlagsMatchRegistry(t *testing.T) {
	doc := loadCompanionReference(t)
	section := companionSection(t, doc, "restore-verifier")
	documented := companionDocumentedFlags(section)
	registered := companionRegisteredFlags()

	var missing, phantom []string
	for f := range registered {
		if !documented[f] {
			missing = append(missing, f)
		}
	}
	for f := range documented {
		if !registered[f] {
			phantom = append(phantom, f)
		}
	}
	sort.Strings(missing)
	sort.Strings(phantom)
	if len(missing) > 0 {
		t.Errorf("registered flags missing from docs/companion-cli-reference.md: %q", missing)
	}
	if len(phantom) > 0 {
		t.Errorf("flags documented in docs/companion-cli-reference.md that the binary does not register: %q", phantom)
	}
}

// TestCLIReferenceEnvVarNamesExist: every ARMOR_/VERIFIER_/SEAM_ variable the
// reference names must be read somewhere in the implementation — a renamed
// or removed variable would leave the reference pointing operators at an
// env var that does nothing.
func TestCLIReferenceEnvVarNamesExist(t *testing.T) {
	doc := loadCompanionReference(t)
	documented := map[string]bool{}
	for _, m := range companionBacktickSpan.FindAllStringSubmatch(doc, -1) {
		name := strings.TrimSpace(m[1])
		if companionEnvName.MatchString(name) {
			documented[name] = true
		}
	}
	if len(documented) == 0 {
		t.Fatal("docs/companion-cli-reference.md names no environment variables")
	}

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	source := map[string]bool{}
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".beads", "bin", "node_modules", "testdata":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for envName := range documented {
			if bytes.Contains(raw, []byte(envName)) {
				source[envName] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning sources for environment variables: %v", err)
	}

	var missing []string
	for name := range documented {
		if !source[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("environment variables documented in docs/companion-cli-reference.md but read nowhere in the implementation: %q", missing)
	}
}

// TestDerivedB2Endpoint pins the documented endpoint derivation: an explicit
// endpoint wins, otherwise the region derives
// https://s3.<region>.backblazeb2.com, and with neither the endpoint stays
// empty (startup then fails on the missing-credential validation).
func TestDerivedB2Endpoint(t *testing.T) {
	cases := []struct {
		region, endpoint, want string
	}{
		{region: "us-west-004", endpoint: "", want: "https://s3.us-west-004.backblazeb2.com"},
		{region: "us-west-004", endpoint: "http://127.0.0.1:9", want: "http://127.0.0.1:9"},
		{region: "", endpoint: "", want: ""},
	}
	for _, tc := range cases {
		if got := deriveB2Endpoint(tc.region, tc.endpoint); got != tc.want {
			t.Errorf("deriveB2Endpoint(%q, %q) = %q, want %q", tc.region, tc.endpoint, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Executable parity: the reference versus the real binary.
// ---------------------------------------------------------------------------

// versionLineRe matches the version line the reference documents:
// `restore-verifier <version> (go<goversion>, <os>/<arch>)`.
var verifierVersionLineRe = regexp.MustCompile(`^restore-verifier \S+ \(go[^,]+, [^/)]+/[^)]+\)$`)

var (
	verifierBinOnce sync.Once
	verifierBinPath string
	verifierBinErr  error
	verifierBinDir  string
)

// verifierBinary builds the restore-verifier binary once per test run and
// returns its path. The toolchain is the one running these tests: `go` from
// PATH, or GOROOT/bin/go — the running toolchain itself — when `go` is not
// on PATH.
func verifierBinary(t *testing.T) string {
	t.Helper()
	verifierBinOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			verifierBinErr = errors.New("runtime.Caller could not locate this test file")
			return
		}
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		dir, err := os.MkdirTemp("", "restore-verifier-reference-bin-")
		if err != nil {
			verifierBinErr = fmt.Errorf("temp dir for reference binary: %w", err)
			return
		}
		verifierBinDir = dir
		goBin, err := exec.LookPath("go")
		if err != nil {
			goBin = filepath.Join(runtime.GOROOT(), "bin", "go")
		}
		bin := filepath.Join(dir, "restore-verifier")
		build := exec.Command(goBin, "build", "-o", bin, ".")
		build.Dir = filepath.Join(repoRoot, "cmd", "restore-verifier")
		if out, buildErr := build.CombinedOutput(); buildErr != nil {
			verifierBinErr = fmt.Errorf("build reference binary: %w\n%s", buildErr, out)
			return
		}
		verifierBinPath = bin
	})
	if verifierBinErr != nil {
		t.Fatalf("%v", verifierBinErr)
	}
	return verifierBinPath
}

// verifierReferenceEnv returns the subprocess environment with every
// ARMOR_*/VERIFIER_* variable (and the console's SEAM_TOKEN) stripped, so
// the documented validation paths are exercised against a clean slate
// regardless of what the surrounding environment sets; overrides are
// appended last.
func verifierReferenceEnv(t *testing.T, overrides ...string) []string {
	t.Helper()
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ARMOR_") || strings.HasPrefix(kv, "VERIFIER_") || strings.HasPrefix(kv, "SEAM_TOKEN=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, overrides...)
}

// verifierRun is one executed invocation of the reference binary.
type verifierRun struct {
	stdout, stderr string
	exitCode       int
}

// runVerifier executes the built binary with a 60-second ceiling, so a
// regression that turns a documented validation error into a long-running
// process fails loudly instead of hanging the suite.
func runVerifier(t *testing.T, env []string, args ...string) verifierRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, verifierBinary(t), args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := verifierRun{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("restore-verifier %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
		}
		res.exitCode = exitErr.ExitCode()
	}
	return res
}

// TestCLIReferenceVersionHelpAndUsageMatchBinary pins the documented global
// paths: the version and help paths exit 0 with the documented output, and
// an undefined flag is a usage error exiting 2 with the diagnostic.
func TestCLIReferenceVersionHelpAndUsageMatchBinary(t *testing.T) {
	env := verifierReferenceEnv(t)

	t.Run("version paths exit 0 with the documented line", func(t *testing.T) {
		for _, args := range [][]string{{"--version"}, {"-v"}} {
			res := runVerifier(t, env, args...)
			if res.exitCode != 0 {
				t.Errorf("restore-verifier %s: exit = %d, want 0\nstderr: %s", strings.Join(args, " "), res.exitCode, res.stderr)
				continue
			}
			if line := strings.TrimSpace(res.stdout); !verifierVersionLineRe.MatchString(line) {
				t.Errorf("restore-verifier %s: stdout %q does not match the documented `restore-verifier <version> (go<goversion>, <os>/<arch>)` shape", strings.Join(args, " "), line)
			}
		}
	})

	t.Run("help paths exit 0 with the usage text", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"-h"}} {
			res := runVerifier(t, env, args...)
			if res.exitCode != 0 {
				t.Errorf("restore-verifier %s: exit = %d, want 0\nstderr: %s", strings.Join(args, " "), res.exitCode, res.stderr)
				continue
			}
			for _, want := range []string{"Continuous backup verification", "Bucket Configuration", "HTTP Endpoints"} {
				if !strings.Contains(res.stderr, want) {
					t.Errorf("restore-verifier %s: usage output missing %q:\n%s", strings.Join(args, " "), want, res.stderr)
				}
			}
		}
	})

	t.Run("undefined flag exits 2 with the usage", func(t *testing.T) {
		res := runVerifier(t, env, "--definitely-not-a-flag")
		if res.exitCode != 2 {
			t.Errorf("undefined flag: exit = %d, want 2", res.exitCode)
		}
		if !strings.Contains(res.stderr, "flag provided but not defined") {
			t.Errorf("undefined flag: stderr missing the diagnostic:\n%s", res.stderr)
		}
		if !strings.Contains(res.stderr, "Usage:") {
			t.Errorf("undefined flag: stderr missing the usage text:\n%s", res.stderr)
		}
	})
}

// TestCLIReferenceStartupValidationExitsOne pins the documented startup
// validation exits: every configuration failure the reference names is
// fatal with exit 1 and a log line naming the problem. The environment is
// stripped, so every case here starts from zero and adds only what it names.
func TestCLIReferenceStartupValidationExitsOne(t *testing.T) {
	const (
		validMEK   = "abababababababababababababababababababababababababababababababab" // 32 bytes of hex
		credEnv    = "ARMOR_B2_REGION=smoke-region,ARMOR_B2_ENDPOINT=http://127.0.0.1:9,ARMOR_B2_ACCESS_KEY_ID=smoke-key,ARMOR_B2_SECRET_ACCESS_KEY=smoke-secret"
		bucketFlag = "-bucket=smoke-bucket"
	)

	cases := []struct {
		name       string
		env        []string
		args       []string
		wantStderr []string
	}{
		{
			name:       "no configuration at all",
			env:        nil,
			wantStderr: []string{"Missing required B2 credentials"},
		},
		{
			name:       "credentials but no MEK",
			env:        strings.Split(credEnv, ","),
			wantStderr: []string{"Missing MEK"},
		},
		{
			name:       "MEK that is not hex",
			env:        append(strings.Split(credEnv, ","), "ARMOR_MEK=not-hex-at-all"),
			args:       []string{bucketFlag},
			wantStderr: []string{"Invalid MEK hex"},
		},
		{
			name:       "MEK of the wrong length",
			env:        append(strings.Split(credEnv, ","), "ARMOR_MEK=abab"),
			args:       []string{bucketFlag},
			wantStderr: []string{"Invalid MEK length"},
		},
		{
			name: "invalid MEK ring entry",
			env: append(strings.Split(credEnv, ","), "ARMOR_MEK="+validMEK,
				"VERIFIER_MEK_RING=zzzz"),
			args:       []string{bucketFlag},
			wantStderr: []string{"Invalid MEK ring entry"},
		},
		{
			name:       "valid credentials and MEK but no bucket scope",
			env:        append(strings.Split(credEnv, ","), "ARMOR_MEK="+validMEK),
			wantStderr: []string{"No buckets configured"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runVerifier(t, verifierReferenceEnv(t, tc.env...), tc.args...)
			if res.exitCode != 1 {
				t.Errorf("restore-verifier %s: exit = %d, want 1\nstdout: %s\nstderr: %s",
					tc.name, res.exitCode, res.stdout, res.stderr)
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(res.stderr, want) {
					t.Errorf("restore-verifier %s: stderr missing %q:\n%s", tc.name, want, res.stderr)
				}
			}
		})
	}
}

// freePort hands back a loopback address that was free a moment ago: bind,
// read the chosen port, close. The tiny race with other listeners is
// acceptable for a smoke address.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// TestCLIReferenceStartupServesHTTPAndShutsDownCleanly is the startup smoke
// the reference documents: with valid-looking configuration the binary
// starts (offline — the backend endpoint points at a closed local port), the
// documented HTTP surface answers, and SIGTERM produces the documented
// graceful shutdown with exit 0.
func TestCLIReferenceStartupServesHTTPAndShutsDownCleanly(t *testing.T) {
	addr := freePort(t)
	env := verifierReferenceEnv(t,
		"ARMOR_B2_REGION=smoke-region",
		"ARMOR_B2_ENDPOINT=http://127.0.0.1:9", // closed port: offline, fast failure
		"ARMOR_B2_ACCESS_KEY_ID=smoke-key",
		"ARMOR_B2_SECRET_ACCESS_KEY=smoke-secret",
		"ARMOR_MEK=abababababababababababababababababababababababababababababababab",
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, verifierBinary(t),
		"-bucket", "smoke-bucket",
		"-http-listen", addr,
		"-check-interval", "1h",
	)
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start restore-verifier: %v", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	// Wait for the listener the reference promises.
	base := "http://" + addr
	deadline := time.Now().Add(30 * time.Second)
	for {
		resp, err := http.Get(base + "/status")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("restore-verifier never opened its HTTP listener on %s\nstderr so far:\n%s", addr, stderr.String())
		}
		time.Sleep(100 * time.Millisecond)
	}

	// GET /status: 200 with a JSON object.
	resp, err := http.Get(base + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /status = %d, want 200", resp.StatusCode)
	}
	var statusObj map[string]any
	if err := json.Unmarshal(body, &statusObj); err != nil {
		t.Errorf("GET /status body is not a JSON object: %v\nbody: %s", err, body)
	}

	// GET /bucket: the documented parameter handling.
	resp, err = http.Get(base + "/bucket")
	if err != nil {
		t.Fatalf("GET /bucket: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("GET /bucket without a parameter = %d, want 400", resp.StatusCode)
	}
	resp, err = http.Get(base + "/bucket?bucket=smoke-bucket")
	if err != nil {
		t.Fatalf("GET /bucket?bucket=smoke-bucket: %v", err)
	}
	resp.Body.Close()
	// 200 once the startup run has recorded state, 404 before it — both are
	// documented; anything else means the route or handler drifted.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /bucket?bucket=smoke-bucket = %d, want 200 or 404", resp.StatusCode)
	}

	// Liveness and readiness answer with exactly the documented pair of codes.
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, want 200 or 503", path, resp.StatusCode)
		}
	}

	// GET /metrics: 200, Prometheus text with the restore-verifier series.
	resp, err = http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /metrics = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "armor_restore_verifier_") {
		t.Errorf("GET /metrics body carries no armor_restore_verifier_ series:\n%s", body)
	}

	// POST /trigger: the documented accept/reject matrix.
	trigger := func(query string) (int, string) {
		resp, err := http.Post(base+"/trigger"+query, "", nil)
		if err != nil {
			t.Fatalf("POST /trigger%s: %v", query, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp.StatusCode, string(body)
	}
	if code, got := trigger(""); code != http.StatusAccepted || !strings.Contains(got, "Verification triggered") {
		t.Errorf("POST /trigger = %d %q, want 202 with \"Verification triggered\"", code, got)
	}
	if code, got := trigger("?mode=dual"); code != http.StatusAccepted {
		t.Errorf("POST /trigger?mode=dual = %d %q, want 202", code, got)
	}
	if code, got := trigger("?mode=dr-drill"); code != http.StatusAccepted || !strings.Contains(got, "DR-drill") {
		t.Errorf("POST /trigger?mode=dr-drill = %d %q, want 202 with a DR-drill confirmation", code, got)
	}
	if code, got := trigger("?mode=bogus"); code != http.StatusBadRequest || !strings.Contains(got, "unknown mode") {
		t.Errorf("POST /trigger?mode=bogus = %d %q, want 400 naming the unknown mode", code, got)
	}

	// SIGTERM: the documented graceful shutdown, exit 0.
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}
	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()
	select {
	case err := <-waitErr:
		stopped = true
		// Wait returning nil IS exit 0; only an ExitError carries a
		// non-zero code.
		code := 0
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("wait for restore-verifier: %v", err)
		}
		if code != 0 {
			t.Errorf("restore-verifier after SIGTERM: exit = %d, want 0\nstderr:\n%s", code, stderr.String())
		}
	case <-time.After(45 * time.Second):
		t.Fatalf("restore-verifier did not shut down within 45s of SIGTERM\nstderr:\n%s", stderr.String())
	}

	for _, want := range []string{
		"HTTP server listening on " + addr,
		"Restore-verifier stopped gracefully",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
