// Package main provides the demo subcommand.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server"
)

// demo flags
var (
	demoDirFlag         string
	demoMEKFileFlag     string
	demoListenFlag      string
	demoAdminListenFlag string
)

func init() {
	// demo-specific flags, on demo's own flag set
	demoFlags := flag.NewFlagSet("demo", flag.ExitOnError)
	demoFlags.StringVar(&demoDirFlag, "dir", "", "Directory for filesystem backend (default: temp directory, removed on exit)")
	demoFlags.StringVar(&demoMEKFileFlag, "mek-file", "", "File for persistent MEK (default: <dir>/.armor-demo-mek when -dir is set)")
	demoFlags.StringVar(&demoListenFlag, "listen", "127.0.0.1:9000", "S3 API listen address")
	demoFlags.StringVar(&demoAdminListenFlag, "admin-listen", "127.0.0.1:9001", "Admin API listen address")

	// Register demo command
	registerCommand(Command{
		Name:        "demo",
		Description: "Start ARMOR demo with filesystem backend and generated dashboard credentials",
		Flags:       demoFlags,
		Func:        demo,
	})
}

// demo implements the demo subcommand.
func demo(fs *flag.FlagSet) {
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments after flags: %v\n", fs.Args())
		fmt.Fprintf(os.Stderr, "Usage: armor demo [--dir DIR] [--mek-file FILE] [--listen ADDR] [--admin-listen ADDR]\n")
		os.Exit(2)
	}

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

	mekFile := demoMEKFilePath(demoDir, demoDirFlag != "", demoMEKFileFlag)
	mek, err := demoMEK(mekFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load demo MEK: %v\n", err)
		if cleanupNeeded {
			os.RemoveAll(tempDir)
		}
		os.Exit(1)
	}

	// Set demo configuration via environment variables and load it through the
	// same path used by the dashboard smoke test.
	cfg, credentials, err := configureDemo(demoDir, hex.EncodeToString(mek), demoListenFlag, demoAdminListenFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		if cleanupNeeded {
			os.RemoveAll(tempDir)
		}
		os.Exit(1)
	}
	if err := initializeDemoBucket(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize demo bucket: %v\n", err)
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
	logger.WithFields(map[string]interface{}{
		"dashboard_user": credentials.dashboardUser,
		"dashboard_pass": credentials.dashboardPass,
		"admin_token":    credentials.adminToken,
	}).Info("Demo dashboard credentials")
	logger.WithField("url", dashboardURL(cfg.AdminListen)).Info("Dashboard")
	logger.WithField("access_key_id", "armor").Info("S3 access key")
	logger.WithField("secret_access_key", "armor-demo-secret").Info("S3 secret key")

	// The ready-to-paste AWS CLI block is plain text, separate from the JSON
	// logger lines above.
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

type demoCredentials struct {
	dashboardUser string
	dashboardPass string
	adminToken    string
}

// configureDemo sets the demo's fixed S3 configuration, fills missing
// dashboard credentials with random values, and returns the loaded config.
func configureDemo(demoDir, mekHex, listen, adminListen string) (*config.Config, demoCredentials, error) {
	credentials := demoCredentials{
		dashboardUser: os.Getenv("ARMOR_DASHBOARD_USER"),
		dashboardPass: os.Getenv("ARMOR_DASHBOARD_PASS"),
		adminToken:    strings.TrimSpace(os.Getenv("ARMOR_ADMIN_TOKEN")),
	}
	if credentials.dashboardUser == "" {
		id, err := randomDemoHex(4)
		if err != nil {
			return nil, demoCredentials{}, fmt.Errorf("generate dashboard username: %w", err)
		}
		credentials.dashboardUser = "demo-" + id
	}
	if credentials.dashboardPass == "" {
		value, err := randomDemoHex(32)
		if err != nil {
			return nil, demoCredentials{}, fmt.Errorf("generate dashboard password: %w", err)
		}
		credentials.dashboardPass = value
	}
	if credentials.adminToken == "" {
		value, err := randomDemoHex(32)
		if err != nil {
			return nil, demoCredentials{}, fmt.Errorf("generate admin token: %w", err)
		}
		credentials.adminToken = value
	}

	for name, value := range map[string]string{
		"ARMOR_BACKEND":              "filesystem",
		"ARMOR_FS_PATH":              demoDir,
		"ARMOR_BUCKET":               "demo-bucket",
		"ARMOR_MEK":                  mekHex,
		"ARMOR_AUTH_ACCESS_KEY":      "armor",
		"ARMOR_AUTH_SECRET_KEY":      "armor-demo-secret",
		"ARMOR_ALLOW_NO_CREDENTIALS": "true",
		"ARMOR_LISTEN":               listen,
		"ARMOR_ADMIN_LISTEN":         adminListen,
		"ARMOR_DASHBOARD_USER":       credentials.dashboardUser,
		"ARMOR_DASHBOARD_PASS":       credentials.dashboardPass,
		"ARMOR_ADMIN_TOKEN":          credentials.adminToken,
	} {
		if err := os.Setenv(name, value); err != nil {
			return nil, demoCredentials{}, fmt.Errorf("set %s: %w", name, err)
		}
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, demoCredentials{}, err
	}
	return cfg, credentials, nil
}

func initializeDemoBucket(cfg *config.Config) error {
	filesystem, err := backend.NewFSBackend(backend.FSConfig{BasePath: cfg.FSPath})
	if err != nil {
		return fmt.Errorf("create filesystem backend: %w", err)
	}
	if err := filesystem.CreateBucket(context.Background(), cfg.Bucket); err != nil {
		return fmt.Errorf("create bucket %s: %w", cfg.Bucket, err)
	}
	return nil
}

func randomDemoHex(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// demoMEKFilePath resolves relative custom paths under -dir. With -dir and no
// custom path, the MEK is persisted in the data directory. Without -dir, the
// default MEK remains in memory so the temporary demo stays disposable.
func demoMEKFilePath(demoDir string, persistentDir bool, requested string) string {
	if requested == "" {
		if persistentDir {
			return filepath.Join(demoDir, ".armor-demo-mek")
		}
		return ""
	}
	if filepath.IsAbs(requested) || !persistentDir {
		return requested
	}
	return filepath.Join(demoDir, requested)
}

// demoMEK returns a fresh key when no path is selected, or atomically creates
// a mode-0600 key file and reuses its contents on subsequent runs.
func demoMEK(path string) ([]byte, error) {
	if path == "" {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		return key, nil
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create MEK directory: %w", err)
	}
	if key, err := readDemoMEK(path); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(dir, ".armor-demo-mek-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary MEK file: %w", err)
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("set MEK file permissions: %w", err)
	}
	if _, err := file.WriteString(hex.EncodeToString(key)); err != nil {
		file.Close()
		return nil, fmt.Errorf("write MEK file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return nil, fmt.Errorf("sync MEK file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close MEK file: %w", err)
	}
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return readDemoMEK(path)
		}
		return nil, fmt.Errorf("install MEK file: %w", err)
	}
	return key, nil
}

func readDemoMEK(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("MEK path %s is not a regular file", path)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read MEK file: %w", err)
	}
	key, err := hex.DecodeString(string(encoded))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("MEK file %s must contain exactly 64 hexadecimal characters", path)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("set MEK file permissions: %w", err)
	}
	return key, nil
}

func dashboardURL(adminListen string) string {
	if strings.HasPrefix(adminListen, ":") {
		adminListen = "127.0.0.1" + adminListen
	}
	return (&url.URL{Scheme: "http", Host: adminListen, Path: "/dashboard"}).String()
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
