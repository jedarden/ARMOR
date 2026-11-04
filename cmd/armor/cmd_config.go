// cmd_config.go implements the 'armor config' subcommand group: offline
// configuration inspection. Neither action touches the network — they load
// and render the same configuration `armor serve` would, and nothing else.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jedarden/armor/internal/config"
)

func init() {
	configFlags := flag.NewFlagSet("config", flag.ExitOnError)
	configFlags.BoolVar(&configJSONFlag, "json", false, "Print 'show' output as one JSON array of {name, value, source} rows")

	registerCommand(Command{
		Name:        "config",
		Description: "Inspect configuration offline: 'validate' reports load errors, 'show' prints every resolved setting redacted",
		Flags:       configFlags,
		Func:        runConfigCommand,
	})
}

// configJSONFlag arms `armor config show --json`.
var configJSONFlag bool

// runConfigCommand dispatches the config group's nested subcommands. The
// group takes exactly one positional argument — validate or show — so an
// unknown or missing name is the same usage error shape every other command
// reports for stray positionals.
//
// The nested subcommand is the first positional, and the group's own flags
// may appear on either side of it (`armor config --json show` and
// `armor config show --json` are equivalent): flag.Parse stops at the first
// positional, so whatever follows the subcommand is re-parsed as flags.
func runConfigCommand(fs *flag.FlagSet) {
	usage := func(args []string) {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v (expected exactly one of: validate, show)\n", args)
		fmt.Fprintf(os.Stderr, "Usage: armor config <validate|show> [--json]\n")
		exit(2)
	}

	args := fs.Args()
	if len(args) == 0 {
		usage(args)
	}
	subcommand, rest := args[0], args[1:]
	if len(rest) > 0 {
		// ExitOnError owns flag failures: an undefined or malformed flag
		// after the subcommand prints the usage and exits 2 before the
		// re-parse returns.
		_ = fs.Parse(rest)
		if fs.NArg() > 0 {
			usage(fs.Args())
		}
	}

	switch subcommand {
	case "validate":
		runConfigValidate()
	case "show":
		runConfigShow()
	default:
		usage(args)
	}
}

// runConfigValidate implements `armor config validate`: load and validate the
// configuration with no network probes, reporting every validation error
// grouped one per line. Exit 0 with a single confirmation line when the
// configuration loads; exit 1 with the grouped errors on stderr when it does
// not — the exit code is the script-facing verdict.
func runConfigValidate() {
	_, err := config.Load()
	if err == nil {
		fmt.Println("Configuration valid")
		return
	}

	problems := joinedErrorList(err)
	fmt.Fprintf(os.Stderr, "Configuration invalid: %d error%s\n", len(problems), plural(len(problems)))
	for _, problem := range problems {
		fmt.Fprintf(os.Stderr, "  - %s\n", problem)
	}
	exit(1)
}

// runConfigShow implements `armor config show`: print every resolved setting
// with its source, secrets redacted. Values render exactly as
// config.SettingRows redacts them — secret material and unpublished
// identifiers never appear, so the output is safe to paste into a ticket.
//
// A configuration that fails to load has no resolved settings to show: the
// command reports the failure and points at `armor config validate` for the
// grouped report rather than dumping a half-resolved table.
func runConfigShow() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: configuration failed to load, so there are no resolved settings to show:\n")
		for _, problem := range joinedErrorList(err) {
			fmt.Fprintf(os.Stderr, "  - %s\n", problem)
		}
		fmt.Fprintf(os.Stderr, "Run 'armor config validate' for the same report with the exit-code contract.\n")
		exit(1)
	}

	// Unrecognized ARMOR_* variables are not errors, but this command is
	// exactly where an operator compares what they set against what ARMOR
	// resolved, so surface them the way `armor check` does.
	for _, warning := range cfg.EnvWarnings {
		fmt.Fprintf(os.Stderr, "[WARN] %s\n", warning)
	}

	rows := cfg.Settings()
	if configJSONFlag {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(rows); err != nil {
			fmt.Fprintf(os.Stderr, "Error: unable to encode settings as JSON: %v\n", err)
			exit(1)
		}
		return
	}
	printSettingRows(os.Stdout, rows)
}

// printSettingRows writes the human table: fixed-order rows in three
// space-padded columns (setting, value, source) under a header. Widths come
// from the rows themselves, so long dynamic values (credential fingerprints,
// ring lists) stay aligned without truncation.
func printSettingRows(w io.Writer, rows []config.SettingRow) {
	nameWidth, valueWidth := len("SETTING"), len("VALUE")
	for _, row := range rows {
		if len(row.Name) > nameWidth {
			nameWidth = len(row.Name)
		}
		if len(row.Value) > valueWidth {
			valueWidth = len(row.Value)
		}
	}
	fmt.Fprintf(w, "%-*s  %-*s  %s\n", nameWidth, "SETTING", valueWidth, "VALUE", "SOURCE")
	for _, row := range rows {
		fmt.Fprintf(w, "%-*s  %-*s  %s\n", nameWidth, row.Name, valueWidth, row.Value, row.Source)
	}
}

// joinedErrorList flattens an errors.Join result (or any error) into its
// per-problem lines, so each validation failure renders on its own line.
func joinedErrorList(err error) []string {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		var problems []string
		for _, e := range joined.Unwrap() {
			problems = append(problems, e.Error())
		}
		if len(problems) > 0 {
			return problems
		}
	}
	return strings.Split(err.Error(), "\n")
}

// plural returns "s" when n is not 1 — the only English plural this file needs.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
