// Package main provides the demo subcommand.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server"
)

// demo flags
var (
	demoDirFlag      string
	demoListenFlag   string
	demoAdminListenFlag string
)

func init() {
	// Register demo command
	registerCommand(Command{
		Name:        "demo",
		Description: "Start ARMOR in demo mode with filesystem backend and fixed credentials",
		Func:        demo,
	})

	// demo-specific flags - these are parsed in the demo() function
	flag.StringVar(&demoDirFlag, "dir", "", "Directory for filesystem backend (default: temp directory, removed on exit)")
	flag.StringVar(&demoListenFlag, "listen", "127.0.0.1:9000", "S3 API listen address")
	flag.StringVar(&demoAdminListenFlag, "admin-listen", "127.0.0.1:9001", "Admin API listen address")
}

// demo implements the demo subcommand.
func demo() {
	// Parse flags
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", flag.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor demo [--dir DIR] [--listen ADDR] [--admin-listen ADDR]\n")
		os.Exit(2)
	}

	// Generate a random MEK in memory (32 bytes)
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate MEK: %v\n", err)
		os.Exit(1)
	}
	mekHex := hex.EncodeToString(mek)

	// Create temp directory if --dir not provided
	tempDir := ""
	demoDir := demoDirFlag
	cleanupNeeded := false
	if demoDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "armor-demo-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create temp directory: %v\n", err)
			os.Exit(1)
		}
		demoDir = tempDir
		cleanupNeeded = true
	} else {
		// Validate --dir exists or can be created
		if err := os.MkdirAll(demoDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create directory %s: %v\n", demoDir, err)
			os.Exit(1)
		}
	}

	// Set demo configuration via environment variables
	os.Setenv("ARMOR_BACKEND", "filesystem")
	os.Setenv("ARMOR_FS_PATH", demoDir)
	os.Setenv("ARMOR_BUCKET", "demo-bucket")
	os.Setenv("ARMOR_MEK", mekHex)
	os.Setenv("ARMOR_AUTH_ACCESS_KEY", "armor")
	os.Setenv("ARMOR_AUTH_SECRET_KEY", "armor-demo-secret")
	os.Setenv("ARMOR_ALLOW_NO_CREDENTIALS", "true")
	os.Setenv("ARMOR_LISTEN", demoListenFlag)
	os.Setenv("ARMOR_ADMIN_LISTEN", demoAdminListenFlag)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		if cleanupNeeded {
			os.RemoveAll(tempDir)
		}
		os.Exit(1)
	}

	// Create logger with configuration
	logger := logging.New("armor-demo")
	logging.SetDefault(logger)

	// Print demo banner
	logger.Info("ARMOR demo mode")
	logger.WithField("dir", demoDir).Info("Filesystem backend")
	logger.WithField("listen", cfg.Listen).Info("S3 API")
	logger.WithField("admin_listen", cfg.AdminListen).Info("Admin API")
	logger.Info("")
	logger.Info("Demo credentials:")
	logger.Info("  Access Key ID: armor")
	logger.Info("  Secret Access Key: armor-demo-secret")
	logger.Info("")

	// Print AWS CLI configuration
	printAWSCLIConfig(cfg.Listen, cfg.Bucket)

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	// Setup signal handling for cleanup
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Create HTTP servers
	httpServer := newS3HTTPServer(cfg.Listen, srv.Handler())

	adminServer, err := newAdminHTTPServer(cfg.AdminListen, srv.AdminHandler(), os.Getenv)
	if err != nil {
		logger.Fatalf("failed to configure admin server: %v", err)
	}

	// Start servers in goroutines
	go func() {
		logger.WithField("addr", cfg.Listen).Info("S3 API listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("S3 server error: %v", err)
		}
	}()

	go func() {
		logger.WithField("addr", cfg.AdminListen).Info("Admin API listening")
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Admin server error: %v", err)
		}
	}()

	logger.Info("")
	logger.Info("Press Ctrl-C to stop")

	// Wait for interrupt signal
	sig := <-quit
	logger.WithField("signal", sig.String()).Info("shutdown signal received")

	// Graceful shutdown
	logger.Info("stopping HTTP servers")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.WithField("error", err).Error("S3 server shutdown error")
	}
	if err := adminServer.Shutdown(ctx); err != nil {
		logger.WithField("error", err).Error("Admin server shutdown error")
	}

	logger.Info("servers stopped")

	// Clean up temp directory if we created it
	if cleanupNeeded && tempDir != "" {
		logger.WithField("dir", tempDir).Info("removing temp directory")
		if err := os.RemoveAll(tempDir); err != nil {
			logger.WithField("error", err).Warn("failed to remove temp directory")
		}
	}

	logger.Info("ARMOR demo shutdown complete")
}

// printAWSCLIConfig prints the AWS CLI configuration for connecting to the demo server.
func printAWSCLIConfig(endpoint, bucket string) {
	fmt.Println("")
	fmt.Println("=== AWS CLI Configuration ===")
	fmt.Println("")
	fmt.Println("# Set these environment variables:")
	fmt.Println("export AWS_ACCESS_KEY_ID=armor")
	fmt.Println("export AWS_SECRET_ACCESS_KEY=armor-demo-secret")
	fmt.Println("")
	fmt.Println("# Test the connection:")
	fmt.Printf("aws --endpoint-url http://%s s3 ls s3://%s\n", endpoint, bucket)
	fmt.Println("")
	fmt.Println("Or use --profile:")
	fmt.Printf("aws configure set profile.demo.endpoint_url http://%s\n", endpoint)
	fmt.Println("aws configure set profile.demo.s3.addressing_style path")
	fmt.Println("")
	fmt.Println("=============================")
	fmt.Println("")
}
