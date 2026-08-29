// Package main provides the help subcommand.
package main

import (
	"fmt"
	"os"
)

func init() {
	registerCommand(Command{
		Name:        "help",
		Description: "Show help information",
		Func:        help,
	})
}

func help() {
	fmt.Printf("ARMOR - S3-compatible object storage server\n\n")
	fmt.Printf("Usage: armor [subcommand]\n\n")
	fmt.Printf("Available subcommands:\n")
	listCommands(os.Stdout)
	fmt.Printf("\nIf no subcommand is provided, 'serve' is the default.\n")
}
