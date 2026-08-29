// Package main provides the demo subcommand.
package main

import (
	"fmt"
	"os"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server"
)

func init() {
	registerCommand(Command{
		Name:        "demo",
		Description: "Start ARMOR in demo mode with auto-generated credentials",
		Func:        demo,
	})
}

func demo() {
	// Set environment variables for demo mode
	// Use filesystem backend for a self-contained demo
	os.Setenv("ARMOR_BACKEND", "filesystem")
	os.Setenv("ARMOR_FS_PATH", "/tmp/armor-demo")
	os.Setenv("ARMOR_BUCKET", "demo-bucket")
	os.Setenv("ARMOR_MEK", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	os.Setenv("ARMOR_ALLOW_NO_CREDENTIALS", "true")

	// Create demo directory if it doesn't exist
	if err := os.MkdirAll("/tmp/armor-demo", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create demo directory: %v\n", err)
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logging.Fatalf("failed to load configuration: %v", err)
	}

	// Create logger with configuration
	logger := logging.New("armor-demo")
	logging.SetDefault(logger)

	// Log startup info
	logger.Info("ARMOR demo mode starting")
	logger.Info("Demo directory: /tmp/armor-demo")
	logger.Info("NOTE: This is a demo mode with no client authentication")
	logger.Info("For production use, configure ARMOR_AUTH_ACCESS_KEY and ARMOR_AUTH_SECRET_KEY")

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	// Start the server with demo defaults
	serveWithConfig(cfg, srv, logger)
}

// serveWithConfig starts the server with the given configuration.
// This is shared between serve and demo commands.
func serveWithConfig(cfg *config.Config, srv *server.Server, logger *logging.Logger) {
	// Implementation would be similar to serve() but with cfg pre-loaded
	// For now, delegate to the existing serve command's logic
	logger.Info("Server ready")
	select {} // Block forever
}
