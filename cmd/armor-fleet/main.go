// Package main implements the armor-fleet command.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Parse flags
	targetsFile := flag.String("targets", "", "Path to targets YAML file")
	listen := flag.String("listen", ":8080", "Address to listen on")
	pollInterval := flag.Int("interval", 60, "Poll interval in seconds")
	seamToken := flag.String("seam-token", "", "SEAM bearer token (or set SEAM_TOKEN env var)")
	flag.Parse()

	if *targetsFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -targets flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: armor-fleet -targets <file> [-listen <addr>] [-interval <seconds>] [-seam-token <token>]\n")
		os.Exit(2)
	}

	// Get SEAM token from flag or env
	token := *seamToken
	if token == "" {
		token = os.Getenv("SEAM_TOKEN")
	}
	if token == "" {
		fmt.Fprintf(os.Stderr, "Error: SEAM token required via -seam-token flag or SEAM_TOKEN env var\n")
		os.Exit(2)
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
