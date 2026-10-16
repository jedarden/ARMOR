// Package main implements the armor-fleet command.
package main

import (
	"flag"
	"fmt"
	"os"
)

// The command line is registered at package level, not inside main, so the
// registry is static: the CLI-reference smoke tests (cli_reference_test.go)
// read exactly these flags against docs/companion-cli-reference.md.
var (
	targetsFile  = flag.String("targets", "", "Path to targets YAML file")
	listen       = flag.String("listen", ":8080", "Address to listen on")
	pollInterval = flag.Int("interval", 60, "Poll interval in seconds")
	// Registered but rejected when set: a SEAM token on the command line
	// lands in shell history, ps, and session transcripts. Keeping the
	// registration means a deployment still carrying the flag fails with a
	// diagnostic naming the environment variable instead of a bare "flag
	// provided but not defined".
	seamToken = flag.String("seam-token", "", "DEPRECATED: rejected — set SEAM_TOKEN instead")
)

func main() {
	flag.Parse()

	if *seamToken != "" {
		fmt.Fprintf(os.Stderr, "Error: -seam-token no longer accepts the SEAM token: a value on the command line lands in shell history, ps, and transcripts. Set the SEAM_TOKEN environment variable instead\n")
		os.Exit(2)
	}

	if *targetsFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -targets flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor-fleet -targets <file> [-listen <addr>] [-interval <seconds>]\n")
		os.Exit(2)
	}

	// Get SEAM token from the environment — the only source
	token := os.Getenv("SEAM_TOKEN")
	if token == "" {
		fmt.Fprintf(os.Stderr, "Error: SEAM token required: set the SEAM_TOKEN environment variable\n")
		os.Exit(1)
	}

	// Load targets
	targets, err := LoadTargets(*targetsFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading targets: %v\n", err)
		os.Exit(1)
	}

	if len(targets) == 0 {
		fmt.Fprintf(os.Stderr, "Error: no targets found in %s\n", *targetsFile)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d targets from %s\n", len(targets), *targetsFile)

	// Create and start fleet monitor
	monitor := NewFleetMonitor(targets, token, *pollInterval)
	monitor.Start()

	// Start HTTP server
	server := NewServer(monitor, *listen)
	fmt.Fprintf(os.Stderr, "Starting fleet server on %s\n", *listen)
	if err := server.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
