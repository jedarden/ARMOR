package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

// mockCommandFunc is a test helper that captures execution
var mockExecuted string

func mockCmd() func(*flag.FlagSet) {
	return func(_ *flag.FlagSet) {
		mockExecuted = "mock"
	}
}

// runCommand parses args into the named subcommand's flag set (the way main
// dispatches) and runs it. Parsing errors are impossible for defined flags;
// test callers pass only parseable arguments and assert the Func's own
// validation paths.
func runCommand(t *testing.T, name string, args ...string) {
	t.Helper()
	cmd, exists := commands[name]
	if !exists {
		t.Fatalf("command %q is not registered", name)
	}
	if err := cmd.Flags.Parse(args); err != nil {
		t.Fatalf("parse %q flags %v: %v", name, args, err)
	}
	cmd.Func(cmd.Flags)
}

// TestRegisterCommand verifies that init() registration populates the commands map
func TestRegisterCommand(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Clear and register test command
	commands = make(map[string]Command)
	testCmd := Command{
		Name:        "test",
		Description: "Test command",
		Func:        mockCmd(),
	}
	registerCommand(testCmd)

	// Verify registration
	if _, exists := commands["test"]; !exists {
		t.Errorf("registerCommand failed to register 'test' command")
	}

	// registerCommand arms every command with its own flag set named after
	// the subcommand, even when the command declares none.
	if commands["test"].Flags == nil {
		t.Errorf("registerCommand did not arm 'test' with a flag set")
	} else if commands["test"].Flags.Name() != "test" {
		t.Errorf("flag set name = %q, want %q", commands["test"].Flags.Name(), "test")
	}

	// Restore original commands
	commands = original
}

// TestCommandDispatch verifies that commands execute correctly
func TestCommandDispatch(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Set up test commands
	mockExecuted = ""
	commands = make(map[string]Command)
	commands["test"] = Command{
		Name:        "test",
		Description: "Test command",
		Func: func(_ *flag.FlagSet) {
			mockExecuted = "test"
		},
	}
	commands["help"] = Command{
		Name:        "help",
		Description: "Show help",
		Func: func(_ *flag.FlagSet) {
			mockExecuted = "help"
		},
	}

	// Test 1: Valid command executes
	// We can't test main() directly due to os.Exit, but we can verify registration
	t.Run("Registration", func(t *testing.T) {
		if _, exists := commands["test"]; !exists {
			t.Error("test command not registered")
		}
		if _, exists := commands["help"]; !exists {
			t.Error("help command not registered")
		}
	})

	// Test 2: Non-existent command handling
	t.Run("UnknownCommand", func(t *testing.T) {
		_, exists := commands["nonexistent"]
		if exists {
			t.Error("nonexistent command should not exist")
		}
	})

	// Restore original commands
	commands = original
}

// TestListCommands verifies that listCommands outputs sorted commands
func TestListCommands(t *testing.T) {
	// Save original commands
	original := make(map[string]Command)
	for k, v := range commands {
		original[k] = v
	}

	// Set up test commands with unsorted names
	commands = make(map[string]Command)
	commands["zebra"] = Command{Name: "zebra", Description: "Last"}
	commands["apple"] = Command{Name: "apple", Description: "First"}
	commands["middle"] = Command{Name: "middle", Description: "Mid"}

	var buf bytes.Buffer
	listCommands(&buf)
	output := buf.String()

	// Verify sorted output (alphabetical)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(lines))
	}

	// Check alphabetical order
	if !strings.Contains(lines[0], "apple") {
		t.Errorf("first line should contain 'apple', got: %s", lines[0])
	}
	if !strings.Contains(lines[1], "middle") {
		t.Errorf("second line should contain 'middle', got: %s", lines[1])
	}
	if !strings.Contains(lines[2], "zebra") {
		t.Errorf("third line should contain 'zebra', got: %s", lines[2])
	}

	// Restore original commands
	commands = original
}

