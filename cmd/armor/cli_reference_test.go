//go:build !integration
// +build !integration

// Smoke coverage for the operator-facing CLI reference
// (docs/cli-reference.md). The document is the operator contract for these
// subcommands; these tests keep it honest against the binary's own registry:
// every registered command is documented and nothing else, every flag a
// command registers appears in its documented section, every flag named in a
// section actually exists on that command, and every documented help path
// renders. Without this, a renamed flag or a new subcommand ships with a
// reference page that no longer describes the binary.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// cliReferencePath resolves docs/cli-reference.md from this file's location,
// so the test finds the document regardless of the working directory go test
// was invoked from.
func cliReferencePath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	// thisFile is cmd/armor/cli_reference_test.go; the repo root is two up.
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))),
		"docs", "cli-reference.md")
}

// sectionHeading matches the reference's per-command headings: ## `armor <name>`
var sectionHeading = regexp.MustCompile("(?m)^## `armor ([a-z-]+)`\\s*$")

// backtickSpan matches a `...` code span anywhere in a section.
var backtickSpan = regexp.MustCompile("`([^`\n]*)`")

// loadCLIReference reads the reference document, skipping the test when the
// file has not been checked out (it lives outside this package's tree).
func loadCLIReference(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(cliReferencePath(t))
	if err != nil {
		t.Fatalf("read CLI reference: %v", err)
	}
	return string(raw)
}

// commandSections splits the document into per-command sections keyed by
// subcommand name. Only `## `armor <name>`` headings count; the general
// sections (global behavior, exit-code conventions) are not bound to any one
// command's registry and are skipped.
func commandSections(doc string) map[string]string {
	sections := make(map[string]string)
	matches := sectionHeading.FindAllStringSubmatchIndex(doc, -1)
	for i, m := range matches {
		name := doc[m[2]:m[3]]
		end := len(doc)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections[name] = doc[m[1]:end]
	}
	return sections
}

// documentedFlagNames extracts the flag names a section documents: every
// backticked token that starts with a dash, dashes stripped. `--help`/`-h`
// are the universal per-command help path, not registered flags, and tokens
// with interior spaces are prose, not flags.
func documentedFlagNames(section string) map[string]bool {
	names := make(map[string]bool)
	for _, m := range backtickSpan.FindAllStringSubmatch(section, -1) {
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

// registeredFlagNames collects the flag names a command's own FlagSet
// defines — the authoritative set `armor <cmd> --help` prints.
func registeredFlagNames(cmd Command) map[string]bool {
	names := make(map[string]bool)
	cmd.Flags.VisitAll(func(f *flag.Flag) { names[f.Name] = true })
	return names
}

// TestCLIReferenceCoversExactlyTheRegisteredCommands: the reference's command
// sections and the binary's registry must be the same set — a new subcommand
// without a section and a section for a removed subcommand are both drift.
func TestCLIReferenceCoversExactlyTheRegisteredCommands(t *testing.T) {
	doc := loadCLIReference(t)
	documented := commandSections(doc)
	if len(documented) == 0 {
		t.Fatal("docs/cli-reference.md documents no `armor <cmd>` sections; the heading format the smoke tests match is `## `armor <name>``")
	}

	var want, got []string
	for name := range commands {
		want = append(want, name)
	}
	for name := range documented {
		got = append(got, name)
	}
	sort.Strings(want)
	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("docs/cli-reference.md command sections %q do not match the registered subcommands %q", got, want)
	}
}

// TestCLIReferenceFlagsMatchRegistry pins each command's documented flag list
// to its FlagSet, in both directions: a flag the binary registers must be
// documented, and a flag the document names must exist on that command
// (renames and typo'd flags fail here, not in an operator's terminal).
func TestCLIReferenceFlagsMatchRegistry(t *testing.T) {
	doc := loadCLIReference(t)
	for name, section := range commandSections(doc) {
		cmd, exists := commands[name]
		if !exists {
			t.Errorf("section for unregistered command %q", name)
			continue
		}
		documented := documentedFlagNames(section)
		registered := registeredFlagNames(cmd)

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
			t.Errorf("%s: registered flags missing from docs/cli-reference.md: %q", name, missing)
		}
		if len(phantom) > 0 {
			t.Errorf("%s: flags documented in docs/cli-reference.md that the command does not register: %q", name, phantom)
		}
	}
}

