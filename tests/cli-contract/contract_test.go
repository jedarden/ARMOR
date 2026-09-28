// Package clicontract contains the executable smoke matrix for the operator
// CLI contracts.  The package deliberately builds the real binaries instead
// of importing their main packages: a contract regression is useful only when
// it observes the same flag parsing, streams, and exit status an operator sees.
package clicontract

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type commandResult struct {
	stdout   string
	stderr   string
	exitCode int
}

func (r commandResult) combined() string { return r.stdout + r.stderr }

type contractBinary struct {
	name string
	path string
}

var (
	binariesOnce sync.Once
	binaries     map[string]contractBinary
	binariesErr  error
)

// repoRoot resolves the repository from this source file, so the suite also
// works from the git-archive extraction used by the completion checklist.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate the contract suite")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func builtBinaries(t *testing.T) map[string]contractBinary {
	t.Helper()
	binariesOnce.Do(func() {
		root := repoRoot(t)
		dir, err := os.MkdirTemp("", "armor-cli-contract-")
		if err != nil {
			binariesErr = fmt.Errorf("create binary directory: %w", err)
			return
		}
		binaries = make(map[string]contractBinary, 3)
		for _, spec := range []struct {
			name string
			dir  string
		}{
			{name: "armor", dir: "cmd/armor"},
			{name: "restore-verifier", dir: "cmd/restore-verifier"},
			{name: "armor-fleet", dir: "cmd/armor-fleet"},
		} {
			path := filepath.Join(dir, spec.name)
			cmd := exec.Command("go", "build", "-buildvcs=false", "-o", path, ".")
			cmd.Dir = filepath.Join(root, spec.dir)
			if output, buildErr := cmd.CombinedOutput(); buildErr != nil {
				binariesErr = fmt.Errorf("build %s: %w\n%s", spec.name, buildErr, output)
				return
			}
			binaries[spec.name] = contractBinary{name: spec.name, path: path}
		}
	})
	if binariesErr != nil {
		t.Fatal(binariesErr)
	}
	return binaries
}

// cleanEnv removes configuration inherited from the developer or CI shell.
// Contract cases then list every input they intentionally provide, which is
// especially important for tests of missing credentials and key fallbacks.
func cleanEnv(overrides ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, value := range os.Environ() {
		if strings.HasPrefix(value, "ARMOR_") ||
			strings.HasPrefix(value, "VERIFIER_") ||
			strings.HasPrefix(value, "SEAM_TOKEN=") {
			continue
		}
		env = append(env, value)
	}
	return append(env, overrides...)
}

func runBinary(t *testing.T, binary contractBinary, env []string, args ...string) commandResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary.path, args...)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("%s %s timed out: %v\nstderr: %s", binary.name, strings.Join(args, " "), ctx.Err(), stderr.String())
	}
	result := commandResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run %s %s: %v\nstderr: %s", binary.name, strings.Join(args, " "), err, stderr.String())
	}
	result.exitCode = exitErr.ExitCode()
	return result
}

var (
	armorSectionHeading     = regexp.MustCompile(`(?m)^## \x60armor ([a-z-]+)\x60\s*$`)
	companionSectionHeading = regexp.MustCompile(`(?m)^## \x60(restore-verifier|armor-fleet)\x60\s*$`)
	codeSpan                = regexp.MustCompile("`([^`\\n]*)`")
)

func readContract(t *testing.T, path ...string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, path...)...))
	if err != nil {
		t.Fatalf("read contract %s: %v", filepath.Join(path...), err)
	}
	return string(raw)
}

func sectionsByHeading(t *testing.T, doc string, heading *regexp.Regexp) map[string]string {
	t.Helper()
	matches := heading.FindAllStringSubmatchIndex(doc, -1)
	sections := make(map[string]string, len(matches))
	for i, match := range matches {
		end := len(doc)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		sections[doc[match[2]:match[3]]] = doc[match[1]:end]
	}
	return sections
}

