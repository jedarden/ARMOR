// armor-decrypt is a standalone CLI tool for decrypting ARMOR-encrypted objects offline.
// It can decrypt from B2 buckets using only a MEK, or from local files with a wrapped DEK.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

var (
	// MEK input
	mekFlag        string
	mekFileFlag    string
	mekEnvFallback bool

	// Input sources
	inputFlag    string
	b2BucketFlag string
	b2KeyID      string
	wrappedDEKFlag string

	// Output
	outputFlag string

	// Other options
	verboseFlag bool
)

func init() {
	flag.StringVar(&mekFlag, "mek", "", "Master encryption key (hex, 64 chars)")
	flag.StringVar(&mekFileFlag, "mek-file", "", "Read MEK from file (hex, 64 chars)")
	flag.BoolVar(&mekEnvFallback, "mek-env", true, "Fallback to ARMOR_MEK env var if flags not set")
	flag.StringVar(&inputFlag, "input", "", "Input: B2 URL (b2://bucket/key) or local file path")
	flag.StringVar(&b2BucketFlag, "b2-bucket", "", "B2 bucket (alternative to B2 URL)")
	flag.StringVar(&b2KeyID, "key-id", "", "Key ID for multi-key MEK (from x-amz-meta-armor-key-id)")
	flag.StringVar(&wrappedDEKFlag, "wrapped-dek", "", "Wrapped DEK (base64, for local files)")
	flag.StringVar(&outputFlag, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(&verboseFlag, "v", false, "Verbose output")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %v\n", flag.Args())
		flag.Usage()
		os.Exit(2)
	}

	ctx := context.Background()

	// Get MEK
	mek, err := getMEK()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading MEK: %v\n", err)
		os.Exit(1)
	}

	// Determine input source
	src, err := getInputSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Decrypt
	plaintext, err := decrypt(ctx, src, mek, b2KeyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decryption failed: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := writeOutput(plaintext); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}

	if verboseFlag {
		fmt.Fprintln(os.Stderr, "Decryption successful")
	}
}

// usage prints the usage message.
func usage() {
	fmt.Fprintf(os.Stderr, `armor-decrypt - Offline ARMOR decryption tool

Usage:
  armor-decrypt -mek HEX -input SOURCE [options]

MEK Input (one required):
  -mek HEX           Master encryption key as hex string (64 chars)
  -mek-file FILE     Read MEK from file (hex, 64 chars)
  -mek-env           Allow ARMOR_MEK env var fallback (default: true)

Input Source (required):
  -input SOURCE      B2 URL (b2://bucket/key) or local file path
  -b2-bucket BUCKET  B2 bucket (alternative to B2 URL)

Decryption Parameters:
  -key-id ID         Key ID for multi-key MEK (from x-amz-meta-armor-key-id)
  -wrapped-dek B64  Wrapped DEK base64 (required for local files)

Output:
  -output FILE        Write to file instead of stdout

Options:
  -v                  Verbose output (to stderr)

Examples:
  # Decrypt from B2 with MEK from flag
  armor-decrypt -mek 0123456789abcdef... -input b2://my-bucket/path/to/file

  # Decrypt local file (requires wrapped DEK)
  armor-decrypt -mek 0123... -input encrypted.bin \\
    -wrapped-dek WWF... -output plaintext.bin

  # Decrypt to stdout
  armor-decrypt -mek 0123... -input b2://bucket/file | tar -xz

  # Decrypt using specific key ID (multi-key setup)
  armor-decrypt -mek 0123... -input b2://bucket/file -key-id backup-key

`)
}

