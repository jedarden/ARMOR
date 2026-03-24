// Package main is the entry point for the ARMOR server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log startup info
	log.Printf("ARMOR starting...")
	log.Printf("Listen: %s", cfg.Listen)
	log.Printf("Admin Listen: %s", cfg.AdminListen)
	log.Printf("Bucket: %s", cfg.Bucket)
	log.Printf("Cloudflare Domain: %s", cfg.CFDomain)
	log.Printf("Block Size: %d", cfg.BlockSize)
	log.Printf("Writer ID: %s", cfg.WriterID)
	log.Printf("Auth Access Key: %s", cfg.AuthAccessKey)

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
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

	// Start servers in goroutines
	go func() {
		log.Printf("S3 API listening on %s", cfg.Listen)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("S3 server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Admin API listening on %s", cfg.AdminListen)
		if err := adminServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Admin server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("S3 server shutdown error: %v", err)
	}

	if err := adminServer.Shutdown(ctx); err != nil {
		log.Printf("Admin server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}
