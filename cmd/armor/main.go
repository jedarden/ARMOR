// Package main is the entry point for the ARMOR server.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/jedarden/armor/internal/version"
)

// Command represents a subcommand that can be registered and executed.
type Command struct {
	Name        string
	Description string
	Func        func() // The function to execute for this command
}

// commands registry - populated by init() functions in cmd_*.go files
var commands = make(map[string]Command)

// registerCommand adds a command to the registry. Called by init() functions.
func registerCommand(cmd Command) {
	commands[cmd.Name] = cmd
}

func main() {
	// Check for --version flag before parsing other flags
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		version.Print("armor")
		os.Exit(0)
	}

	// Parse flags - we only care about subcommand name
	flag.Parse()

	args := flag.Args()

	// Default to "serve" if no subcommand provided
	subcommand := "serve"
	if len(args) > 0 {
		subcommand = args[0]
	}

	// Look up the command
	cmd, exists := commands[subcommand]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		fmt.Fprintf(os.Stderr, "\nAvailable subcommands:\n")
		listCommands(os.Stderr)
		os.Exit(2)
	}

	// Consume the subcommand name before dispatch. Subcommands that take
	// flags (demo, client-config, migrate, verify) re-parse the shared
	// flag.CommandLine themselves, and flag parsing stops at the first
	// non-flag argument — leaving the subcommand name in place made every
	// one of them see itself as an unexpected positional and reject its own
	// flags (e.g. "armor demo --listen ..." failed with "unexpected
	// arguments"). Re-slicing os.Args hands each subcommand only its own
	// flags and positionals.
	if len(args) > 0 {
		os.Args = append([]string{os.Args[0]}, args[1:]...)
	}

	// Execute the command
	cmd.Func()
}

// listCommands prints all registered commands to the given writer.
func listCommands(w io.Writer) {
	// Sort commands by name for consistent output
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cmd := commands[name]
		fmt.Fprintf(w, "  %-12s %s\n", name, cmd.Description)
	}
}

// versionJSONFlag arms `armor version --json`.
var versionJSONFlag bool

func init() {
	// version specific flag, on the shared flag.CommandLine like every other
	// subcommand's flags
	flag.BoolVar(&versionJSONFlag, "json", false, "Print version information as a single-line JSON object")

	// Register version command
	registerCommand(Command{
		Name:        "version",
		Description: "Print version information (--json for machine-readable output)",
		Func:        runVersion,
	})
}

// runVersion implements the version subcommand: the one-line text form by
// default, or a JSON object with --json. `armor --version` and `-v` are
// handled in main and always print the text form.
func runVersion() {
	// Parse flags
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor version [--json]\n")
		exit(2)
	}

	if !versionJSONFlag {
		version.Print("armor")
		return
	}

	out, err := version.JSON("armor", formatWriteVersion())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: rendering version JSON: %v\n", err)
		exit(1)
	}
	fmt.Println(out)
}

// formatWriteVersion resolves the envelope format this build writes, from the
// same ARMOR_FORMAT_VERSION variable internal/config reads (default 3), so
// `armor version --json` reports the same value the server's /version
// endpoint would. config.Load cannot be used here: it requires server
// credentials, so the default-and-validate logic is mirrored instead.
func formatWriteVersion() int {
	v := version.DefaultFormatWriteVersion
	if s := os.Getenv("ARMOR_FORMAT_VERSION"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || (n != 2 && n != 3) {
			fmt.Fprintf(os.Stderr, "Error: ARMOR_FORMAT_VERSION must be an integer (2 or 3), got %q\n", s)
			exit(2)
		}
		v = n
	}
	return v
}
