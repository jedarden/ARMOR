// restore-verifier is a standalone service for continuous backup verification.
// It runs dual-path verification (ARMOR read path + armor-decrypt direct) to prove
// that backups are restorable through both the normal server path and disaster recovery.
//
// Usage:
//   restore-verifier [flags]
//
// Environment Variables:
//   ARMOR_B2_REGION              B2 region (e.g., us-west-004)
//   ARMOR_B2_ENDPOINT            B2 S3 API endpoint
//   ARMOR_B2_ACCESS_KEY_ID       B2 application key ID
//   ARMOR_B2_SECRET_ACCESS_KEY   B2 application key
//   ARMOR_CF_DOMAIN              Cloudflare domain (optional)
//   ARMOR_MEK                    Master encryption key (hex, 64 chars)
//   ARMOR_BLOCK_SIZE             Encryption block size (default: 65536)
//   VERIFIER_CHECK_INTERVAL      How often to run verification (default: 6h)
//   VERIFIER_SAMPLE_SIZE         Number of historical objects to sample (default: 10)
//   VERIFIER_HTTP_LISTEN         HTTP listen address (default: :9002)
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/restoreverifier"
)

var (
	// Configuration flags
	b2Region      = flag.String("b2-region", os.Getenv("ARMOR_B2_REGION"), "B2 region")
	b2Endpoint    = flag.String("b2-endpoint", os.Getenv("ARMOR_B2_ENDPOINT"), "B2 S3 endpoint")
	b2AccessKey   = flag.String("b2-access-key", os.Getenv("ARMOR_B2_ACCESS_KEY_ID"), "B2 access key ID")
	b2SecretKey   = flag.String("b2-secret-key", os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY"), "B2 secret key")
	cfDomain      = flag.String("cf-domain", os.Getenv("ARMOR_CF_DOMAIN"), "Cloudflare domain")
	mekHex        = flag.String("mek", os.Getenv("ARMOR_MEK"), "Master encryption key (hex)")
	blockSize     = flag.Int("block-size", 65536, "Encryption block size")

	// Verifier configuration
	checkInterval = flag.Duration("check-interval", parseDuration(os.Getenv("VERIFIER_CHECK_INTERVAL"), 6*time.Hour), "Verification check interval")
	sampleSize    = flag.Int("sample-size", parseInt(os.Getenv("VERIFIER_SAMPLE_SIZE"), 10), "Number of historical objects to sample")
	httpListen    = flag.String("http-listen", os.Getenv("VERIFIER_HTTP_LISTEN"), "HTTP listen address (default :9002)")

	// Bucket configuration (can be specified multiple times)
	bucketFlag bucketFlags
)

func init() {
	flag.Var(&bucketFlag, "bucket", "Buckets to verify (format: name[,prefix][,artifact_type][,enabled]). Can be specified multiple times.")
}

type bucketFlags []restoreverifier.BucketConfig

func (b *bucketFlags) String() string {
	return fmt.Sprintf("%v", *b)
}

func (b *bucketFlags) Set(value string) error {
	// Parse format: name[,prefix][,artifact_type][,enabled]
	var cfg restoreverifier.BucketConfig

	parts := splitCSV(value)
	if len(parts) == 0 {
		return errors.New("empty bucket config")
	}

	cfg.Bucket = parts[0]
	cfg.Enabled = true // default enabled

	if len(parts) > 1 && parts[1] != "" {
		cfg.Prefix = parts[1]
	}
	if len(parts) > 2 && parts[2] != "" {
		cfg.ArtifactType = restoreverifier.ArtifactType(parts[2])
	}
	if len(parts) > 3 && parts[3] != "" {
		cfg.Enabled = parts[3] == "true" || parts[3] == "1"
	}
	if len(parts) > 4 && parts[4] != "" {
		cfg.HistoricalSampleSize = parseInt(parts[4], *sampleSize)
	} else {
		cfg.HistoricalSampleSize = *sampleSize
	}

	*b = append(*b, cfg)
	return nil
}

func main() {
	flag.Usage = usage
	flag.Parse()

	// Derive the B2 S3 endpoint from the region when not supplied, mirroring the
	// ARMOR server (internal/config/config.go). This lets each per-cluster
	// restore-verifier Deployment reuse the cluster's existing ARMOR_B2_REGION
	// (ConfigMap or ExternalSecret) without hardcoding an endpoint per cluster.
	if *b2Endpoint == "" && *b2Region != "" {
		*b2Endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", *b2Region)
	}

	// Validate required flags
	if *b2Region == "" || *b2Endpoint == "" || *b2AccessKey == "" || *b2SecretKey == "" {
		log.Fatal("Missing required B2 credentials. Set ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY")
	}

	if *mekHex == "" {
		log.Fatal("Missing MEK. Set ARMOR_MEK environment variable or -mek flag")
	}

	// Decode MEK
	mek, err := hex.DecodeString(*mekHex)
	if err != nil {
		log.Fatalf("Invalid MEK hex: %v", err)
	}
	if len(mek) != 32 {
		log.Fatalf("Invalid MEK length: got %d bytes, expected 32", len(mek))
	}

	log.Printf("Starting restore-verifier with block size %d, check interval %v, sample size %d",
		*blockSize, *checkInterval, *sampleSize)

	// Initialize B2 backend
	ctx := context.Background()
	b2Backend, err := backend.NewB2Backend(ctx, backend.B2Config{
		Region:      *b2Region,
		Endpoint:    *b2Endpoint,
		AccessKeyID: *b2AccessKey,
		SecretKey:   *b2SecretKey,
		CFDomain:    *cfDomain,
	})
	if err != nil {
		log.Fatalf("Failed to initialize B2 backend: %v", err)
	}

	// If no buckets specified via -bucket flags, fall back to the ARMOR_BUCKET
	// environment variable (single bucket). Every ARMOR cluster already sets
	// ARMOR_BUCKET for its server (via ConfigMap or ExternalSecret), so each
	// per-cluster restore-verifier Deployment can stay uniform — the bucket name
	// is sourced from the cluster's existing config rather than hardcoded here.
	// (A prior revision listed all six logical buckets as a default; that was
	// wrong, since a single instance holds one MEK and one B2 credential set
	// and can only verify the bucket it has keys for.)
	if len(bucketFlag) == 0 {
		if envBucket := os.Getenv("ARMOR_BUCKET"); envBucket != "" {
			log.Printf("No -bucket flags; verifying ARMOR_BUCKET=%q from environment", envBucket)
			bucketFlag = append(bucketFlag, restoreverifier.BucketConfig{
				Bucket:               envBucket,
				ArtifactType:         restoreverifier.ArtifactGeneric,
				Enabled:              true,
				HistoricalSampleSize: *sampleSize,
			})
		} else {
			log.Fatal("No buckets configured: pass -bucket flags or set ARMOR_BUCKET")
		}
	}

	// Initialize manifest index (for metadata cache, optional but recommended)
	manifestIndex := manifest.New()

	// Initialize metrics
	metricsCollector := metrics.NewMetrics()

	// Create verifier
	cfg := restoreverifier.Config{
		Buckets:       bucketFlag,
		Interval:      *checkInterval,
		SampleSize:    *sampleSize,
		EscrowMEKPath: "", // MEK passed directly, not from file
		Metrics:       metricsCollector,
	}

	verifier := restoreverifier.New(
		b2Backend,
		mek,
		*blockSize,
		manifestIndex,
		cfg,
	)

	// Set HTTP listen address
	listenAddr := *httpListen
	if listenAddr == "" {
		listenAddr = ":9002"
	}

	// Setup HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/status", verifier.StatusHandler(metricsCollector))
	mux.HandleFunc("/bucket", verifier.BucketStatusHandler(metricsCollector))
	mux.HandleFunc("/trigger", verifier.TriggerHandler(metricsCollector))
	mux.HandleFunc("/healthz", verifier.HealthHandler(metricsCollector))
	mux.HandleFunc("/readyz", verifier.ReadyHandler(metricsCollector))
	mux.HandleFunc("/metrics", metricsCollector.Handler())

	// Start HTTP server
	server := &http.Server{
		Addr:    listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("HTTP server listening on %s", listenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Start verification loop in background
	verifierCtx, cancelVerifier := context.WithCancel(context.Background())
	go verifier.Start(verifierCtx)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	// Stop verifier
	cancelVerifier()
	verifier.Stop()

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Restore-verifier stopped gracefully")
}

// usage prints usage information.
func usage() {
	fmt.Fprintf(os.Stderr, `restore-verifier - Continuous backup verification for ARMOR

Usage:
  restore-verifier [flags]

Environment Variables:
  ARMOR_B2_REGION              B2 region (e.g., us-west-004)
  ARMOR_B2_ENDPOINT            B2 S3 API endpoint
  ARMOR_B2_ACCESS_KEY_ID       B2 application key ID
  ARMOR_B2_SECRET_ACCESS_KEY   B2 application key
  ARMOR_CF_DOMAIN              Cloudflare domain (optional)
  ARMOR_MEK                    Master encryption key (hex, 64 chars)
  VERIFIER_CHECK_INTERVAL      Verification check interval (default: 6h)
  VERIFIER_SAMPLE_SIZE         Historical sample size (default: 10)
  VERIFIER_HTTP_LISTEN         HTTP listen address (default: :9002)

Flags:
`)
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Bucket Configuration (-bucket flag):
  Format: name[,prefix][,artifact_type][,enabled][,sample_size]
  Examples:
    -bucket mybucket
    -bucket mybucket,backups/
    -bucket mybucket,backups/,sqlite
    -bucket mybucket,backups/,sqlite,true
    -bucket mybucket,,sqlite,true,20

Artifact Types:
  sqlite   - SQLite database (PRAGMA integrity_check)
  parquet  - Parquet file (footer validation)
  tar-gz   - tar.gz archive (read validation)
  generic  - Generic file (SHA-256 verification only)

HTTP Endpoints:
  GET  /status           - Verification status for all buckets
  GET  /bucket?bucket=X  - Status for specific bucket
  POST /trigger          - Trigger immediate verification run
  GET  /healthz          - Liveness check
  GET  /readyz           - Readiness check
  GET  /metrics          - Prometheus metrics

Examples:
  # Run with environment variables
  export ARMOR_B2_REGION=us-west-004
  export ARMOR_B2_ENDPOINT=s3.us-west-004.backblazeb2.com
  export ARMOR_B2_ACCESS_KEY_ID=your-key-id
  export ARMOR_B2_SECRET_ACCESS_KEY=your-secret
  export ARMOR_MEK=0123456789abcdef...
  restore-verifier -bucket mybucket -check-interval 1h

  # Specify multiple buckets
  restore-verifier \
    -bucket armor-apexalgo,,generic \
    -bucket ord-devimprint,,sqlite \
    -bucket iad-ci,,tar-gz
`)
}

// parseDuration parses a duration string with a default fallback.
func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		log.Printf("Invalid duration %q, using default %v: %v", s, defaultVal, err)
		return defaultVal
	}
	return d
}

// parseInt parses an int string with a default fallback.
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		log.Printf("Invalid int %q, using default %d: %v", s, defaultVal, err)
		return defaultVal
	}
	return i
}

// splitCSV splits a CSV string.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var parts []string
	var current string
	inQuotes := false

	for _, r := range s {
		switch r {
		case ',':
			if inQuotes {
				current += string(r)
			} else {
				parts = append(parts, current)
				current = ""
			}
		case '"':
			inQuotes = !inQuotes
		default:
			current += string(r)
		}
	}
	parts = append(parts, current)
	return parts
}
