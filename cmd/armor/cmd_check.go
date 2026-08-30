// cmd_check.go implements the 'armor check' subcommand for deployment verification
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

func init() {
	registerCommand(Command{
		Name:        "check",
		Description: "Verify ARMOR deployment: config, backend connectivity, Cloudflare path, MEK, and credentials",
		Func:        check,
	})
}

// CheckResult represents the result of a single check probe
type CheckResult struct {
	Name    string
	Status  string // "PASS", "WARN", "FAIL"
	Message string
}

// check runs deployment verification checks
func check() {
	// Load configuration (collects all errors at once)
	cfg, err := config.Load()
	if err != nil {
		// Config errors should exit 1
		for _, result := range runConfigProbe(nil) {
			printCheckResult(result)
		}
		fmt.Fprintf(os.Stderr, "\nConfig errors: %v\n", err)
		os.Exit(1)
	}

	// Print redacted configuration
	fmt.Fprintf(os.Stderr, "=== ARMOR Configuration (redacted) ===\n")
	printRedactedConfig(cfg.Redacted())
	fmt.Fprintf(os.Stderr, "\n")

	// Run all probes
	results := runAllProbes(cfg)

	// Print results
	anyFail := false
	anyWarn := false
	for _, result := range results {
		printCheckResult(result)
		if result.Status == "FAIL" {
			anyFail = true
		} else if result.Status == "WARN" {
			anyWarn = true
		}
	}

	// Exit with appropriate code
	if anyFail {
		fmt.Fprintf(os.Stderr, "\nCheck FAILED: one or more probes failed\n")
		os.Exit(2)
	} else if anyWarn {
		fmt.Fprintf(os.Stderr, "\nCheck PASSED with warnings\n")
		os.Exit(0)
	} else {
		fmt.Fprintf(os.Stderr, "\nCheck PASSED: all probes OK\n")
		os.Exit(0)
	}
}

// printCheckResult prints a single check result
func printCheckResult(result CheckResult) {
	fmt.Printf("[%s] %s: %s\n", result.Status, result.Name, result.Message)
}

// printRedactedConfig prints the redacted configuration
func printRedactedConfig(rc *config.RedactedConfig) {
	fmt.Fprintf(os.Stderr, "Backend: %s\n", rc.Backend)
	if rc.Backend == "b2" {
		fmt.Fprintf(os.Stderr, "  B2 Region: %s\n", rc.B2Region)
		fmt.Fprintf(os.Stderr, "  B2 Endpoint: %s\n", rc.B2Endpoint)
		fmt.Fprintf(os.Stderr, "  B2 Access Key: %s\n", rc.B2AccessKeyID)
		fmt.Fprintf(os.Stderr, "  B2 Secret Key: %s\n", rc.B2SecretAccessKey)
	} else if rc.Backend == "filesystem" {
		fmt.Fprintf(os.Stderr, "  Filesystem Path: %s\n", rc.FSPath)
	}
	fmt.Fprintf(os.Stderr, "Bucket: %s\n", rc.Bucket)
	fmt.Fprintf(os.Stderr, "Prefix: %s\n", rc.Prefix)
	fmt.Fprintf(os.Stderr, "Cloudflare Domain: %s\n", rc.CFDomain)
	fmt.Fprintf(os.Stderr, "MEK: %s\n", rc.MEK)
	fmt.Fprintf(os.Stderr, "Block Size: %d\n", rc.BlockSize)
	fmt.Fprintf(os.Stderr, "Compress: %v\n", rc.Compress)
	fmt.Fprintf(os.Stderr, "Read Concurrency: %d\n", rc.ReadConcurrency)
	fmt.Fprintf(os.Stderr, "Credentials: %d configured\n", len(rc.Credentials))
	fmt.Fprintf(os.Stderr, "Auth File Path: %s\n", rc.AuthFilePath)
}