// getMEK loads the MEK from flag, file, or environment.
func getMEK() ([]byte, error) {
	var mekHex string
	var source string

	// Try flag first
	if mekFlag != "" {
		mekHex = mekFlag
		source = "flag"
	}

	// Try file
	if mekHex == "" && mekFileFlag != "" {
		data, err := os.ReadFile(mekFileFlag)
		if err != nil {
			return nil, fmt.Errorf("read MEK file: %w", err)
		}
		mekHex = strings.TrimSpace(string(data))
		source = "file"
	}

	// Try environment
	if mekHex == "" && mekEnvFallback {
		mekHex = os.Getenv("ARMOR_MEK")
		if mekHex != "" {
			source = "env"
		}
	}

	if mekHex == "" {
		return nil, errors.New("no MEK provided: use -mek, -mek-file, or set ARMOR_MEK env var")
	}

	// Decode hex
	mek, err := hex.DecodeString(mekHex)
	if err != nil {
		return nil, fmt.Errorf("decode MEK hex: %w", err)
	}

	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK length: got %d bytes, expected 32", len(mek))
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Loaded MEK from %s\n", source)
	}

	return mek, nil
}

// inputSource represents where to read encrypted data from.
type inputSource struct {
	Type       string // "local" or "b2"
	Path       string // Local file path or B2 key
	Bucket     string // B2 bucket (for B2 type)
	WrappedDEK []byte // Wrapped DEK (nil for B2, required for local)
}

// getInputSource parses the -input flag and returns an inputSource.
func getInputSource() (*inputSource, error) {
	if inputFlag == "" {
		return nil, errors.New("no input source specified: use -input")
	}

	// Check if it's a B2 URL
	if strings.HasPrefix(inputFlag, "b2://") {
		return parseB2URL(inputFlag)
	}

	// Check if bucket is specified separately
	if b2BucketFlag != "" {
		return &inputSource{
			Type:   "b2",
			Bucket: b2BucketFlag,
			Path:   inputFlag,
		}, nil
	}

	// Local file - requires wrapped DEK
	if wrappedDEKFlag == "" {
		return nil, errors.New("local file input requires -wrapped-dek flag")
	}

	wrappedDEK, err := base64.StdEncoding.DecodeString(wrappedDEKFlag)
	if err != nil {
		return nil, fmt.Errorf("decode wrapped DEK: %w", err)
	}

	return &inputSource{
		Type:       "local",
		Path:       inputFlag,
		WrappedDEK: wrappedDEK,
	}, nil
}

// parseB2URL parses a B2 URL like b2://bucket/path/to/file.
func parseB2URL(url string) (*inputSource, error) {
	// Remove b2:// prefix
	rest := strings.TrimPrefix(url, "b2://")
	if rest == url {
		return nil, errors.New("invalid B2 URL format")
	}

	// Split bucket and path
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid B2 URL format: missing path after bucket")
	}

	bucket := parts[0]
	key := parts[1]

	return &inputSource{
		Type:   "b2",
		Bucket: bucket,
		Path:   key,
	}, nil
}

// decrypt performs the decryption from the input source.
func decrypt(ctx context.Context, src *inputSource, mek []byte, keyID string) ([]byte, error) {
	switch src.Type {
	case "local":
		return decryptLocal(ctx, src, mek)
	case "b2":
		return decryptB2(ctx, src, mek, keyID)
	default:
		return nil, fmt.Errorf("unsupported input type: %s", src.Type)
	}
}

