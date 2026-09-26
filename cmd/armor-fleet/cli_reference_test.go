//go:build !integration
// +build !integration

// Smoke coverage for the companion-binary CLI reference
// (docs/companion-cli-reference.md), armor-fleet half. The document is the
// operator contract for this binary; these tests keep it honest against the
// binary itself: every registered flag is documented and nothing else, the
// SEAM_TOKEN environment variable the document names is read by the
// implementation, and the built binary behaves as documented on the help,
// usage-error, targets-validation, HTTP-surface, and graceful-shutdown
// paths. Without this, a renamed flag or a changed exit code ships with a
// reference page that no longer describes the binary. The restore-verifier
// half lives in cmd/restore-verifier.
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

// fleetReferencePath resolves docs/companion-cli-reference.md from this
// file's location, so the test finds the document regardless of the working
// directory go test was invoked from.
func fleetReferencePath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	// thisFile is cmd/armor-fleet/cli_reference_test.go; the repo root is
	// two up.
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))),
		"docs", "companion-cli-reference.md")
}

// fleetSectionRe matches the reference's per-binary headings:
// ## `restore-verifier` and ## `armor-fleet`.
var fleetSectionRe = regexp.MustCompile("(?m)^## `(restore-verifier|armor-fleet)`\\s*$")

// fleetBacktickSpan matches a `...` code span anywhere in a section.
var fleetBacktickSpan = regexp.MustCompile("`([^`\n]*)`")

// fleetEnvName matches an environment variable literal in the reference.
var fleetEnvName = regexp.MustCompile("^(ARMOR|VERIFIER|SEAM)_[A-Z0-9_]+$")

// loadFleetReference reads the reference document.
func loadFleetReference(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(fleetReferencePath(t))
	if err != nil {
		t.Fatalf("read companion CLI reference: %v", err)
	}
	return string(raw)
}

// fleetSection returns the document's section for the named binary, from
// its `## <name>` heading to the next binary heading (or EOF).
func fleetSection(t *testing.T, doc, name string) string {
	t.Helper()
	matches := fleetSectionRe.FindAllStringSubmatchIndex(doc, -1)
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

// fleetDocumentedFlags extracts the flag names a section documents: every
// backticked token that starts with a dash, dashes stripped. -h/--help are
// the universal help path, not registered flags, and tokens with interior
// spaces or slashes are prose, not flags.
func fleetDocumentedFlags(section string) map[string]bool {
	names := make(map[string]bool)
	for _, m := range fleetBacktickSpan.FindAllStringSubmatch(section, -1) {
		tok := strings.TrimSpace(m[1])
		if !strings.HasPrefix(tok, "-") {
			continue
		}
		name := strings.TrimLeft(tok, "-")
		if name == "" || name == "h" || name == "help" || strings.ContainsAny(name, " \t/") {
			continue
		}
		names[name] = true
	}
	return names
}

// fleetRegisteredFlags collects the flags this binary registers on the
// command line — the authoritative set its usage prints. The test binary's
// own test.* flags share the same FlagSet; they contain a dot and the
// binary's flags never do, so the dot is the filter.
func fleetRegisteredFlags() map[string]bool {
	names := make(map[string]bool)
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		if strings.Contains(f.Name, ".") {
			return
		}
		names[f.Name] = true
	})
	return names
}

