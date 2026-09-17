// Package main is the entry point for the ARMOR server.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

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

func init() {
	// Register version command
	registerCommand(Command{
		Name:        "version",
		Description: "Print version information",
		Func: func() {
			version.Print("armor")
		},
	})
}
