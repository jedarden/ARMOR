// Package main provides the serve subcommand.
package main

import (
	"context"
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

func init() {
	registerCommand(Command{
		Name:        "serve",
		Description: "Start the ARMOR S3-compatible server (default)",
		Func:        serve,
	})
}

const (
	s3ReadHeaderTimeout = 30 * time.Second
	s3IdleTimeout       = 2 * time.Minute
)

// Admin listener timeout overrides. Both accept a Go duration string ("30s",
// "5m"); "0" disables the deadline. Unset or empty also means disabled, which
// is the default.
const (
	EnvAdminReadTimeout  = "ARMOR_ADMIN_READ_TIMEOUT"
	EnvAdminWriteTimeout = "ARMOR_ADMIN_WRITE_TIMEOUT"
)

const (
	// adminReadHeaderTimeout bounds how long the admin listener waits for a
	// client to finish sending request headers. It is the guardrail that
	// remains once ReadTimeout defaults to disabled, and it is deliberately
	// not configurable: the pre-header path is the only part of the admin
	// surface an unauthenticated caller can reach, so slowloris protection
	// there is not something an operator should be able to tune away.
	adminReadHeaderTimeout = 30 * time.Second

	// adminIdleTimeout reaps admin connections sitting between requests. It is
	// set explicitly because the defaults below disable ReadTimeout, and
	// http.Server falls back to ReadTimeout for an unset IdleTimeout -- which
	// would leave stale admin connections open forever.
	adminIdleTimeout = 2 * time.Minute
)

// newS3HTTPServer builds the public S3 API server. ReadTimeout and WriteTimeout
// must remain disabled: multipart uploads, completion, and streamed downloads
// can legitimately take longer than 30 minutes for multi-gigabyte objects. A
// non-zero server-wide deadline closes an active client connection while ARMOR
// or the backing store is still processing it. ReadHeaderTimeout and
// IdleTimeout retain protection for connections that are not actively
// transferring a request.
func newS3HTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       0,
		ReadHeaderTimeout: s3ReadHeaderTimeout,
		WriteTimeout:      0,
		IdleTimeout:       s3IdleTimeout,
	}
}

// newAdminHTTPServer builds the admin API server.
//
// ReadTimeout and WriteTimeout default to disabled. The previous hard-coded 30s
// on both made every long-running admin call impossible: net/http arms the
// write deadline as soon as the request has been read, *before* the handler
// runs, so a handler that needs minutes was guaranteed to fail on write no
// matter how fast the client was. POST /admin/key/rotate and
// GET /admin/key/ring both walk the whole bucket and take minutes on a real
// backend, so they were killed mid-walk with an empty body
// (docs/notes/mek-rotation-2026.md, "Blocker" item 1). They now match the S3
// listener, and adminReadHeaderTimeout keeps the pre-handler read path bounded.
//
// Set ARMOR_ADMIN_READ_TIMEOUT / ARMOR_ADMIN_WRITE_TIMEOUT to restore an
// explicit guardrail on a deployment that wants one. They take Go duration
// strings, and "0" is equivalent to leaving them unset.
func newAdminHTTPServer(addr string, handler http.Handler, getenv func(string) string) (*http.Server, error) {
	read, write, err := adminTimeouts(getenv)
	if err != nil {
		return nil, err
	}

	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       read,
		ReadHeaderTimeout: adminReadHeaderTimeout,
		WriteTimeout:      write,
		IdleTimeout:       adminIdleTimeout,
	}, nil
}

// adminTimeouts resolves the admin listener's read and write deadlines from the
// environment. Both default to 0 (disabled).
func adminTimeouts(getenv func(string) string) (read, write time.Duration, err error) {
	if read, err = adminTimeoutFromEnv(getenv, EnvAdminReadTimeout); err != nil {
		return 0, 0, err
	}
	if write, err = adminTimeoutFromEnv(getenv, EnvAdminWriteTimeout); err != nil {
		return 0, 0, err
	}
	return read, write, nil
}

