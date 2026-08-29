// armor-decrypt is a thin compatibility wrapper for the 'armor decrypt' subcommand.
//
// This binary is maintained for one release cycle to provide backward compatibility.
// It simply execs the 'armor' binary with the 'decrypt' subcommand and all arguments.
//
// New code should use 'armor decrypt' directly.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	// Find the armor binary - look in the same directory as this binary
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine executable path: %v\n", err)
		os.Exit(1)
	}

	armorPath := filepath.Join(filepath.Dir(execPath), "armor")

	// Build the command: armor decrypt [args...]
	args := append([]string{"decrypt"}, os.Args[1:]...)

	cmd := exec.Command(armorPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Exit with the same status code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "Error executing armor decrypt: %v\n", err)
		os.Exit(1)
	}
}
