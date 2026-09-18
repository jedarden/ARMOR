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
	Name        string                 // subcommand name, as typed on the command line
	Description string                 // one-line summary, shown in the top-level help table
	Flags       *flag.FlagSet          // the subcommand's own flag set; nil means "takes no flags"
	Func        func(fs *flag.FlagSet) // executed after Flags has parsed successfully
}

// commands registry - populated by init() functions in cmd_*.go files
var commands = make(map[string]Command)

// registerCommand adds a command to the registry and arms its per-subcommand
// help. Each command owns its own flag.FlagSet (created here when the command
// takes no flags), so `armor <cmd> --help` prints only that subcommand's
// summary and flags: under ExitOnError, flag's built-in -h/--help handling
// invokes the Usage function below and exits 0 without running the command.
// Called by init() functions.
func registerCommand(cmd Command) {
	if cmd.Flags == nil {
		cmd.Flags = flag.NewFlagSet(cmd.Name, flag.ExitOnError)
	}
	flags := cmd.Flags
	flags.Usage = func() {
		out := flags.Output()
		fmt.Fprintf(out, "%s\n\n", cmd.Description)
		fmt.Fprintf(out, "Usage: armor %s", cmd.Name)
		if hasFlags(flags) {
			fmt.Fprint(out, " [flags]")
		}
		fmt.Fprint(out, "\n\n")
		flags.PrintDefaults()
	}
	commands[cmd.Name] = cmd
}

// hasFlags reports whether the flag set defines at least one flag, so the
// usage line only offers "[flags]" where flags exist.
func hasFlags(fs *flag.FlagSet) bool {
	seen := false
	fs.VisitAll(func(*flag.Flag) { seen = true })
	return seen
}

func main() {
	args := os.Args[1:]

	// Check for --version flag before parsing other flags
	if len(args) > 0 && (args[0] == "--version" || args[0] == "-v") {
		version.Print("armor")
		os.Exit(0)
	}

	// Top-level help. `armor help` is a registered subcommand (cmd_help.go);
	// --help/-h are not subcommands, so they are intercepted here.
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h" || args[0] == "-help") {
		printTopLevelHelp(os.Stdout)
		os.Exit(0)
	}

	// Dispatch on os.Args[1] only; the subcommand's own FlagSet parses the
	// rest. Default to "serve" if no subcommand provided: the container
	// ENTRYPOINT is bare /armor and every deployment relies on this default.
	subcommand := "serve"
	var rest []string
	if len(args) > 0 {
		subcommand = args[0]
		rest = args[1:]
	}

	// Look up the command
	cmd, exists := commands[subcommand]
	if !exists {
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		fmt.Fprintf(os.Stderr, "\nAvailable subcommands:\n")
		listCommands(os.Stderr)
		os.Exit(2)
	}

	// Parse the subcommand's flags. ExitOnError owns the failure paths: an
	// undefined or malformed flag prints the subcommand usage and exits 2,
	// and -h/--help prints it and exits 0 — neither reaches Func.
	cmd.Flags.Parse(rest)

	// Execute the command
	cmd.Func(cmd.Flags)
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
		fmt.Fprintf(w, "  %-14s %s\n", name, cmd.Description)
	}
}

// versionJSONFlag arms `armor version --json`.
var versionJSONFlag bool

func init() {
	// version's own flag set, like every other subcommand's
	versionFlags := flag.NewFlagSet("version", flag.ExitOnError)
	versionFlags.BoolVar(&versionJSONFlag, "json", false, "Print version information as a single-line JSON object")

	// Register version command
	registerCommand(Command{
		Name:        "version",
		Description: "Print version information (--json for machine-readable output)",
		Flags:       versionFlags,
		Func:        runVersion,
	})
}

// runVersion implements the version subcommand: the one-line text form by
// default, or a JSON object with --json. `armor --version` and `-v` are
// handled in main and always print the text form.
func runVersion(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", fs.Args())
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
