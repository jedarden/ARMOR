// Package main provides the serve subcommand.
package main

import (
	"context"
	"errors"
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
	httpServer := &http.Server{
		Addr:         cfg.Listen,
		Handler:      srv.Handler(),
		ReadTimeout:  30 * time.Minute, // Long timeout for large uploads
		WriteTimeout: 30 * time.Minute, // Long timeout for large downloads
	}

	// Create admin HTTP server
	adminServer := &http.Server{
		Addr:         cfg.AdminListen,
		Handler:      srv.AdminHandler(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
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

	logger.Info("ARMOR shutdown complete")
}
