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
//   VERIFIER_DR_DRILL_INTERVAL   Cadence of the periodic direct-only DR drill (default: disabled)
//
// HTTP:
//   POST /trigger            Trigger an immediate dual-path verification run
//   POST /trigger?mode=dr-drill  Trigger a direct-only DR drill (excludes the ARMOR read path)
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
	"strings"
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

	// DR-drill cadence (ModeDRDrill). Independent of -check-interval so a
	// deployment can run the frequent dual-path verification yet exercise the
	// direct-only "server-is-gone" recovery on its own (typically longer)
	// schedule. Zero disables the periodic drill; the drill is still available
	// on demand via POST /trigger?mode=dr-drill.
	drillInterval = flag.Duration("dr-drill-interval", parseDuration(os.Getenv("VERIFIER_DR_DRILL_INTERVAL"), 0), "Cadence of the periodic direct-only DR drill (0 = disabled; on-demand via /trigger?mode=dr-drill)")

	// Escalation configuration (ADR-004 §5: one bead per distinct failure +
	// one staleness bead per freshness window; storm-proof). Defaults to
	// disabled so the running fleet is unchanged until a deployment can host
	// the bf CLI; enable once `br` is available in the image.
	escalationEnabled = flag.Bool("escalation", parseBool(os.Getenv("VERIFIER_ESCALATION"), false), "File one bead per distinct verification failure + staleness (requires bf/br CLI)")
	escalDeployment   = flag.String("escalation-deployment", os.Getenv("ARMOR_DEPLOYMENT"), "Deployment name recorded in escalation bead bodies")
	escalFreshness    = flag.Duration("escalation-freshness-window", parseDuration(os.Getenv("VERIFIER_FRESHNESS_WINDOW"), 24*time.Hour), "Staleness window: escalate once per window when no verified restore occurs")
	escalStatePath    = flag.String("escalation-state", getenvDefault("VERIFIER_ESCALATION_STATE", "/var/lib/restore-verifier/escalation-state.json"), "Path to the persisted dedupe-state file (mount a volume here for restart-survival)")
	escalWorkspace    = flag.String("escalation-workspace", os.Getenv("VERIFIER_ESCALATION_WORKSPACE"), "bf workspace (-w) for filed beads (empty = cwd .beads/)")
	escalLabel        = flag.String("escalation-label", os.Getenv("VERIFIER_ESCALATION_LABEL"), "Label applied to every escalation bead (optional)")
	escalBinary       = flag.String("escalation-bf-binary", getenvDefault("VERIFIER_ESCALATION_BF_BINARY", "br"), "Path to the bf/br CLI used to file escalation beads")
	escalExecTimeout  = flag.Duration("escalation-exec-timeout", parseDuration(os.Getenv("VERIFIER_ESCALATION_EXEC_TIMEOUT"), 10*time.Second), "Per-call timeout for a single bf create")

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
		Buckets:         bucketFlag,
		Interval:        *checkInterval,
		SampleSize:      *sampleSize,
		EscrowMEKPath:   "", // MEK passed directly, not from file
		Metrics:         metricsCollector,
		DRDrillInterval: *drillInterval,
	}

	// Escalation (ADR-004 §5). Disabled by default — the running fleet has no
	// bf/br CLI in-image, so filing is inert (noop filer) until a deployment
	// opts in. When enabled, the BFCLIFiler shells out to `br create`; the
	// Escalator itself is storm-proof regardless (persisted dedupe set, one bead
	// per distinct failure, one staleness bead per window, no retry loops).
	if *escalationEnabled {
		filer := &restoreverifier.BFCLIFiler{
			Binary:      *escalBinary,
			Workspace:   *escalWorkspace,
			Label:       *escalLabel,
			ExecTimeout: *escalExecTimeout,
		}
		cfg.Escalator = restoreverifier.NewEscalator(restoreverifier.EscalatorConfig{
			Filer:           filer,
			Deployment:      *escalDeployment,
			FreshnessWindow: *escalFreshness,
			StatePath:       *escalStatePath,
		})
		log.Printf("Escalation enabled: deployment=%q freshness=%s state=%s (storm-proof: one bead per distinct failure)",
			orDefault(*escalDeployment, "(unset)"), *escalFreshness, *escalStatePath)
	} else {
		log.Printf("Escalation disabled (VERIFIER_ESCALATION=false); failures surface via metrics only")
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
  VERIFIER_DR_DRILL_INTERVAL   Direct-only DR drill interval (default: disabled)

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
  POST /trigger          - Trigger immediate verification run (dual path)
  POST /trigger?mode=dr-drill - Trigger a direct-only DR drill (excludes ARMOR read path)
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

// parseBool parses a bool string with a default fallback. Accepts the usual
// strconv.ParseBool set (1, t, true, TRUE, etc.); anything malformed falls back
// to the default rather than failing startup.
func parseBool(s string, defaultVal bool) bool {
	if s == "" {
		return defaultVal
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "t", "true", "yes", "on":
		return true
	case "0", "f", "false", "no", "off":
		return false
	}
	log.Printf("Invalid bool %q, using default %v", s, defaultVal)
	return defaultVal
}

// getenvDefault returns the named env var, or defaultVal when unset/empty.
func getenvDefault(name, defaultVal string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return defaultVal
}

// orDefault returns s, or defaultVal when s is empty.
func orDefault(s, defaultVal string) string {
	if s == "" {
		return defaultVal
	}
	return s
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