// TestTopLevelHelpListsSubcommands verifies the `armor help` / `armor --help`
// output: every registered subcommand appears with its one-line summary, the
// per-subcommand help hint is present, and — the bug this layout fixes — no
// flags leak into the top-level listing (it used to print one flat list of
// every subcommand's flags).
func TestTopLevelHelpListsSubcommands(t *testing.T) {
	var buf bytes.Buffer
	printTopLevelHelp(&buf)
	out := buf.String()

	for _, name := range []string{"serve", "demo", "check", "client-config", "decrypt", "migrate", "verify", "version", "help"} {
		if cmd, exists := commands[name]; exists {
			if !strings.Contains(out, name) {
				t.Errorf("top-level help does not list %q", name)
			}
			if !strings.Contains(out, cmd.Description) {
				t.Errorf("top-level help does not carry %q's one-line summary", name)
			}
		} else {
			t.Errorf("expected subcommand %q is not registered", name)
		}
	}

	if !strings.Contains(out, "--help for its flags") {
		t.Errorf("top-level help missing the per-subcommand help hint, got:\n%s", out)
	}
	if !strings.Contains(out, "With no subcommand, armor serves") {
		t.Errorf("top-level help missing the bare-armor-serves note, got:\n%s", out)
	}

	for _, forbidden := range []string{"-for", "-escrow", "-iv", "-keys-file", "-admin-url", "-listen"} {
		if strings.Contains(out, forbidden) {
			t.Errorf("top-level help must not list subcommand flags; found %q in:\n%s", forbidden, out)
		}
	}
}

// TestSubcommandHelpShowsOnlyOwnFlags verifies `armor <cmd> --help` content:
// the command's one-line summary followed by only that subcommand's flags.
// client-config owns -for; check owns no flags at all.
func TestSubcommandHelpShowsOnlyOwnFlags(t *testing.T) {
	t.Run("client-config", func(t *testing.T) {
		cmd, exists := commands["client-config"]
		if !exists {
			t.Fatal("client-config is not registered")
		}
		var buf bytes.Buffer
		cmd.Flags.SetOutput(&buf)
		cmd.Flags.Usage()

		out := buf.String()
		if !strings.Contains(out, cmd.Description) {
			t.Errorf("client-config help missing its one-line summary, got:\n%s", out)
		}
		for _, own := range []string{"-for", "-endpoint", "-bucket", "-credential"} {
			if !strings.Contains(out, own) {
				t.Errorf("client-config help missing its own flag %q, got:\n%s", own, out)
			}
		}
		for _, foreign := range []string{"-escrow", "-iv", "-keys-file", "-admin-url", "-json"} {
			if strings.Contains(out, foreign) {
				t.Errorf("client-config help must not show foreign flag %q, got:\n%s", foreign, out)
			}
		}
	})

	t.Run("check", func(t *testing.T) {
		cmd, exists := commands["check"]
		if !exists {
			t.Fatal("check is not registered")
		}
		var buf bytes.Buffer
		cmd.Flags.SetOutput(&buf)
		cmd.Flags.Usage()

		out := buf.String()
		if !strings.Contains(out, cmd.Description) {
			t.Errorf("check help missing its one-line summary, got:\n%s", out)
		}
		if strings.Contains(out, "  -") {
			t.Errorf("check takes no flags; its help must list none, got:\n%s", out)
		}
	})
}

// TestEveryCommandArmedForDispatch verifies the registry invariant main
// dispatches against: every command has both a flag set and a runnable Func.
func TestEveryCommandArmedForDispatch(t *testing.T) {
	if len(commands) == 0 {
		t.Fatal("no commands registered")
	}
	for name, cmd := range commands {
		if cmd.Flags == nil {
			t.Errorf("command %q has no flag set", name)
		}
		if cmd.Func == nil {
			t.Errorf("command %q has no Func", name)
		}
	}
}