// runAllProbes runs all verification probes
func runAllProbes(cfg *config.Config) []CheckResult {
	var results []CheckResult

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize backend
	b2, err := initBackendForCheck(cfg)
	if err != nil {
		// If backend init fails, we can't run connectivity probes
		results = append(results, CheckResult{
			Name:    "backend",
			Status:  "FAIL",
			Message: fmt.Sprintf("backend initialization failed: %v", err),
		})
		// Still run config-only probes
		results = append(results, runConfigProbe(cfg)...)
		return results
	}

	// Run config probe
	results = append(results, runConfigProbe(cfg)...)

	// Run backend connectivity probe
	results = append(results, runBackendProbe(ctx, b2, cfg)...)

	// Run Cloudflare path probe (if configured)
	results = append(results, runCloudflareProbe(ctx, b2, cfg)...)

	// Run MEK verification probe
	results = append(results, runMEKProbe(ctx, b2, cfg)...)

	// Run fingerprint retire gate probe
	results = append(results, runFingerprintProbe(ctx, b2, cfg)...)

	return results
}

// runConfigProbe runs config validation probe
func runConfigProbe(cfg *config.Config) []CheckResult {
	if cfg == nil {
		return []CheckResult{{
			Name:    "config",
			Status:  "FAIL",
			Message: "config is nil",
		}}
	}

	var messages []string

	// Check required fields
	if cfg.Bucket == "" {
		messages = append(messages, "bucket not set")
	}
	if len(cfg.MEK) == 0 {
		messages = append(messages, "MEK not set")
	}
	if cfg.Backend == "b2" {
		if cfg.B2Region == "" {
			messages = append(messages, "B2 region not set")
		}
		if cfg.B2Endpoint == "" {
			messages = append(messages, "B2 endpoint not set")
		}
		if cfg.B2AccessKeyID == "" {
			messages = append(messages, "B2 access key not set")
		}
		if cfg.B2SecretAccessKey == "" {
			messages = append(messages, "B2 secret key not set")
		}
	}

	// Check credentials
	if len(cfg.Credentials) == 0 && !cfg.AllowNoCredentials {
		messages = append(messages, "no client credentials configured")
	}

	// Check auth file
	if cfg.AuthFilePath != "" {
		messages = append(messages, fmt.Sprintf("auth file loaded: %s", cfg.AuthFilePath))
	}

	if len(messages) == 0 {
		return []CheckResult{{
			Name:    "config",
			Status:  "PASS",
			Message: fmt.Sprintf("%d credentials configured", len(cfg.Credentials)),
		}}
	}

	return []CheckResult{{
		Name:    "config",
		Status:  "FAIL",
		Message: strings.Join(messages, "; "),
	}}
}

// runBackendProbe runs backend connectivity probe
func runBackendProbe(ctx context.Context, b2 backend.Backend, cfg *config.Config) []CheckResult {
	// HeadBucket to verify connectivity
	err := b2.HeadBucket(ctx, cfg.Bucket)
	if err != nil {
		return []CheckResult{{
			Name:    "backend",
			Status:  "FAIL",
			Message: fmt.Sprintf("HeadBucket failed: %v", err),
		}}
	}

	return []CheckResult{{
		Name:    "backend",
		Status:  "PASS",
		Message: fmt.Sprintf("bucket %s accessible", cfg.Bucket),
	}}
}