func documentedFlags(section string, ignored ...string) []string {
	ignore := make(map[string]bool, len(ignored))
	for _, name := range ignored {
		ignore[name] = true
	}
	seen := make(map[string]bool)
	for _, match := range codeSpan.FindAllStringSubmatch(section, -1) {
		token := strings.TrimSpace(match[1])
		if !strings.HasPrefix(token, "-") {
			continue
		}
		name := strings.TrimLeft(token, "-")
		if name == "" || strings.ContainsAny(name, " \t/") || ignore[name] {
			continue
		}
		seen[name] = true
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

type helpCase struct {
	name       string
	binaryName string
	args       []string
	section    string
	ignored    []string
	usage      string
}

// TestDocumentedHelpContract is intentionally table-driven.  Adding a new
// command or companion binary requires a row in the documented contract and
// makes the real executable's help output prove every documented flag exists.
func TestDocumentedHelpContract(t *testing.T) {
	bins := builtBinaries(t)
	armorDoc := readContract(t, "docs", "cli-reference.md")
	companionDoc := readContract(t, "docs", "companion-cli-reference.md")
	armorSections := sectionsByHeading(t, armorDoc, armorSectionHeading)
	companionSections := sectionsByHeading(t, companionDoc, companionSectionHeading)

	wantArmor := []string{"serve", "demo", "check", "decrypt", "verify", "migrate", "client-config", "version", "help"}
	wantCompanions := []string{"restore-verifier", "armor-fleet"}
	assertNames := func(label string, got map[string]string, want []string) {
		t.Helper()
		actual := make([]string, 0, len(got))
		for name := range got {
			actual = append(actual, name)
		}
		sort.Strings(actual)
		sort.Strings(want)
		if strings.Join(actual, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("%s contract sections = %q, want %q", label, actual, want)
		}
	}
	assertNames("armor", armorSections, wantArmor)
	assertNames("companion", companionSections, wantCompanions)

	var cases []helpCase
	for _, name := range wantArmor {
		cases = append(cases, helpCase{
			name: name, binaryName: "armor", args: []string{name, "--help"},
			section: armorSections[name], ignored: []string{"h", "help"},
			usage: "Usage: armor " + name,
		})
	}
	for _, name := range wantCompanions {
		cases = append(cases, helpCase{
			name: name, binaryName: name, args: []string{"--help"},
			section: companionSections[name], ignored: []string{"h", "help", "v", "version"},
			usage: map[string]string{
				"restore-verifier": "Usage:",
				"armor-fleet":      "Usage of",
			}[name],
		})
	}

	for _, tc := range cases {
		t.Run(tc.binaryName+"/"+tc.name, func(t *testing.T) {
			res := runBinary(t, bins[tc.binaryName], cleanEnv(), tc.args...)
			if res.exitCode != 0 {
				t.Fatalf("%s %s: exit = %d, want 0\n%s", tc.binaryName, strings.Join(tc.args, " "), res.exitCode, res.combined())
			}
			if !strings.Contains(res.combined(), tc.usage) {
				t.Errorf("help output missing %q:\n%s", tc.usage, res.combined())
			}
			for _, flagName := range documentedFlags(tc.section, tc.ignored...) {
				if !strings.Contains(res.combined(), "-"+flagName) {
					t.Errorf("help output missing documented flag -%s:\n%s", flagName, res.combined())
				}
			}
		})
	}
}

type exitCase struct {
	name       string
	binaryName string
	args       []string
	env        []string
	wantExit   int
	wantText   string
}

// TestDocumentedExitContract checks the shared usage convention for every
// armor subcommand and the documented startup failures for the companions.
// The companion processes intentionally ignore positional arguments; that
// exception is asserted separately below rather than silently generalized.
func TestDocumentedExitContract(t *testing.T) {
	bins := builtBinaries(t)
	var cases []exitCase
	for _, name := range []string{"serve", "demo", "check", "decrypt", "verify", "migrate", "client-config", "version"} {
		cases = append(cases, exitCase{
			name: name + " rejects a positional argument", binaryName: "armor",
			args: []string{name, "cli-contract-positional"}, wantExit: 2,
			wantText: "unexpected arguments",
		})
		cases = append(cases, exitCase{
			name: name + " rejects an undefined flag", binaryName: "armor",
			args: []string{name, "--cli-contract-undefined"}, wantExit: 2,
			wantText: "flag provided but not defined",
		})
	}
	cases = append(cases,
		exitCase{name: "help ignores extra arguments", binaryName: "armor", args: []string{"help", "extra", "arguments"}, wantExit: 0, wantText: "Available subcommands:"},
		exitCase{name: "unknown armor subcommand", binaryName: "armor", args: []string{"cli-contract-no-such-command"}, wantExit: 2, wantText: "Unknown subcommand"},
		exitCase{name: "missing verify bucket", binaryName: "armor", args: []string{"verify"}, wantExit: 2, wantText: "-bucket is required"},
		exitCase{name: "missing migrate admin URL", binaryName: "armor", args: []string{"migrate"}, wantExit: 2, wantText: "-admin-url is required"},
		exitCase{name: "missing client-config tool", binaryName: "armor", args: []string{"client-config"}, wantExit: 2, wantText: "-for is required"},
		exitCase{name: "decrypt without a MEK", binaryName: "armor", args: []string{"decrypt"}, wantExit: 1, wantText: "no MEK provided"},
		exitCase{name: "check without deployment configuration", binaryName: "armor", args: []string{"check"}, wantExit: 1, wantText: "Config errors"},
		exitCase{name: "restore-verifier missing B2 configuration", binaryName: "restore-verifier", wantExit: 1, wantText: "Missing required B2 credentials"},
		exitCase{name: "armor-fleet missing targets", binaryName: "armor-fleet", wantExit: 2, wantText: "-targets flag is required"},
	)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runBinary(t, bins[tc.binaryName], cleanEnv(tc.env...), tc.args...)
			if res.exitCode != tc.wantExit {
				t.Errorf("%s %s: exit = %d, want %d\nstdout: %s\nstderr: %s", tc.binaryName, strings.Join(tc.args, " "), res.exitCode, tc.wantExit, res.stdout, res.stderr)
			}
			if tc.wantText != "" && !strings.Contains(res.combined(), tc.wantText) {
				t.Errorf("%s %s: output missing %q:\n%s", tc.binaryName, strings.Join(tc.args, " "), tc.wantText, res.combined())
			}
		})
	}
}