// TestDocumentedCommandsRenderHelp exercises the output `armor <cmd> --help`
// prints (flag.ExitOnError renders this Usage and exits 0 without running the
// command, so rendering it directly is the in-process smoke path): every
// command must produce its usage line and summary, and no command's help may
// be empty.
func TestDocumentedCommandsRenderHelp(t *testing.T) {
	for _, name := range sortedCommandNames() {
		cmd := commands[name]
		var buf bytes.Buffer
		cmd.Flags.SetOutput(&buf)
		cmd.Flags.Usage()

		out := buf.String()
		if !strings.Contains(out, "Usage: armor "+name) {
			t.Errorf("%s: help output missing \"Usage: armor %s\", got:\n%s", name, name, out)
		}
		if cmd.Description != "" && !strings.Contains(out, cmd.Description) {
			t.Errorf("%s: help output missing its one-line summary %q, got:\n%s", name, cmd.Description, out)
		}
	}
}

// sortedCommandNames returns the registry's names in stable order.
func sortedCommandNames() []string {
	var names []string
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestVersionCommandRejectsUnexpectedArguments covers a documented validation
// path no other test in the package reaches: `armor version <anything>` is a
// usage error and must exit 2, not print a version line.
func TestVersionCommandRejectsUnexpectedArguments(t *testing.T) {
	oldJSON := versionJSONFlag
	oldExit := exit
	oldStderr := os.Stderr
	defer func() {
		versionJSONFlag = oldJSON
		exit = oldExit
		os.Stderr = oldStderr
	}()
	versionJSONFlag = false

	var exitCode int
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}
	stderrR, stderrW, _ := os.Pipe()
	os.Stderr = stderrW

	cmd := commands["version"]
	_ = cmd.Flags.Parse([]string{"stray-argument"})
	func() {
		defer func() { _ = recover() }() // the expected exit panic
		cmd.Func(cmd.Flags)
	}()

	stderrW.Close()
	os.Stderr = oldStderr
	var stderrBuf bytes.Buffer
	stderrBuf.ReadFrom(stderrR)

	if exitCode != 2 {
		t.Errorf("version with a positional argument: exit code = %d, want 2 (stderr: %q)", exitCode, stderrBuf.String())
	}
}

// TestVersionCommandJSONSmoke runs the documented `armor version --json`
// happy path: valid single-line JSON on stdout carrying the app name and the
// format write version the README documents.
func TestVersionCommandJSONSmoke(t *testing.T) {
	oldJSON := versionJSONFlag
	oldFormatEnv, hadFormatEnv := os.LookupEnv("ARMOR_FORMAT_VERSION")
	oldStdout := os.Stdout
	defer func() {
		versionJSONFlag = oldJSON
		if hadFormatEnv {
			os.Setenv("ARMOR_FORMAT_VERSION", oldFormatEnv)
		} else {
			os.Unsetenv("ARMOR_FORMAT_VERSION")
		}
		os.Stdout = oldStdout
	}()
	versionJSONFlag = true
	os.Unsetenv("ARMOR_FORMAT_VERSION")

	stdoutR, stdoutW, _ := os.Pipe()
	os.Stdout = stdoutW

	cmd := commands["version"]
	_ = cmd.Flags.Parse(nil)
	cmd.Func(cmd.Flags) // the happy path never exits

	stdoutW.Close()
	os.Stdout = oldStdout
	var stdoutBuf bytes.Buffer
	stdoutBuf.ReadFrom(stdoutR)

	var info struct {
		App                string `json:"app"`
		FormatWriteVersion int    `json:"format_write_version"`
	}
	if err := json.Unmarshal(stdoutBuf.Bytes(), &info); err != nil {
		t.Fatalf("version --json did not emit a JSON object (got %q): %v", stdoutBuf.String(), err)
	}
	if info.App != "armor" {
		t.Errorf("version --json app = %q, want %q", info.App, "armor")
	}
	if info.FormatWriteVersion == 0 {
		t.Errorf("version --json format_write_version = 0; the README documents this field as the write format `armor version` reports")
	}
}

// ---------------------------------------------------------------------------
// Executable parity: the reference versus the real binary.
//
// The registry-level tests above catch doc/registry drift statically. The
// tests below build the actual armor binary and exercise what the reference
// documents about running it: the help/version paths exit 0 with the
// documented output, unknown subcommands, undefined flags, missing required
// flags, and stray positional arguments exit 2, the commands that need
// credentials fail with exit 1 without them, client-config accepts exactly
// the documented tools and prints their configuration to stdout, migrate
// -json keeps stdout to JSON only, and every ARMOR_* variable named in the
// reference is read by the implementation. A documented behavior that drifts
// from the binary fails here, not in an operator's terminal.
// ---------------------------------------------------------------------------