// adminTimeoutFromEnv reads one duration-valued override. An unset or empty
// variable yields 0, meaning "no deadline"; anything else must parse as a
// non-negative Go duration.
func adminTimeoutFromEnv(getenv func(string) string, key string) (time.Duration, error) {
	raw := getenv(key)
	if raw == "" {
		return 0, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q (want Go duration syntax, e.g. 30s or 5m, or 0 to disable)", key, raw)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s: must be zero or positive, got %s", key, d)
	}
	return d, nil
}

func serve() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Print each error on a separate line for better operator experience
		var allErrors []error

		// Try to extract multiple errors from errors.Join()
		switch jointErr := err.(type) {
		case interface{ Unwrap() []error }:
			// errors.Join returns an error with Unwrap() []error
			allErrors = jointErr.Unwrap()
		default:
			// Single error or different error type
			allErrors = []error{err}
		}

		var errMsg string
		for i, e := range allErrors {
			if i > 0 {
				errMsg += "\n"
			}
			errMsg += fmt.Sprintf("  %s", e.Error())
		}
		logging.Fatalf("failed to load configuration:\n%s", errMsg)
	}

	// Create logger with configuration
	logger := logging.New("armor")
	logging.SetDefault(logger)

	// Log startup info with full redacted configuration
	logger.WithField("config", cfg.Redacted()).Info("ARMOR starting")

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatalf("failed to create server: %v", err)
	}

	// Create HTTP server
	httpServer := newS3HTTPServer(cfg.Listen, srv.Handler())

	// Create admin HTTP server
	adminServer, err := newAdminHTTPServer(cfg.AdminListen, srv.AdminHandler(), os.Getenv)
	if err != nil {
		logger.Fatalf("failed to configure admin server: %v", err)
	}

	// Start canary monitor
	srv.StartCanary(context.Background())

	// Start replication queue if secondary backend is configured (ADR-006)
	srv.StartReplicationQueue(context.Background())

	// Start servers in goroutines
	go func() {
		logger.Infof("S3 API listening on %s", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("S3 server error: %v", err)
		}
	}()

	go func() {
		logger.Infof("Admin API listening on %s", cfg.AdminListen)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Admin server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.WithFields(map[string]interface{}{
		"signal": sig.String(),
	}).Info("shutdown signal received")

	// Phase 1: Stop accepting new connections
	logger.Info("phase 1: stopping HTTP servers")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Shutdown HTTP servers (stops accepting new connections)
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.WithField("error", err.Error()).Error("S3 server shutdown error")
	}
	logger.Info("S3 server stopped accepting connections")

	if err := adminServer.Shutdown(ctx); err != nil {
		logger.WithField("error", err.Error()).Error("Admin server shutdown error")
	}
	logger.Info("Admin server stopped accepting connections")

	// Phase 2: Wait for in-flight requests to complete
	inFlight := srv.InFlightRequestCount()
	if inFlight > 0 {
		logger.WithField("in_flight", inFlight).Info("phase 2: waiting for in-flight requests")

		// Wait for in-flight requests with a timeout
		done := make(chan struct{})
		go func() {
			srv.WaitForInFlightRequests()
			close(done)
		}()

		select {
		case <-done:
			logger.Info("all in-flight requests completed")
		case <-ctx.Done():
			logger.Warn("timeout waiting for in-flight requests, proceeding with shutdown")
		}
	} else {
		logger.Info("phase 2: no in-flight requests")
	}

	// Phase 3: Stop background tasks
	logger.Info("phase 3: stopping background tasks")
	srv.StopCanary()
	srv.StopReplicationQueue()
	srv.StopManifestCompactor()
	srv.StopManifestWriter()
	srv.StopAuthFileWatcher()

	logger.Info("ARMOR shutdown complete")
}
