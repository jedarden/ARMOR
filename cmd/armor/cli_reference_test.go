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
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
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