// versionLineRe matches the version line the reference documents:
// `armor <version> (go<goversion>, <os>/<arch>)`.
var versionLineRe = regexp.MustCompile(`^armor \S+ \(go[^,]+, [^/)]+/[^)]+\)$`)

// armorEnvName matches an environment variable literal in Go source.
var armorEnvName = regexp.MustCompile(`(ARMOR_[A-Z0-9_]+)`)

var (
	referenceBinOnce sync.Once
	referenceBinPath string
	referenceBinErr  error
)

// cliReferenceBinDir is the temp directory the reference binary is built
// into; TestMain removes it after the run.
var cliReferenceBinDir string

// cliReferenceFileLoc returns this test file's absolute path and the repo
// root it sits under (cmd/armor/cli_reference_test.go → two directories up).
func cliReferenceFileLoc(t *testing.T) (thisFile, repoRoot string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	return file, filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// referenceBinary builds the armor binary once per test run and returns its
// path. The toolchain is the one running these tests: `go` from PATH, or
// GOROOT/bin/go — the running toolchain itself — when `go` is not on PATH.
func referenceBinary(t *testing.T) string {
	t.Helper()
	referenceBinOnce.Do(func() {
		_, repoRoot := cliReferenceFileLoc(t)
		dir, err := os.MkdirTemp("", "armor-cli-reference-bin-")
		if err != nil {
			referenceBinErr = fmt.Errorf("temp dir for reference binary: %w", err)
			return
		}
		cliReferenceBinDir = dir
		goBin, err := exec.LookPath("go")
		if err != nil {
			goBin = filepath.Join(runtime.GOROOT(), "bin", "go")
		}
		bin := filepath.Join(dir, "armor")
		build := exec.Command(goBin, "build", "-o", bin, ".")
		build.Dir = filepath.Join(repoRoot, "cmd", "armor")
		if out, buildErr := build.CombinedOutput(); buildErr != nil {
			referenceBinErr = fmt.Errorf("build reference binary: %w\n%s", buildErr, out)
			return
		}
		referenceBinPath = bin
	})
	if referenceBinErr != nil {
		t.Fatalf("%v", referenceBinErr)
	}
	return referenceBinPath
}

// armorReferenceEnv returns the subprocess environment with every ARMOR_*
// variable stripped, so the documented credential exit paths are exercised
// against a clean slate regardless of what the surrounding environment sets;
// overrides are appended last.
func armorReferenceEnv(t *testing.T, overrides ...string) []string {
	t.Helper()
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "ARMOR_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, overrides...)
}

// referenceRun is one executed invocation of the reference binary.
type referenceRun struct {
	stdout, stderr string
	exitCode       int
}

// runReference executes the built binary with a 60-second ceiling, so a
// regression that turns a documented usage error into a long-running command
// (e.g. serve binding a listener) fails loudly instead of hanging the suite.
func runReference(t *testing.T, env []string, args ...string) referenceRun {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, referenceBinary(t), args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := referenceRun{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("armor %s: %v\nstderr: %s", strings.Join(args, " "), err, stderr.String())
		}
		res.exitCode = exitErr.ExitCode()
	}
	return res
}