// runCloudflareProbe runs Cloudflare path probe
func runCloudflareProbe(ctx context.Context, b2 backend.Backend, cfg *config.Config) []CheckResult {
	// Skip if CF domain not set
	if cfg.CFDomain == "" {
		return []CheckResult{{
			Name:    "cloudflare",
			Status:  "WARN",
			Message: "ARMOR_CF_DOMAIN not set - Cloudflare path not verified",
		}}
	}

	// Find the newest canary object
	canaryKey, err := findNewestCanary(ctx, b2, cfg.Bucket, cfg.Prefix)
	if err != nil {
		return []CheckResult{{
			Name:    "cloudflare",
			Status:  "WARN",
			Message: fmt.Sprintf("no canary object found for verification: %v", err),
		}}
	}

	// Perform ranged GET through Cloudflare path
	start := time.Now()
	body, headers, err := b2.GetRangeWithHeaders(ctx, cfg.Bucket, canaryKey, 0, 1024)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return []CheckResult{{
			Name:    "cloudflare",
			Status:  "FAIL",
			Message: fmt.Sprintf("ranged GET failed: %v", err),
		}}
	}
	defer body.Close()

	// Read and verify we got data
	data, err := io.ReadAll(body)
	if err != nil {
		return []CheckResult{{
			Name:    "cloudflare",
			Status:  "FAIL",
			Message: fmt.Sprintf("read failed: %v", err),
		}}
	}

	if len(data) == 0 {
		return []CheckResult{{
			Name:    "cloudflare",
			Status:  "FAIL",
			Message: "received empty response",
		}}
	}

	// Check Cloudflare cache status
	cacheStatus := "UNKNOWN"
	if cfStatus, ok := headers["CF-Cache-Status"]; ok {
		cacheStatus = cfStatus
	}

	return []CheckResult{{
		Name:    "cloudflare",
		Status:  "PASS",
		Message: fmt.Sprintf("ranged GET OK (%d bytes, CF-Cache-Status: %s, %dms)", len(data), cacheStatus, latency),
	}}
}

// runMEKProbe runs MEK verification probe
func runMEKProbe(ctx context.Context, b2 backend.Backend, cfg *config.Config) []CheckResult {
	// Find the newest canary object
	canaryKey, err := findNewestCanary(ctx, b2, cfg.Bucket, cfg.Prefix)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "WARN",
			Message: fmt.Sprintf("no canary object found for MEK verification: %v", err),
		}}
	}

	// Get canary object metadata
	info, err := b2.Head(ctx, cfg.Bucket, canaryKey)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("Head failed: %v", err),
		}}
	}

	// Check if it's an ARMOR encrypted object
	if !info.IsARMOREncrypted {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: "object is not ARMOR encrypted",
		}}
	}

	// Get wrapped DEK from metadata
	wrappedDEKBase64 := info.Metadata["x-amz-meta-armor-wrapped-dek"]
	if wrappedDEKBase64 == "" {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: "missing wrapped DEK in metadata",
		}}
	}

	// Decode wrapped DEK
	wrappedDEK, err := decodeBase64ToBytes(wrappedDEKBase64)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("failed to decode wrapped DEK: %v", err),
		}}
	}

	// Read header to verify envelope structure
	body, err := b2.GetRange(ctx, cfg.Bucket, canaryKey, 0, 1024)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("failed to read object: %v", err),
		}}
	}
	defer body.Close()

	header := make([]byte, 1024)
	n, err := io.ReadFull(body, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("failed to read header: %v", err),
		}}
	}

	if n < crypto.HeaderSize {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("object too small: only %d bytes, need at least %d", n, crypto.HeaderSize),
		}}
	}

	// Parse envelope header
	envelope, err := crypto.DecodeHeader(header)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("failed to decode envelope header: %v", err),
		}}
	}

	// Verify envelope magic
	if string(envelope.Magic[:]) != crypto.Magic {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: "invalid ARMOR magic bytes",
		}}
	}

	// Try to unwrap DEK with MEK
	dek, err := crypto.UnwrapDEK(cfg.MEK, wrappedDEK)
	if err != nil {
		return []CheckResult{{
			Name:    "mek",
			Status:  "FAIL",
			Message: fmt.Sprintf("MEK verification failed: %v (wrong MEK or corrupted wrapped DEK)", err),
		}}
	}
	// Zero DEK immediately after use
	for i := range dek {
		dek[i] = 0
	}

	return []CheckResult{{
		Name:    "mek",
		Status:  "PASS",
		Message: "MEK successfully decrypted wrapped DEK",
	}}
}