func TestDocumentedOutputEnvironmentAndSafetyContract(t *testing.T) {
	bins := builtBinaries(t)
	armor := bins["armor"]

	t.Run("version JSON honors documented environment input", func(t *testing.T) {
		res := runBinary(t, armor, cleanEnv("ARMOR_FORMAT_VERSION=2"), "version", "--json")
		if res.exitCode != 0 {
			t.Fatalf("version --json: exit = %d\n%s", res.exitCode, res.combined())
		}
		var value struct {
			App                string `json:"app"`
			FormatWriteVersion int    `json:"format_write_version"`
		}
		if err := json.Unmarshal([]byte(res.stdout), &value); err != nil {
			t.Fatalf("version --json is not JSON: %v\n%s", err, res.stdout)
		}
		if value.App != "armor" || value.FormatWriteVersion != 2 {
			t.Errorf("version JSON = %+v, want app armor and format_write_version 2", value)
		}
		if strings.TrimSpace(res.stderr) != "" {
			t.Errorf("version --json wrote diagnostics to stderr: %s", res.stderr)
		}
	})

	t.Run("invalid documented environment value is a usage error", func(t *testing.T) {
		res := runBinary(t, armor, cleanEnv("ARMOR_FORMAT_VERSION=4"), "version", "--json")
		if res.exitCode != 2 || !strings.Contains(res.stderr, "ARMOR_FORMAT_VERSION") {
			t.Errorf("invalid ARMOR_FORMAT_VERSION: exit = %d, stderr = %q; want exit 2 naming the variable", res.exitCode, res.stderr)
		}
	})

	for _, tool := range []string{"aws-cli", "rclone", "boto3", "duckdb", "litestream", "barman"} {
		tool := tool
		t.Run("client-config/"+tool, func(t *testing.T) {
			res := runBinary(t, armor, cleanEnv(), "client-config", "-for", tool, "-endpoint", "http://127.0.0.1:19000")
			if res.exitCode != 0 {
				t.Fatalf("client-config -for %s: exit = %d\n%s", tool, res.exitCode, res.combined())
			}
			if !strings.Contains(res.stdout, "http://127.0.0.1:19000") || !strings.Contains(res.stdout, "Multipart") {
				t.Errorf("client-config -for %s omitted endpoint or multipart contract:\n%s", tool, res.stdout)
			}
			if strings.Contains(res.stdout, "smoke-secret") || strings.Contains(res.stderr, "smoke-secret") {
				t.Errorf("client-config -for %s echoed a planted credential marker", tool)
			}
		})
	}

	t.Run("migrate JSON keeps stdout machine-readable and authenticates", func(t *testing.T) {
		const token = "cli-contract-token-marker"
		var authorization string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authorization = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintln(w, `{"status":"completed","total_objects":1,"processed_objects":1,"failed_objects":0}`)
		}))
		defer server.Close()

		res := runBinary(t, armor, cleanEnv("ARMOR_ADMIN_TOKEN="+token), "migrate", "-admin-url", server.URL, "-json")
		if res.exitCode != 0 {
			t.Fatalf("migrate -json: exit = %d\n%s", res.exitCode, res.combined())
		}
		if authorization != "Bearer "+token {
			t.Errorf("migrate Authorization = %q, want bearer token", authorization)
		}
		if !json.Valid([]byte(strings.TrimSpace(res.stdout))) {
			t.Errorf("migrate -json stdout is not pure JSON: %q", res.stdout)
		}
		if strings.Contains(res.stdout, "Starting migration") || strings.Contains(res.stdout, token) || strings.Contains(res.stderr, token) {
			t.Errorf("migrate exposed progress or token in output:\nstdout: %s\nstderr: %s", res.stdout, res.stderr)
		}
	})

	for _, tc := range []struct {
		name   string
		args   []string
		env    []string
		secret string
	}{
		{name: "mek flag", args: []string{"decrypt", "-mek", strings.Repeat("9e", 32)}, secret: strings.Repeat("9e", 32)},
		{name: "mek environment", args: []string{"decrypt"}, env: []string{"ARMOR_MEK=" + strings.Repeat("8d", 32)}, secret: strings.Repeat("8d", 32)},
	} {
		tc := tc
		t.Run("decrypt safety/"+tc.name, func(t *testing.T) {
			res := runBinary(t, armor, cleanEnv(tc.env...), tc.args...)
			if res.exitCode != 1 {
				t.Errorf("decrypt failure path: exit = %d, want 1\n%s", res.exitCode, res.combined())
			}
			if strings.Contains(res.combined(), tc.secret) {
				t.Errorf("decrypt echoed key material into its output:\n%s", res.combined())
			}
		})
	}
}