// TestCLIReferenceGlobalBehaviorMatchesBinary pins the "Global behavior"
// section of the reference to the binary: the help paths and --version exit 0
// and print the command table / version line, an unknown subcommand exits 2
// with the table on stderr, an undefined flag exits 2 with the subcommand's
// usage, and every command that takes arguments rejects positional ones with
// exit 2 while help ignores them.
func TestCLIReferenceGlobalBehaviorMatchesBinary(t *testing.T) {
	env := armorReferenceEnv(t)
	commandNames := sortedCommandNames()

	t.Run("help paths exit 0 with the full command table", func(t *testing.T) {
		invocations := [][]string{
			{"--help"}, {"-h"}, {"-help"}, {"help"},
			{"help", "these", "arguments", "are", "ignored"},
		}
		for _, args := range invocations {
			res := runReference(t, env, args...)
			if res.exitCode != 0 {
				t.Errorf("armor %s: exit = %d, want 0\nstderr: %s", strings.Join(args, " "), res.exitCode, res.stderr)
				continue
			}
			if !strings.Contains(res.stdout, "Available subcommands:") {
				t.Errorf("armor %s: stdout missing the command table:\n%s", strings.Join(args, " "), res.stdout)
			}
			for _, name := range commandNames {
				if !strings.Contains(res.stdout, name) {
					t.Errorf("armor %s: help table missing registered command %q", strings.Join(args, " "), name)
				}
			}
		}
	})

	t.Run("version paths exit 0 with the documented line", func(t *testing.T) {
		for _, args := range [][]string{{"--version"}, {"-v"}} {
			res := runReference(t, env, args...)
			if res.exitCode != 0 {
				t.Errorf("armor %s: exit = %d, want 0\nstderr: %s", strings.Join(args, " "), res.exitCode, res.stderr)
				continue
			}
			if line := strings.TrimSpace(res.stdout); !versionLineRe.MatchString(line) {
				t.Errorf("armor %s: stdout %q does not match the documented `armor <version> (go<goversion>, <os>/<arch>)` shape", strings.Join(args, " "), line)
			}
		}
	})

	t.Run("unknown subcommand exits 2 with the command table on stderr", func(t *testing.T) {
		res := runReference(t, env, "no-such-subcommand")
		if res.exitCode != 2 {
			t.Errorf("unknown subcommand: exit = %d, want 2", res.exitCode)
		}
		if !strings.Contains(res.stderr, "Unknown subcommand") || !strings.Contains(res.stderr, "Available subcommands:") {
			t.Errorf("unknown subcommand: stderr missing the diagnostic and command table:\n%s", res.stderr)
		}
		for _, name := range commandNames {
			if !strings.Contains(res.stderr, name) {
				t.Errorf("unknown subcommand: stderr table missing registered command %q", name)
			}
		}
	})

	t.Run("undefined flag exits 2 with the subcommand usage", func(t *testing.T) {
		for _, args := range [][]string{
			{"version", "--definitely-not-a-flag"},
			{"decrypt", "--definitely-not-a-flag"},
		} {
			res := runReference(t, env, args...)
			if res.exitCode != 2 {
				t.Errorf("armor %s: exit = %d, want 2", strings.Join(args, " "), res.exitCode)
			}
			if !strings.Contains(res.stderr, "flag provided but not defined") {
				t.Errorf("armor %s: stderr missing the undefined-flag diagnostic:\n%s", strings.Join(args, " "), res.stderr)
			}
			if !strings.Contains(res.stderr, "Usage: armor "+args[0]) {
				t.Errorf("armor %s: stderr missing the subcommand usage:\n%s", strings.Join(args, " "), res.stderr)
			}
		}
	})

	t.Run("every command but help rejects positional arguments with exit 2", func(t *testing.T) {
		for _, name := range commandNames {
			if name == "help" {
				continue // documented exception: asked-for help is never an error
			}
			res := runReference(t, env, name, "cli-reference-stray-argument")
			if res.exitCode != 2 {
				t.Errorf("armor %s with a stray positional: exit = %d, want 2\nstderr: %s", name, res.exitCode, res.stderr)
			}
			if !strings.Contains(res.stderr, "unexpected arguments") {
				t.Errorf("armor %s with a stray positional: stderr missing the diagnostic:\n%s", name, res.stderr)
			}
		}
	})
}

