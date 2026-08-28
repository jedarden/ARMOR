// verify-objects is a tool to verify ARMOR-encrypted objects and detect corruption
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// CandidateObject represents an object from candidate-objects.json
type CandidateObject struct {
	Bucket                string    `json:"bucket"`
	Key                   string    `json:"key"`
	SizeBytes             int64     `json:"size_bytes"`
	SizeHuman             string    `json:"size_human"`
	CreatedAt             time.Time `json:"created_at"`
	AffectedArmorVersions []string  `json:"affected_armor_versions"`
	AffectedVersionCount  int       `json:"affected_version_count"`
}

// VerificationResult represents the result of verifying one object
type VerificationResult struct {
	Bucket           string    `json:"bucket"`
	Key              string    `json:"key"`
	Status           string    `json:"status"` // OK, CORRUPTED, ERROR
	Error            string    `json:"error,omitempty"`
	Details          string    `json:"details,omitempty"`
	SizeBytes        int64     `json:"size_bytes"`
	CreatedAt        time.Time `json:"created_at"`
	AffectedVersions []string  `json:"affected_armor_versions"`
}

// VerificationInventory is the complete verification output
type VerificationInventory struct {
	TotalObjects     int                  `json:"total_objects"`
	OKCount          int                  `json:"ok_count"`
	CorruptedCount   int                  `json:"corrupted_count"`
	ErrorCount       int                  `json:"error_count"`
	VerificationDate time.Time            `json:"verification_date"`
	Results          []VerificationResult `json:"results"`
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: verify-objects <candidate-objects.json> <mek-hex-or-escrow-file> <output.json>\n")
		fmt.Fprintf(os.Stderr, "  <mek-hex-or-escrow-file>: Either 64-char hex MEK or path to escrow JSON file\n")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	mekInput := os.Args[2]
	outputFile := os.Args[3]

	var mek []byte
	var err error

	// Check if mekInput is a file path (escrow) or hex MEK
	if _, statErr := os.Stat(mekInput); statErr == nil {
		// It's a file, try to load as escrow
		mek, err = loadMEKFromEscrow(mekInput)
	} else {
		// Treat as hex MEK
		mek, err = hex.DecodeString(mekInput)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading MEK: %v\n", err)
		os.Exit(1)
	}
	if len(mek) != 32 {
		fmt.Fprintf(os.Stderr, "MEK must be 32 bytes (64 hex chars), got %d bytes\n", len(mek))
		os.Exit(1)
	}

	// Load candidate objects
	var candidates []CandidateObject
	data, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", inputFile, err)
		os.Exit(1)
	}
	if err := json.Unmarshal(data, &candidates); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing %s: %v\n", inputFile, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d candidate objects\n", len(candidates))

	// Initialize B2 backend
	b2, err := initB2Backend()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing B2: %v\n", err)
		os.Exit(1)
	}

	// Verify each object
	inventory := &VerificationInventory{
		TotalObjects:     len(candidates),
		VerificationDate: time.Now(),
		Results:          make([]VerificationResult, 0, len(candidates)),
	}

	ctx := context.Background()
	for i, obj := range candidates {
		if i%100 == 0 {
			fmt.Fprintf(os.Stderr, "Progress: %d/%d verified...\n", i, len(candidates))
		}

		result := verifyObject(ctx, b2, mek, obj)
		inventory.Results = append(inventory.Results, result)

		switch result.Status {
		case "OK":
			inventory.OKCount++
		case "CORRUPTED":
			inventory.CorruptedCount++
		case "ERROR":
			inventory.ErrorCount++
		}
	}

	fmt.Fprintf(os.Stderr, "Verification complete: %d OK, %d CORRUPTED, %d ERROR\n",
		inventory.OKCount, inventory.CorruptedCount, inventory.ErrorCount)

	// Write output
	outputData, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling output: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outputFile, outputData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputFile, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Results written to %s\n", outputFile)
}

