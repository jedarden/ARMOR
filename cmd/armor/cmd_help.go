// Package main provides the help subcommand.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func init() {
	registerCommand(Command{
		Name:        "help",
		Description: "Show help information",
		Func:        help,
	})
}

// help implements `armor help`: the top-level help (one-line description,
// subcommand table, per-command help hint). Extra arguments are ignored —
// asked-for help should never be an error.
func help(_ *flag.FlagSet) {
	printTopLevelHelp(os.Stdout)
}

// printTopLevelHelp writes the top-level help: what the binary is, how to
// invoke it, the subcommand table with a one-line summary each, and where to
// find per-subcommand flags. It deliberately lists no flags itself — they
// belong to the subcommands and live behind `armor <cmd> --help`.
func printTopLevelHelp(w io.Writer) {
	fmt.Fprintf(w, "ARMOR - S3-compatible object storage server\n\n")
	fmt.Fprintf(w, "Usage: armor [subcommand] [flags]\n\n")
	fmt.Fprintf(w, "Available subcommands:\n")
	listCommands(w)
	fmt.Fprintf(w, "\nRun armor <subcommand> --help for its flags.\n")
	fmt.Fprintf(w, "With no subcommand, armor serves.\n")
}