// TestFleetCLIReferenceFlagsMatchRegistry pins the documented flag list to
// the binary's registry, in both directions: a flag the binary registers
// must be documented, and a flag the document names must exist in the
// registry (renames and typo'd flags fail here, not in an operator's
// terminal).
func TestFleetCLIReferenceFlagsMatchRegistry(t *testing.T) {
	doc := loadFleetReference(t)
	section := fleetSection(t, doc, "armor-fleet")
	documented := fleetDocumentedFlags(section)
	registered := fleetRegisteredFlags()

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

// TestFleetCLIReferenceEnvVarNamesExist: every ARMOR_/SEAM_ variable the
// armor-fleet section names must be read somewhere in the implementation —
// a renamed or removed variable would leave the reference pointing
// operators at an env var that does nothing.
func TestFleetCLIReferenceEnvVarNamesExist(t *testing.T) {
	doc := loadFleetReference(t)
	section := fleetSection(t, doc, "armor-fleet")
	documented := map[string]bool{}
	for _, m := range fleetBacktickSpan.FindAllStringSubmatch(section, -1) {
		name := strings.TrimSpace(m[1])
		if fleetEnvName.MatchString(name) {
			documented[name] = true
		}
	}
	if len(documented) == 0 {
		t.Fatal("docs/companion-cli-reference.md armor-fleet section names no environment variables")
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

// ---------------------------------------------------------------------------
// Executable parity: the reference versus the real binary.
// ---------------------------------------------------------------------------

var (
	fleetBinOnce sync.Once
	fleetBinPath string
	fleetBinErr  error
	fleetBinDir  string
)

// fleetBinary builds the armor-fleet binary once per test run and returns
// its path. The toolchain is the one running these tests: `go` from PATH,
// or GOROOT/bin/go — the running toolchain itself — when `go` is not on
// PATH.
func fleetBinary(t *testing.T) string {
	t.Helper()
	fleetBinOnce.Do(func() {
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			fleetBinErr = errors.New("runtime.Caller could not locate this test file")
			return
		}
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		dir, err := os.MkdirTemp("", "armor-fleet-reference-bin-")
		if err != nil {
			fleetBinErr = fmt.Errorf("temp dir for reference binary: %w", err)
			return
		}
		fleetBinDir = dir
		goBin, err := exec.LookPath("go")
		if err != nil {
			goBin = filepath.Join(runtime.GOROOT(), "bin", "go")
		}
		bin := filepath.Join(dir, "armor-fleet")
		build := exec.Command(goBin, "build", "-o", bin, ".")
		build.Dir = filepath.Join(repoRoot, "cmd", "armor-fleet")
		if out, buildErr := build.CombinedOutput(); buildErr != nil {
			fleetBinErr = fmt.Errorf("build reference binary: %w\n%s", buildErr, out)
			return
		}
		fleetBinPath = bin
	})
	if fleetBinErr != nil {
		t.Fatalf("%v", fleetBinErr)
	}
	return fleetBinPath
}

// fleetReferenceEnv returns the subprocess environment with SEAM_TOKEN
// stripped, so the documented token-validation paths are exercised against
// a clean slate regardless of what the surrounding environment sets;
// overrides are appended last.
func fleetReferenceEnv(t *testing.T, overrides ...string) []string {
	t.Helper()
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SEAM_TOKEN=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, overrides...)
}

// runFleet executes the built binary with a 60-second ceiling, so a
// regression that turns a documented usage error into a long-running
// process fails loudly instead of hanging the suite.
func runFleet(t *testing.T, env []string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, fleetBinary(t), args...)
	cmd.Env = env
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("armor-fleet %s: %v\nstderr: %s", strings.Join(args, " "), err, errBuf.String())
		}
		exitCode = exitErr.ExitCode()
	}
	return out.String(), errBuf.String(), exitCode
}