// findNewestCanary finds the newest .armor/canary object in the bucket.
// b2.List returns one page per call (Objects/IsTruncated/NextToken), not a
// streaming iterator, so this pages through the whole prefix rather than
// just the first 100 objects.
func findNewestCanary(ctx context.Context, b2 backend.Backend, bucket, prefix string) (string, error) {
	// List objects with .armor/canary prefix
	canaryPrefix := prefix + ".armor/canary/"

	var newestKey string
	var newestTime time.Time
	continuationToken := ""

	for {
		result, err := b2.List(ctx, bucket, canaryPrefix, "", continuationToken, 100)
		if err != nil {
			return "", fmt.Errorf("list failed: %w", err)
		}

		for _, obj := range result.Objects {
			if obj.LastModified.After(newestTime) {
				newestTime = obj.LastModified
				newestKey = obj.Key
			}
		}

		if !result.IsTruncated {
			break
		}
		continuationToken = result.NextToken
	}

	if newestKey == "" {
		return "", errors.New("no canary objects found")
	}

	return newestKey, nil
}

// runFingerprintProbe runs the fingerprint retire gate probe.
// It collects active and ring fingerprints, scans objects to build a histogram,
// and fails if any object's fingerprint is missing from available keys.
func runFingerprintProbe(ctx context.Context, b2 backend.Backend, cfg *config.Config) []CheckResult {
	// Collect all available fingerprints
	availableFingerprints := collectAvailableFingerprints(cfg)

	// Print active and ring fingerprints per key-id
	var fingerprintMessages []string
	fingerprintMessages = append(fingerprintMessages, fmt.Sprintf("default key: active fingerprint=%s", crypto.MEKFingerprint(cfg.MEK)))

	// Add ring fingerprints for default key
	if ringMEKs, ok := cfg.KeyRings["default"]; ok {
		var ringFPs []string
		for i := 0; i < len(ringMEKs); i += 32 {
			mek := ringMEKs[i : i+32]
			fp := crypto.MEKFingerprint(mek)
			ringFPs = append(ringFPs, fp)
		}
		if len(ringFPs) > 0 {
			fingerprintMessages = append(fingerprintMessages, fmt.Sprintf("default key: ring fingerprints=[%s]", strings.Join(ringFPs, ", ")))
		}
	}

	// Add named key fingerprints
	for name, mek := range cfg.NamedKeys {
		fp := crypto.MEKFingerprint(mek)
		fingerprintMessages = append(fingerprintMessages, fmt.Sprintf("key %s: active fingerprint=%s", name, fp))
		if ringMEKs, ok := cfg.KeyRings[name]; ok {
			var ringFPs []string
			for i := 0; i < len(ringMEKs); i += 32 {
				mek := ringMEKs[i : i+32]
				fp := crypto.MEKFingerprint(mek)
				ringFPs = append(ringFPs, fp)
			}
			if len(ringFPs) > 0 {
				fingerprintMessages = append(fingerprintMessages, fmt.Sprintf("key %s: ring fingerprints=[%s]", name, strings.Join(ringFPs, ", ")))
			}
		}
	}

	// Scan objects to build histogram
	histogram, missingFingerprints, err := scanObjectFingerprints(ctx, b2, cfg.Bucket, cfg.Prefix, availableFingerprints)
	if err != nil {
		// Can't scan objects - warn but don't fail
		msg := fmt.Sprintf("%s (object scan unavailable)", strings.Join(fingerprintMessages, "; "))
		return []CheckResult{{
			Name:    "fingerprint",
			Status:  "WARN",
			Message: msg,
		}}
	}

	// Check for missing fingerprints
	if len(missingFingerprints) > 0 {
		msg := fmt.Sprintf("%s; MISSING FINGERPRINTS: [%s] - these objects use keys not in ARMOR_MEK_RING and would become unreadable",
			strings.Join(fingerprintMessages, "; "),
			strings.Join(missingFingerprints, ", "))
		return []CheckResult{{
			Name:    "fingerprint",
			Status:  "FAIL",
			Message: msg,
		}}
	}

	// Build histogram message
	var histogramParts []string
	for fp, count := range histogram {
		if fp == "" {
			histogramParts = append(histogramParts, fmt.Sprintf("legacy=%d", count))
		} else {
			histogramParts = append(histogramParts, fmt.Sprintf("%s=%d", fp, count))
		}
	}

	msg := fmt.Sprintf("%s; objects by fingerprint: {%s}", strings.Join(fingerprintMessages, "; "), strings.Join(histogramParts, ", "))
	return []CheckResult{{
		Name:    "fingerprint",
		Status:  "PASS",
		Message: msg,
	}}
}