// decryptLocal decrypts from a local file.
func decryptLocal(ctx context.Context, src *inputSource, mek []byte) ([]byte, error) {
	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Reading from local file: %s\n", src.Path)
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, src.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w (wrong MEK or corrupted wrapped DEK)", err)
	}

	if verboseFlag {
		fmt.Fprintln(os.Stderr, "Successfully unwrapped DEK")
	}

	// Open file
	f, err := os.Open(src.Path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Read envelope header
	header, err := crypto.ReadEnvelopeHeader(f)
	if err != nil {
		return nil, fmt.Errorf("read envelope header: %w", err)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Envelope: version=%d, block_size=%d, plaintext_size=%d\n",
			header.Version, header.BlockSize(), header.PlaintextSize)
	}

	// Check if HMAC is sidecar (reserved byte flag)
	useSidecarHMAC := header.Reserved[1] == 0x01

	if verboseFlag && useSidecarHMAC {
		fmt.Fprintln(os.Stderr, "Detected sidecar HMAC (multipart upload)")
	}

	// Calculate sizes
	plaintextSize := int64(header.PlaintextSize)
	blockSize := header.BlockSize()
	blockCount := header.BlockCount()

	// Read encrypted data
	encryptedData := make([]byte, plaintextSize)
	if _, err := io.ReadFull(f, encryptedData); err != nil {
		return nil, fmt.Errorf("read encrypted data: %w", err)
	}

	var hmacTable []byte

	if useSidecarHMAC {
		// Fetch HMAC from sidecar file
		sidecarPath := getSidecarHMACPath(src.Path)
		if verboseFlag {
			fmt.Fprintf(os.Stderr, "Reading sidecar HMAC from: %s\n", sidecarPath)
		}

		hmacTable, err = os.ReadFile(sidecarPath)
		if err != nil {
			return nil, fmt.Errorf("read sidecar HMAC from %s: %w (file may not exist or be corrupted)", sidecarPath, err)
		}
	} else {
		// Read inline HMAC table
		hmacTable = make([]byte, int64(blockCount)*crypto.HMACSize)
		if _, err := io.ReadFull(f, hmacTable); err != nil {
			return nil, fmt.Errorf("read HMAC table: %w", err)
		}
	}

	// Verify sizes
	if len(hmacTable) < int(blockCount)*crypto.HMACSize {
		return nil, fmt.Errorf("HMAC table too short: got %d bytes, need %d",
			len(hmacTable), blockCount*crypto.HMACSize)
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(dek, header.IV[:], blockSize)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	// Decrypt
	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
	}

	// Verify plaintext SHA-256
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("plaintext SHA-256 verification failed: %w", err)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
	}

	return plaintext, nil
}

// decryptB2 decrypts from a B2 bucket.
func decryptB2(ctx context.Context, src *inputSource, mek []byte, keyID string) ([]byte, error) {
	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Reading from B2: %s/%s\n", src.Bucket, src.Path)
	}

	// Initialize B2 backend from environment
	b2Backend, err := initB2Backend()
	if err != nil {
		return nil, err
	}

	// Head object to get metadata
	info, err := b2Backend.Head(ctx, src.Bucket, src.Path)
	if err != nil {
		return nil, fmt.Errorf("head B2 object: %w", err)
	}

	// Check if it's ARMOR-encrypted
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		return nil, errors.New("object is not ARMOR-encrypted (missing x-amz-meta-armor-version)")
	}

	// Check key ID if specified
	if keyID != "" && armorMeta.KeyID != keyID {
		return nil, fmt.Errorf("key ID mismatch: expected %s, got %s", keyID, armorMeta.KeyID)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "ARMOR version: %d, Block size: %d, Plaintext size: %d\n",
			armorMeta.Version, armorMeta.BlockSize, armorMeta.PlaintextSize)
		if armorMeta.KeyID != "" {
			fmt.Fprintf(os.Stderr, "Using key ID: %s\n", armorMeta.KeyID)
		}
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w (wrong MEK or corrupted wrapped DEK)", err)
	}

	if verboseFlag {
		fmt.Fprintln(os.Stderr, "Successfully unwrapped DEK")
	}

	// Calculate sizes
	plaintextSize := armorMeta.PlaintextSize
	blockSize := armorMeta.BlockSize
	blockCount := crypto.ComputeBlockCount(plaintextSize, blockSize)

	// Read envelope header (64 bytes) from B2
	headerReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, 0, crypto.HeaderSize)
	if err != nil {
		return nil, fmt.Errorf("read envelope header from B2: %w", err)
	}
	defer headerReader.Close()

	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
		return nil, fmt.Errorf("read header bytes: %w", err)
	}

	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("decode envelope header: %w", err)
	}

	// Read encrypted data
	encryptedData := make([]byte, plaintextSize)
	encryptedReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, crypto.HeaderSize, plaintextSize)
	if err != nil {
		return nil, fmt.Errorf("read encrypted data from B2: %w", err)
	}
	defer encryptedReader.Close()

	if _, err := io.ReadFull(encryptedReader, encryptedData); err != nil {
		return nil, fmt.Errorf("read encrypted bytes: %w", err)
	}

	// Check if HMAC is sidecar (reserved byte flag)
	useSidecarHMAC := header.Reserved[1] == 0x01

	var hmacTable []byte
	var hmacSize = int64(blockCount) * crypto.HMACSize

	if useSidecarHMAC {
		// Fetch HMAC from sidecar object
		sidecarKey := getSidecarHMACKey(src.Path)
		if verboseFlag {
			fmt.Fprintf(os.Stderr, "Fetching sidecar HMAC from: %s\n", sidecarKey)
		}

		hmacReader, _, err := b2Backend.GetDirect(ctx, src.Bucket, sidecarKey)
		if err != nil {
			return nil, fmt.Errorf("fetch sidecar HMAC from %s: %w", sidecarKey, err)
		}
		defer hmacReader.Close()

		hmacTable = make([]byte, hmacSize)
		if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
			return nil, fmt.Errorf("read sidecar HMAC: %w", err)
		}
	} else {
		// Read inline HMAC table
		hmacOffset := crypto.HeaderSize + plaintextSize

		hmacReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, hmacOffset, hmacSize)
		if err != nil {
			return nil, fmt.Errorf("read HMAC table from B2: %w", err)
		}
		defer hmacReader.Close()

		hmacTable = make([]byte, hmacSize)
		if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
			return nil, fmt.Errorf("read HMAC bytes: %w", err)
		}
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Read %d encrypted bytes and %d HMAC entries\n", len(encryptedData), blockCount)
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(dek, header.IV[:], blockSize)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	// Decrypt
	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
	}

	// Verify plaintext SHA-256
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("plaintext SHA-256 verification failed: %w", err)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
	}

	return plaintext, nil
}