// fleetSmokeTargets writes a valid one-target targets file and returns its
// path.
func fleetSmokeTargets(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.yaml")
	const content = `- name: console-smoke-target
  cluster: smoke-cluster
  namespace: smoke-ns
  service: armor
  admin_port: 9001
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write targets file: %v", err)
	}
	return path
}

// TestFleetCLIReferenceUsageAndValidationMatchBinary pins the documented
// exit codes: the help path exits 0, a missing required flag or credential
// is a usage error (2), and every targets-file failure the reference names
// exits 1 with the diagnostic it documents.
func TestFleetCLIReferenceUsageAndValidationMatchBinary(t *testing.T) {
	env := fleetReferenceEnv(t)

	t.Run("help paths exit 0 with the flag defaults", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"-h"}} {
			_, stderr, code := runFleet(t, env, args...)
			if code != 0 {
				t.Errorf("armor-fleet %s: exit = %d, want 0\nstderr: %s", strings.Join(args, " "), code, stderr)
				continue
			}
			if !strings.Contains(stderr, "Usage of") {
				t.Errorf("armor-fleet %s: stderr missing the usage header:\n%s", strings.Join(args, " "), stderr)
			}
			for _, f := range []string{"-targets", "-listen", "-interval", "-seam-token"} {
				if !strings.Contains(stderr, f) {
					t.Errorf("armor-fleet %s: usage output missing -%s:\n%s", strings.Join(args, " "), f, stderr)
				}
			}
		}
	})

	t.Run("missing -targets exits 2", func(t *testing.T) {
		_, stderr, code := runFleet(t, env)
		if code != 2 {
			t.Errorf("no flags: exit = %d, want 2", code)
		}
		if !strings.Contains(stderr, "-targets flag is required") {
			t.Errorf("no flags: stderr missing the diagnostic:\n%s", stderr)
		}
	})

	t.Run("valid targets without a SEAM token exits 2", func(t *testing.T) {
		_, stderr, code := runFleet(t, env, "-targets", fleetSmokeTargets(t))
		if code != 2 {
			t.Errorf("no token: exit = %d, want 2", code)
		}
		if !strings.Contains(stderr, "SEAM token required") {
			t.Errorf("no token: stderr missing the diagnostic:\n%s", stderr)
		}
	})

	t.Run("undefined flag exits 2", func(t *testing.T) {
		_, stderr, code := runFleet(t, env, "--definitely-not-a-flag")
		if code != 2 {
			t.Errorf("undefined flag: exit = %d, want 2", code)
		}
		if !strings.Contains(stderr, "flag provided but not defined") {
			t.Errorf("undefined flag: stderr missing the diagnostic:\n%s", stderr)
		}
	})

	t.Run("targets-file failures exit 1", func(t *testing.T) {
		cases := []struct {
			name       string
			yaml       string
			wantStderr []string
		}{
			{
				name:       "unreadable targets file",
				yaml:       "", // no file written at all
				wantStderr: []string{"Error loading targets", "read file"},
			},
			{
				name:       "empty targets file",
				yaml:       "",
				wantStderr: []string{"no targets found"},
			},
			{
				name: "entry missing a required field",
				yaml: `- name: broken
  namespace: ns
  service: armor
  admin_port: 9001
`,
				wantStderr: []string{"cluster is required"},
			},
			{
				name:       "unparseable YAML",
				yaml:       "{{{ not yaml",
				wantStderr: []string{"parse YAML"},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "targets.yaml")
				if tc.yaml != "" || tc.name == "empty targets file" {
					if err := os.WriteFile(path, []byte(tc.yaml), 0o644); err != nil {
						t.Fatalf("write targets file: %v", err)
					}
				}
				_, stderr, code := runFleet(t, env, "-targets", path, "-seam-token", "smoke-fake-token")
				if code != 1 {
					t.Errorf("%s: exit = %d, want 1\nstderr: %s", tc.name, code, stderr)
				}
				for _, want := range tc.wantStderr {
					if !strings.Contains(stderr, want) {
						t.Errorf("%s: stderr missing %q:\n%s", tc.name, want, stderr)
					}
				}
			})
		}
	})
}

// fleetFreePort hands back a loopback address that was free a moment ago:
// bind, read the chosen port, close. The tiny race with other listeners is
// acceptable for a smoke address.
func fleetFreePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().String()
}

// TestFleetCLIReferenceStartupServesConsoleAndShutsDownCleanly is the
// startup smoke the reference documents: with a valid targets file and a
// token the binary loads the targets, runs its first poll (the fake token
// cannot reach SEAM, which exercises the documented failed-poll-is-not-
// fatal path), serves the console, and exits 0 on SIGTERM.
func TestFleetCLIReferenceStartupServesConsoleAndShutsDownCleanly(t *testing.T) {
	addr := fleetFreePort(t)
	targets := fleetSmokeTargets(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, fleetBinary(t),
		"-targets", targets,
		"-listen", addr,
		"-interval", "3600",
		"-seam-token", "smoke-fake-token",
	)
	cmd.Env = fleetReferenceEnv(t)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start armor-fleet: %v", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}()

	// The listener binds only after the synchronous first poll, which talks
	// to the real SEAM host with a fake token; give the documented
	// per-request timeout (10s) plus DNS slack before giving up.
	base := "http://" + addr
	deadline := time.Now().Add(60 * time.Second)
	for {
		resp, err := http.Get(base + "/fleet.json")
		if err == nil {
			resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("armor-fleet never opened its HTTP listener on %s\nstderr so far:\n%s", addr, stderr.String())
		}
		time.Sleep(200 * time.Millisecond)
	}

	// GET /: the documented dashboard HTML.
	resp, err := http.Get(base + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	page, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(page), "ARMOR Fleet Dashboard") {
		t.Errorf("GET / body is not the fleet dashboard:\n%.200s", page)
	}

	// GET /fleet.json: JSON keyed by target name, with the failed first
	// poll recorded — not fatal, and not hidden.
	resp, err = http.Get(base + "/fleet.json")
	if err != nil {
		t.Fatalf("GET /fleet.json: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /fleet.json = %d, want 200", resp.StatusCode)
	}
	var fleet map[string]struct {
		Name      string `json:"name"`
		Reachable bool   `json:"reachable"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal(body, &fleet); err != nil {
		t.Fatalf("GET /fleet.json body is not the documented JSON object: %v\nbody: %s", err, body)
	}
	entry, ok := fleet["console-smoke-target"]
	if !ok {
		t.Errorf("GET /fleet.json is missing the configured target; got keys of body: %s", body)
	} else {
		if entry.Name != "console-smoke-target" {
			t.Errorf("fleet.json entry name = %q, want the targets-file name", entry.Name)
		}
		// The first poll used a fake token: unreachable with the poll error
		// recorded is exactly the documented failed-poll behavior.
		if entry.Reachable {
			t.Error("fleet.json entry reports the fake-token poll as reachable; the smoke target must stay unreachable")
		}
		if entry.Error == "" {
			t.Error("fleet.json entry carries no poll error for the failed first poll")
		}
	}

	// GET /metrics: the two documented fleet gauges are registered.
	resp, err = http.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /metrics = %d, want 200", resp.StatusCode)
	}
	for _, gauge := range []string{"armor_fleet_up_targets", "armor_fleet_down_targets"} {
		if !strings.Contains(string(metrics), gauge) {
			t.Errorf("GET /metrics is missing the documented %s gauge:\n%s", gauge, metrics)
		}
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
			t.Fatalf("wait for armor-fleet: %v", err)
		}
		if code != 0 {
			t.Errorf("armor-fleet after SIGTERM: exit = %d, want 0\nstderr:\n%s", code, stderr.String())
		}
	case <-time.After(45 * time.Second):
		t.Fatalf("armor-fleet did not shut down within 45s of SIGTERM\nstderr:\n%s", stderr.String())
	}

	for _, want := range []string{
		"Loaded 1 targets from " + targets,
		"Starting fleet server on " + addr,
		"Server stopped",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}