// TestCLIReferenceRequiredFlagAndCredentialExitsMatchBinary pins the
// per-command exit codes the reference documents for its failure paths: a
// missing required flag or stray argument is a usage error (2), and a
// command run without the credentials it documents fails with 1 before doing
// any work. The environment carries no ARMOR_* variables, so every
// credential path here is exercised from zero.
func TestCLIReferenceRequiredFlagAndCredentialExitsMatchBinary(t *testing.T) {
	env := armorReferenceEnv(t)

	cases := []struct {
		name       string
		args       []string
		wantExit   int
		wantStderr []string
	}{
		{
			name: "verify without -bucket exits 2", args: []string{"verify"}, wantExit: 2,
			wantStderr: []string{"-bucket is required"},
		},
		{
			name: "migrate without -admin-url exits 2", args: []string{"migrate"}, wantExit: 2,
			wantStderr: []string{"-admin-url is required"},
		},
		{
			name: "client-config without -for exits 2 naming the tools", args: []string{"client-config"}, wantExit: 2,
			wantStderr: []string{"-for is required", "Available tools:"},
		},
		{
			name: "client-config without -endpoint exits 2", args: []string{"client-config", "-for", "aws-cli"}, wantExit: 2,
			wantStderr: []string{"-endpoint is required"},
		},
		{
			name: "client-config with an unknown tool exits 2",
			args: []string{"client-config", "-for", "no-such-tool", "-endpoint", "http://127.0.0.1:9000"}, wantExit: 2,
			wantStderr: []string{"unknown tool"},
		},
		{
			name: "decrypt with no MEK source exits 1", args: []string{"decrypt"}, wantExit: 1,
			wantStderr: []string{"no MEK provided"},
		},
		{
			name: "decrypt with an unreadable -mek-file exits 1",
			args: []string{"decrypt", "-mek-file", filepath.Join(t.TempDir(), "absent-mek")}, wantExit: 1,
			wantStderr: []string{"read MEK file"},
		},
		{
			name: "decrypt with a malformed MEK exits 1", args: []string{"decrypt", "-mek", "not-hex"}, wantExit: 1,
			wantStderr: []string{"decode MEK hex"},
		},
		{
			name: "verify with no MEK exits 1", args: []string{"verify", "-bucket", "some-bucket"}, wantExit: 1,
			wantStderr: []string{"Error loading MEK"},
		},
		{
			name: "migrate without ARMOR_ADMIN_TOKEN exits 1",
			args: []string{"migrate", "-admin-url", "http://127.0.0.1:9"}, wantExit: 1,
			wantStderr: []string{"ARMOR_ADMIN_TOKEN"},
		},
		{
			name: "check without configuration exits 1", args: []string{"check"}, wantExit: 1,
			wantStderr: []string{"Config errors"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runReference(t, env, tc.args...)
			if res.exitCode != tc.wantExit {
				t.Errorf("armor %s: exit = %d, want %d", strings.Join(tc.args, " "), res.exitCode, tc.wantExit)
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(res.stderr, want) {
					t.Errorf("armor %s: stderr missing %q:\n%s", strings.Join(tc.args, " "), want, res.stderr)
				}
			}
		})
	}
}

// documentedClientConfigTools extracts the tool list the reference documents
// on client-config's `-for` row.
func documentedClientConfigTools(t *testing.T, doc string) []string {
	t.Helper()
	for _, line := range strings.Split(doc, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `-for` |") {
			continue
		}
		var tools []string
		for _, m := range backtickSpan.FindAllStringSubmatch(trimmed, -1) {
			tok := strings.TrimSpace(m[1])
			if tok == "" || strings.HasPrefix(tok, "-") {
				continue
			}
			tools = append(tools, tok)
		}
		if len(tools) == 0 {
			t.Fatal("docs/cli-reference.md client-config -for row no longer names any tool; update documentedClientConfigTools")
		}
		return tools
	}
	t.Fatal("docs/cli-reference.md no longer has a client-config `-for` table row; update documentedClientConfigTools")
	return nil
}

// TestCLIReferenceClientConfigToolsMatchBinary: every tool the reference
// documents generates a configuration (exit 0, text containing the endpoint
// on stdout), and the binary's own "Available tools" line names exactly the
// documented set — a tool added or removed in code without the reference is
// drift in both directions.
func TestCLIReferenceClientConfigToolsMatchBinary(t *testing.T) {
	doc := loadCLIReference(t)
	tools := documentedClientConfigTools(t, doc)
	env := armorReferenceEnv(t)

	for _, tool := range tools {
		res := runReference(t, env, "client-config", "-for", tool, "-endpoint", "http://127.0.0.1:9000")
		if res.exitCode != 0 {
			t.Errorf("client-config -for %s: exit = %d, want 0\nstderr: %s", tool, res.exitCode, res.stderr)
			continue
		}
		if strings.TrimSpace(res.stdout) == "" {
			t.Errorf("client-config -for %s: empty stdout; the reference promises the configuration text on stdout", tool)
		}
		if !strings.Contains(res.stdout, "http://127.0.0.1:9000") {
			t.Errorf("client-config -for %s: stdout does not carry the endpoint", tool)
		}
	}

	res := runReference(t, env, "client-config")
	listed := map[string]bool{}
	found := false
	for _, line := range strings.Split(res.stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Available tools:") {
			continue
		}
		found = true
		for _, tool := range strings.Split(strings.TrimPrefix(trimmed, "Available tools:"), ",") {
			if tool = strings.TrimSpace(tool); tool != "" {
				listed[tool] = true
			}
		}
	}
	if !found {
		t.Fatalf("client-config error output no longer lists its tools; stderr:\n%s", res.stderr)
	}
	documented := make(map[string]bool, len(tools))
	for _, tool := range tools {
		documented[tool] = true
	}
	var extra, missing []string
	for tool := range listed {
		if !documented[tool] {
			extra = append(extra, tool)
		}
	}
	for _, tool := range tools {
		if !listed[tool] {
			missing = append(missing, tool)
		}
	}
	sort.Strings(extra)
	sort.Strings(missing)
	if len(extra) > 0 || len(missing) > 0 {
		t.Errorf("the binary's Available tools %v and the documented tools %v diverge (undocumented: %q; undocumented-in-binary: %q)",
			listed, tools, extra, missing)
	}
}