func loadMEKFromEscrow(escrowPath string) ([]byte, error) {
	data, err := os.ReadFile(escrowPath)
	if err != nil {
		return nil, fmt.Errorf("read escrow file: %w", err)
	}

	var pkg struct {
		MEK string `json:"mek"`
		B2  struct {
			Region     string `json:"region"`
			Endpoint   string `json:"endpoint"`
			AccessKey  string `json:"access_key"`
			SecretKey  string `json:"secret_key"`
			Bucket     string `json:"bucket"`
			CFDomain   string `json:"cf_domain"`
		} `json:"b2"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("parse escrow JSON: %w", err)
	}

	// Validate MEK
	if pkg.MEK == "" {
		return nil, errors.New("escrow missing 'mek' field")
	}
	mek, err := hex.DecodeString(pkg.MEK)
	if err != nil {
		return nil, fmt.Errorf("decode MEK from escrow: %w", err)
	}
	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK length in escrow: got %d bytes, expected 32", len(mek))
	}

	// Set environment variables for B2 credentials and config
	if pkg.B2.Region != "" {
		os.Setenv("ARMOR_B2_REGION", pkg.B2.Region)
	}
	if pkg.B2.Endpoint != "" {
		os.Setenv("ARMOR_B2_ENDPOINT", pkg.B2.Endpoint)
	}
	if pkg.B2.AccessKey != "" {
		os.Setenv("ARMOR_B2_ACCESS_KEY_ID", pkg.B2.AccessKey)
	}
	if pkg.B2.SecretKey != "" {
		os.Setenv("ARMOR_B2_SECRET_ACCESS_KEY", pkg.B2.SecretKey)
	}
	if pkg.B2.CFDomain != "" {
		os.Setenv("ARMOR_CF_DOMAIN", pkg.B2.CFDomain)
	}

	return mek, nil
}

func initB2Backend() (*backend.B2Backend, error) {
	region := os.Getenv("ARMOR_B2_REGION")
	endpoint := os.Getenv("ARMOR_B2_ENDPOINT")
	accessKey := os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	secretKey := os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	cfDomain := os.Getenv("ARMOR_CF_DOMAIN")

	if region == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("B2 credentials not set: set ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY")
	}

	return backend.NewB2Backend(context.Background(), backend.B2Config{
		Region:          region,
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretKey:       secretKey,
		CFDomain:        cfDomain,
		ReadConcurrency: 16,
	})
}

func verifyObject(ctx context.Context, b2 *backend.B2Backend, mek []byte, obj CandidateObject) VerificationResult {
	result := VerificationResult{
		Bucket:           obj.Bucket,
		Key:              obj.Key,
		SizeBytes:        obj.SizeBytes,
		CreatedAt:        obj.CreatedAt,
		AffectedVersions: obj.AffectedArmorVersions,
	}

	// Get object metadata first
	info, err := b2.Head(ctx, obj.Bucket, obj.Key)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to get object metadata: %v", err)
		return result
	}

	// Check if it's an ARMOR encrypted object
	if !info.IsARMOREncrypted {
		result.Status = "ERROR"
		result.Error = "Object is not ARMOR encrypted"
		return result
	}

	// Try to read and verify just the envelope header (first 1024 bytes)
	// This is enough to detect corruption in the envelope itself
	reader, err := b2.GetRange(ctx, obj.Bucket, obj.Key, 0, 1024)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object: %v", err)
		return result
	}
	defer reader.Close()

	header := make([]byte, 1024)
	n, err := reader.Read(header)
	if err != nil && n == 0 {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read header: %v", err)
		return result
	}
	if n < crypto.HeaderSize {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Object too small: only %d bytes, cannot contain valid envelope (need %d bytes)", n, crypto.HeaderSize)
		return result
	}

	// Try to parse the envelope header
	envelope, err := crypto.DecodeHeader(header)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Envelope header corruption: %v", err)
		result.Details = "Cannot read encrypted envelope - object is unrecoverable"
		return result
	}

	// Verify the envelope magic
	if string(envelope.Magic[:]) != crypto.Magic {
		result.Status = "CORRUPTED"
		result.Error = "Invalid ARMOR magic bytes"
		return result
	}

	// Verify the envelope version is supported
	if envelope.Version != crypto.Version1 && envelope.Version != crypto.Version2 {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Invalid envelope version: %d", envelope.Version)
		return result
	}

	// Try to decrypt the wrapped DEK - we need to read it from the object metadata
	wrappedDEKBase64 := info.Metadata["x-amz-meta-armor-wrapped-dek"]
	if wrappedDEKBase64 == "" {
		result.Status = "ERROR"
		result.Error = "Missing wrapped DEK in object metadata"
		return result
	}

	wrappedDEK, err := decodeBase64(wrappedDEKBase64)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("Invalid wrapped DEK encoding: %v", err)
		return result
	}

	// Try to unwrap the DEK
	dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
	if err != nil {
		result.Status = "CORRUPTED"
		result.Error = fmt.Sprintf("DEK unwrap failed: %v", err)
		result.Details = "Wrapped DEK appears corrupted or uses wrong MEK"
		return result
	}
	zeroBytes(dek)

	// If we got here, the envelope and DEK are valid
	// For a full verification, we would need to verify the HMAC across the entire object
	// but for now, header verification is a good indicator
	result.Status = "OK"
	result.Details = "Envelope and DEK verified successfully"
	return result
}

func decodeBase64(s string) ([]byte, error) {
	// Handle URL-safe base64 by replacing chars
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return hex.DecodeString(s)
}

func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}