// initB2Backend initializes a B2 backend from environment variables.
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
		Region:      region,
		Endpoint:    endpoint,
		AccessKeyID: accessKey,
		SecretKey:   secretKey,
		CFDomain:    cfDomain,
	})
}

// writeOutput writes data to file or stdout.
func writeOutput(data []byte) error {
	if outputFlag == "" {
		// Write to stdout
		_, err := os.Stdout.Write(data)
		return err
	}

	// Write to file
	if err := os.WriteFile(outputFlag, data, 0644); err != nil {
		return fmt.Errorf("write output file: %w", err)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Wrote %d bytes to: %s\n", len(data), outputFlag)
	}

	return nil
}

// getSidecarHMACPath returns the local file path to the sidecar HMAC.
// For local file: /path/to/object.armor → /path/to/.armor/hmac/<sha256-of-key>
func getSidecarHMACPath(objectPath string) string {
	dir := filepath.Dir(objectPath)
	base := filepath.Base(objectPath)

	// SHA256 of the key (object path without leading slash)
	key := base
	if dir != "." && dir != "" {
		key = filepath.Join(dir, base)
	}
	// Normalize to forward slashes for hash consistency
	key = strings.ReplaceAll(key, "\\", "/")
	keyHash := fmt.Sprintf("%x", crypto.ComputePlaintextSHA256([]byte(key)))

	return filepath.Join(dir, ".armor", "hmac", keyHash)
}

// getSidecarHMACKey returns the B2 key for the sidecar HMAC object.
// For B2: bucket/path/to/object → bucket/.armor/hmac/<sha256-of-path>
func getSidecarHMACKey(objectKey string) string {
	// SHA256 of the object key
	keyHash := fmt.Sprintf("%x", crypto.ComputePlaintextSHA256([]byte(objectKey)))
	return fmt.Sprintf(".armor/hmac/%s", keyHash)
}