// TestCLIReferenceMigrateJSONOutputIsPureJSON: the reference promises that
// migrate authenticates to the admin API with the ARMOR_ADMIN_TOKEN bearer
// token, that `-json` leaves stdout carrying nothing but the completion
// report, and that without `-json` the report goes to stderr. A fake admin
// API asserts the credential on the wire; the binary's streams assert the
// rest.
func TestCLIReferenceMigrateJSONOutputIsPureJSON(t *testing.T) {
	const token = "cli-reference-fake-token"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "completed",
			"total_objects":    3,
			"processed_objects": 3,
			"skipped_objects":  0,
			"failed_objects":   0,
			"dry_run":          false,
		})
	}))
	defer srv.Close()

	env := armorReferenceEnv(t, "ARMOR_ADMIN_TOKEN="+token)

	res := runReference(t, env, "migrate", "-admin-url", srv.URL, "-json")
	if res.exitCode != 0 {
		t.Fatalf("migrate -json: exit = %d, want 0\nstderr: %s", res.exitCode, res.stderr)
	}
	if gotAuth != "Bearer "+token {
		t.Errorf("migrate authenticated with %q, want the ARMOR_ADMIN_TOKEN bearer header", gotAuth)
	}
	report := strings.TrimSpace(res.stdout)
	if report == "" || !json.Valid([]byte(report)) {
		t.Fatalf("migrate -json stdout is not the promised JSON report: %q", res.stdout)
	}
	if !strings.Contains(report, `"status"`) {
		t.Errorf("migrate -json report missing the status field:\n%s", report)
	}
	if strings.Contains(report, "Starting migration") {
		t.Errorf("migrate -json leaked progress text to stdout; the reference promises stdout carries nothing but JSON:\n%s", report)
	}
	if !strings.Contains(res.stderr, "Migration started successfully.") {
		t.Errorf("migrate -json: stderr missing the human-readable summary:\n%s", res.stderr)
	}

	res = runReference(t, env, "migrate", "-admin-url", srv.URL)
	if res.exitCode != 0 {
		t.Fatalf("migrate: exit = %d, want 0\nstderr: %s", res.exitCode, res.stderr)
	}
	if strings.TrimSpace(res.stdout) != "" {
		t.Errorf("migrate without -json wrote %q to stdout; the reference promises the report goes to stderr", res.stdout)
	}
}

// TestCLIReferenceEnvVarNamesExist: every ARMOR_* variable the reference
// names must be read somewhere in the implementation (the internal config or
// the commands themselves) — a renamed or removed variable would leave the
// reference pointing operators at an env var that does nothing.
func TestCLIReferenceEnvVarNamesExist(t *testing.T) {
	doc := loadCLIReference(t)
	documented := map[string]bool{}
	for _, m := range backtickSpan.FindAllStringSubmatch(doc, -1) {
		name := strings.TrimSpace(m[1])
		if armorEnvName.MatchString(name) && name == armorEnvName.FindString(name) {
			documented[name] = true
		}
	}
	if len(documented) == 0 {
		t.Fatal("docs/cli-reference.md names no ARMOR_* variables")
	}

	_, repoRoot := cliReferenceFileLoc(t)
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
		for _, m := range armorEnvName.FindAllStringSubmatch(string(raw), -1) {
			source[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scanning sources for ARMOR_* variables: %v", err)
	}

	var missing []string
	for name := range documented {
		if !source[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("ARMOR_* variables documented in docs/cli-reference.md but read nowhere in the implementation: %q", missing)
	}
}