// collectAvailableFingerprints collects all available fingerprints from active and ring keys.
func collectAvailableFingerprints(cfg *config.Config) map[string]bool {
	available := make(map[string]bool)

	// Add active key fingerprint
	available[crypto.MEKFingerprint(cfg.MEK)] = true

	// Add ring key fingerprints for default key
	if ringMEKs, ok := cfg.KeyRings["default"]; ok {
		for i := 0; i < len(ringMEKs); i += 32 {
			mek := ringMEKs[i : i+32]
			fp := crypto.MEKFingerprint(mek)
			available[fp] = true
		}
	}

	// Add named keys and their rings
	for name, mek := range cfg.NamedKeys {
		fp := crypto.MEKFingerprint(mek)
		available[fp] = true
		if ringMEKs, ok := cfg.KeyRings[name]; ok {
			for i := 0; i < len(ringMEKs); i += 32 {
				mek := ringMEKs[i : i+32]
				fp := crypto.MEKFingerprint(mek)
				available[fp] = true
			}
		}
	}

	return available
}

// scanObjectFingerprints scans ARMOR objects and returns a histogram of objects by fingerprint,
// plus a list of fingerprints that are not available in the active/ring keys.
func scanObjectFingerprints(ctx context.Context, b2 backend.Backend, bucket, prefix string, available map[string]bool) (map[string]int, []string, error) {
	histogram := make(map[string]int)
	var missingFingerprints []string

	// List all objects with .armor prefix (excluding canary)
	armorPrefix := prefix + ".armor/"

	result, err := b2.List(ctx, bucket, armorPrefix, "", "", 1000)
	if err != nil {
		return nil, nil, fmt.Errorf("list failed: %w", err)
	}

	for _, obj := range result.Objects {
		// Skip canary objects
		if strings.Contains(obj.Key, "/canary/") {
			continue
		}

		// Extract fingerprint from wrapped DEK metadata
		fp := extractFingerprintFromMetadata(obj.Metadata)
		histogram[fp]++

		// Track missing fingerprints
		if fp != "" && !available[fp] {
			missingFingerprints = append(missingFingerprints, fp)
		}
	}

	return histogram, missingFingerprints, nil
}

// extractFingerprintFromMetadata extracts the fingerprint from wrapped DEK metadata.
// Returns empty string for legacy format (no fingerprint).
func extractFingerprintFromMetadata(metadata map[string]string) string {
	wrappedDEK := metadata["x-amz-meta-armor-wrapped-dek"]
	if wrappedDEK == "" {
		return ""
	}

	// Check for v2:<fp16>:<base64> format
	if len(wrappedDEK) > 4 && wrappedDEK[:3] == "v2:" {
		parts := strings.SplitN(wrappedDEK, ":", 3)
		if len(parts) == 3 && parts[0] == "v2" {
			return parts[1] // fingerprint
		}
	}

	// Legacy format - no fingerprint
	return ""
}

// initBackendForCheck initializes the backend for checking
func initBackendForCheck(cfg *config.Config) (backend.Backend, error) {
	switch cfg.Backend {
	case "b2":
		return backend.NewB2Backend(context.Background(), backend.B2Config{
			Region:          cfg.B2Region,
			Endpoint:        cfg.B2Endpoint,
			AccessKeyID:     cfg.B2AccessKeyID,
			SecretKey:       cfg.B2SecretAccessKey,
			CFDomain:        cfg.CFDomain,
			ReadConcurrency: cfg.ReadConcurrency,
		})
	case "filesystem":
		return backend.NewFSBackend(backend.FSConfig{BasePath: cfg.FSPath})
	default:
		return nil, fmt.Errorf("unknown backend type: %s", cfg.Backend)
	}
}

// decodeBase64ToBytes decodes a base64 string (with URL-safe handling) to bytes
func decodeBase64ToBytes(s string) ([]byte, error) {
	// Handle URL-safe base64
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")

	// Add padding if needed
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}

	return base64.StdEncoding.DecodeString(s)
}
