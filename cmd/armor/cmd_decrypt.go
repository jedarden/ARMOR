// cmd_decrypt.go implements the 'armor decrypt' subcommand for offline ARMOR decryption.
// It can decrypt from B2 buckets using only a MEK, or from local files with a wrapped DEK.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

func init() {
	registerCommand(Command{
		Name:        "decrypt",
		Description: "Decrypt ARMOR-encrypted objects offline (MEK + B2 or local files)",
		Func:        decrypt,
	})
}

// Decryption flags
var (
	// MEK input
	mekFlag        string
	mekFileFlag    string
	mekRingFlag    string // comma-separated hex fingerprints for MEK ring
	mekEnvFallback bool
	escrowFile     string // path to escrow JSON file containing MEK + B2 credentials

	// Input sources
	inputFlag      string
	b2BucketFlag   string
	b2KeyID        string
	wrappedDEKFlag string

	// B2 backend configuration (for break-glass recovery)
	b2RegionFlag   string
	b2EndpointFlag string

	// Multipart local-file inputs (ADR-003 headerless layout)
	sidecarFlag string // path to a JSON HMAC sidecar file (HMACTableSidecar wire format)
	ivFlag      string // object IV (hex), required for local multipart — no header to read it from
	versionFlag int    // envelope version (1 or 2), for local multipart files (default: 1)

	// Output
	outputFlag string

	// Other options
	decryptVerboseFlag bool
	readConcurrency    int
)

func init() {
	flag.StringVar(&mekFlag, "mek", "", "Master encryption key (hex, 64 chars)")
	flag.StringVar(&mekFileFlag, "mek-file", "", "Read MEK from file (hex, 64 chars)")
	flag.StringVar(&mekRingFlag, "mek-ring", "", "Comma-separated MEK ring (hex fingerprints, each 16 chars)")
	flag.BoolVar(&mekEnvFallback, "mek-env", true, "Fallback to ARMOR_MEK env var if flags not set")
	flag.StringVar(&escrowFile, "escrow", "", "Path to escrow JSON file containing MEK and B2 credentials (self-contained recovery)")
	flag.StringVar(&inputFlag, "input", "", "Input: B2 URL (b2://bucket/key) or local file path")
	flag.StringVar(&b2BucketFlag, "b2-bucket", "", "B2 bucket (alternative to B2 URL)")
	flag.StringVar(&b2RegionFlag, "b2-region", "", "B2 region (e.g., us-west-004); overrides ARMOR_B2_REGION env var")
	flag.StringVar(&b2EndpointFlag, "b2-endpoint", "", "B2 S3 endpoint (e.g., https://s3.us-west-004.backblazeb2.com); overrides ARMOR_B2_ENDPOINT env var")
	flag.StringVar(&b2KeyID, "key-id", "", "Key ID for multi-key MEK (from x-amz-meta-armor-key-id)")
	flag.StringVar(&wrappedDEKFlag, "wrapped-dek", "", "Wrapped DEK (base64, for local files)")
	flag.StringVar(&sidecarFlag, "sidecar", "", "Path to a JSON HMAC sidecar file for a local multipart object (ADR-003 headerless layout)")
	flag.StringVar(&ivFlag, "iv", "", "Object IV (hex, 16 bytes) for a local multipart object (required with -sidecar)")
	flag.IntVar(&versionFlag, "version", 1, "Envelope version (1 or 2), for local multipart files without envelope header (default: 1)")
	flag.StringVar(&outputFlag, "output", "", "Output file path (default: stdout)")
	flag.BoolVar(&decryptVerboseFlag, "v", false, "Verbose output")
	flag.IntVar(&readConcurrency, "read-concurrency", envInt("ARMOR_READ_CONCURRENCY", 16), "Maximum concurrent ranged reads")
}

func decrypt() {
	// Get MEK and ring keys - try escrow file first, then fall back to other methods
	var mek []byte
	var ringKeys []crypto.RingKeyEntry
	var err error
	if escrowFile != "" {
		mek, ringKeys, err = loadEscrow()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading escrow: %v\n", err)
			os.Exit(1)
		}
	} else {
		mek, err = getMEK()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading MEK: %v\n", err)
			os.Exit(1)
		}
		// Parse ring from flag if provided
		if mekRingFlag != "" {
			ringKeys, err = parseMEKRing(mekRingFlag)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error parsing MEK ring: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Determine input source
	src, err := getInputSource()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Decrypt
	ctx := context.Background()
	plaintext, err := decryptObject(ctx, src, mek, ringKeys, b2KeyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Decryption failed: %v\n", err)
		os.Exit(1)
	}

	// Write output
	if err := writeOutput(plaintext); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output: %v\n", err)
		os.Exit(1)
	}

	if decryptVerboseFlag {
		fmt.Fprintln(os.Stderr, "Decryption successful")
	}
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
		return nil, errors.New("no MEK provided: use -mek, -mek-file, -escrow, or set ARMOR_MEK env var")
	}

	// Decode hex
	mek, err := hex.DecodeString(mekHex)
	if err != nil {
		return nil, fmt.Errorf("decode MEK hex: %w", err)
	}

	if len(mek) != 32 {
		return nil, fmt.Errorf("invalid MEK length: got %d bytes, expected 32", len(mek))
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Loaded MEK from %s\n", source)
	}

	return mek, nil
}

// parseMEKRing parses a comma-separated list of hex fingerprints into ring key entries.
// This is used when -mek-ring is provided without an escrow file. The actual MEK values
// must be retrieved from a key store by fingerprint in a real deployment; for testing,
// this simply parses the fingerprint format and returns entries with nil MEK values that
// will be populated by the lookup function.
func parseMEKRing(ringStr string) ([]crypto.RingKeyEntry, error) {
	if ringStr == "" {
		return nil, nil
	}

	fingerprints := strings.Split(ringStr, ",")
	var ringKeys []crypto.RingKeyEntry

	for _, fp := range fingerprints {
		fp = strings.TrimSpace(fp)
		if fp == "" {
			continue
		}
		if len(fp) != 16 {
			return nil, fmt.Errorf("invalid fingerprint length: got %d chars, expected 16", len(fp))
		}
		// Validate hex format
		if !isHex(fp) {
			return nil, fmt.Errorf("invalid hex fingerprint: %s", fp)
		}

		// Note: The actual MEK values need to be retrieved from a key store.
		// For now, we create ring entries with nil MEK that will be populated
		// by the lookup function during unwrapping.
		ringKeys = append(ringKeys, crypto.RingKeyEntry{
			MEK:         nil, // Will be populated by lookup
			Fingerprint: fp,
		})
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Parsed MEK ring: %d fingerprints\n", len(ringKeys))
	}

	return ringKeys, nil
}

// isHex checks if a string is valid hexadecimal.
func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// escrowPackage represents the complete escrow package for break-glass recovery.
type escrowPackage struct {
	MEK     string `json:"mek"`
	MEKRing []struct {
		MEK         string `json:"mek"`
		Fingerprint string `json:"fingerprint"` // 16 hex chars (first 8 bytes of SHA-256(MEK))
	} `json:"mek_ring"`
	B2 struct {
		Region    string `json:"region"`
		Endpoint  string `json:"endpoint"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		Bucket    string `json:"bucket"`
		CFDomain  string `json:"cf_domain"`
	} `json:"b2"`
}

// loadEscrow reads the escrow JSON file and sets environment variables for B2 credentials.
// Returns the MEK as hex bytes and a slice of ring key entries (MEK + fingerprint).
func loadEscrow() ([]byte, []crypto.RingKeyEntry, error) {
	data, err := os.ReadFile(escrowFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read escrow file: %w", err)
	}

	var pkg escrowPackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, nil, fmt.Errorf("parse escrow JSON: %w", err)
	}

	// Validate MEK
	if pkg.MEK == "" {
		return nil, nil, errors.New("escrow missing 'mek' field")
	}
	mek, err := hex.DecodeString(pkg.MEK)
	if err != nil {
		return nil, nil, fmt.Errorf("decode MEK from escrow: %w", err)
	}
	if len(mek) != 32 {
		return nil, nil, fmt.Errorf("invalid MEK length in escrow: got %d bytes, expected 32", len(mek))
	}

	// Build ring keys from escrow mek_ring array
	var ringKeys []crypto.RingKeyEntry
	for _, rk := range pkg.MEKRing {
		if rk.MEK == "" || rk.Fingerprint == "" {
			continue
		}
		ringMEK, err := hex.DecodeString(rk.MEK)
		if err != nil {
			return nil, nil, fmt.Errorf("decode MEK ring key: %w", err)
		}
		if len(ringMEK) != 32 {
			return nil, nil, fmt.Errorf("invalid MEK ring key length: got %d bytes, expected 32", len(ringMEK))
		}
		if len(rk.Fingerprint) != 16 {
			return nil, nil, fmt.Errorf("invalid MEK ring fingerprint length: got %d chars, expected 16", len(rk.Fingerprint))
		}
		ringKeys = append(ringKeys, crypto.RingKeyEntry{
			MEK:         ringMEK,
			Fingerprint: rk.Fingerprint,
		})
	}

	// Set environment variables for B2 credentials and config
	// These will be picked up by initB2Backend()
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
	if pkg.B2.Bucket != "" {
		os.Setenv("ARMOR_BUCKET", pkg.B2.Bucket)
	}
	if pkg.B2.CFDomain != "" {
		os.Setenv("ARMOR_CF_DOMAIN", pkg.B2.CFDomain)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Loaded escrow from %s (MEK + %d ring keys + B2 config)\n", escrowFile, len(ringKeys))
	}

	return mek, ringKeys, nil
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

// decryptObject performs the decryption from the input source.
func decryptObject(ctx context.Context, src *inputSource, mek []byte, ringKeys []crypto.RingKeyEntry, keyID string) ([]byte, error) {
	switch src.Type {
	case "local":
		return decryptLocal(ctx, src, mek, ringKeys)
	case "b2":
		return decryptB2(ctx, src, mek, ringKeys, keyID)
	default:
		return nil, fmt.Errorf("unsupported input type: %s", src.Type)
	}
}

// decryptLocal decrypts from a local file.
//
// It supports both on-disk layouts ARMOR writes (ADR-003):
//
//   - Single-PUT envelope: [64-byte header][encrypted blocks][inline HMAC table].
//   - Multipart: headerless raw ciphertext with the per-block HMAC table in a
//     JSON sidecar file alongside (-sidecar). The IV is supplied via -iv because
//     a multipart object has no header byte stream to read it from.
//
// The layout is selected by the -sidecar flag: its presence means multipart.
func decryptLocal(ctx context.Context, src *inputSource, mek []byte, ringKeys []crypto.RingKeyEntry) ([]byte, error) {
	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Reading from local file: %s\n", src.Path)
	}

	// Unwrap DEK using fingerprint-based selection or legacy trial unwrapping
	// Build lookup function for ring keys
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// For local files, we only support the default key (keyID is empty)
		if keyID != "" {
			return nil, false
		}

		// Check ring keys by fingerprint
		for _, rk := range ringKeys {
			if rk.Fingerprint == fingerprint && rk.MEK != nil {
				return rk.MEK, true
			}
		}
		return nil, false
	}

	// Legacy fallback: try active MEK first, then ring keys
	legacyFallback := func(wrappedBytes []byte) ([]byte, error) {
		// Try active MEK first
		dek, err := crypto.UnwrapDEK(mek, wrappedBytes)
		if err == nil {
			if decryptVerboseFlag {
				fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with active MEK (legacy format)\n")
			}
			return dek, nil
		}

		// Try ring keys
		for _, rk := range ringKeys {
			if rk.MEK == nil {
				continue
			}
			dek, err := crypto.UnwrapDEK(rk.MEK, wrappedBytes)
			if err == nil {
				if decryptVerboseFlag {
					fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with ring key %s (legacy format)\n", rk.Fingerprint)
				}
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in ring could unwrap DEK: %w", err)
	}

	// For local files, wrappedDEKFlag is already in base64 format
	wrappedDEKStr := base64.StdEncoding.EncodeToString(src.WrappedDEK)
	dek, fingerprint, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w (wrong MEK or corrupted wrapped DEK)", err)
	}

	if decryptVerboseFlag {
		if fingerprint != "" {
			fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with fingerprint %s\n", fingerprint)
		} else {
			fmt.Fprintln(os.Stderr, "Successfully unwrapped DEK")
		}
	}

	// A sidecar JSON signals an ADR-003 multipart object (headerless ciphertext +
	// external HMAC table). Without it, the file is a single-PUT envelope.
	if sidecarFlag != "" {
		return decryptLocalMultipart(src, dek)
	}
	return decryptLocalEnvelope(src, dek)
}

// decryptLocalEnvelope decrypts a single-PUT envelope file.
//
// For v1/v2: [64-byte header][encrypted blocks][inline HMAC table]
// For v3:   [64-byte header][encrypted blocks][block table trailer]
//
// The IV and plaintext SHA are read from the header. The block table trailer
// contains per-block HMACs and ciphertext lengths with compression flags.
func decryptLocalEnvelope(src *inputSource, dek []byte) ([]byte, error) {
	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Reading single-PUT envelope: %s\n", src.Path)
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

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Envelope: version=%d, block_size=%d, plaintext_size=%d\n",
			header.Version, header.BlockSize(), header.PlaintextSize)
	}

	// Calculate sizes
	blockSize := header.BlockSize()
	blockCount := header.BlockCount()

	// For v3, read block table trailer and use DecryptV3
	if header.Version == crypto.Version3 {
		return decryptLocalV3Envelope(f, src, dek, header, blockSize, blockCount)
	}

	// v1/v2: Read inline HMAC table
	plaintextSize := int64(header.PlaintextSize)
	encryptedData := make([]byte, plaintextSize)
	if _, err := io.ReadFull(f, encryptedData); err != nil {
		return nil, fmt.Errorf("read encrypted data: %w", err)
	}

	hmacTable := make([]byte, int64(blockCount)*crypto.HMACSize)
	if _, err := io.ReadFull(f, hmacTable); err != nil {
		return nil, fmt.Errorf("read HMAC table: %w", err)
	}

	// Create decryptor with version from envelope header
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], blockSize, header.Version)
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

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
	}

	return plaintext, nil
}

// decryptLocalV3Envelope decrypts a v3 single-PUT envelope with block table trailer.
// Reads the encrypted data and block table trailer, then decrypts with per-(part, block)
// counters and optional decompression.
func decryptLocalV3Envelope(f *os.File, src *inputSource, dek []byte, header *crypto.EnvelopeHeader, blockSize int, blockCount uint32) ([]byte, error) {
	if decryptVerboseFlag {
		fmt.Fprintln(os.Stderr, "Decrypting v3 envelope with block table trailer")
	}

	// Get file size to calculate trailer offset
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileSize := fileInfo.Size()

	// Calculate block table trailer offset and size
	// v3 trailer is at the end: 36 bytes per block (32-byte HMAC + 4-byte clen)
	trailerSize := int64(blockCount) * crypto.BlockTableEntrySize
	trailerOffset := fileSize - trailerSize

	if trailerOffset < int64(crypto.HeaderSize) {
		return nil, fmt.Errorf("invalid trailer offset: %d < header size %d", trailerOffset, crypto.HeaderSize)
	}

	// Read block table trailer
	trailer := make([]byte, trailerSize)
	if _, err := f.Seek(trailerOffset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to trailer: %w", err)
	}
	if _, err := io.ReadFull(f, trailer); err != nil {
		return nil, fmt.Errorf("read block table trailer: %w", err)
	}

	// Decode block table
	blockTable, err := crypto.DecodeBlockTable(trailer, blockSize, blockCount)
	if err != nil {
		return nil, fmt.Errorf("decode block table: %w", err)
	}

	// Calculate encrypted data size from block table
	ciphertextSize := blockTable.TotalCiphertextLength()

	// Read encrypted data (comes right after header)
	if _, err := f.Seek(int64(crypto.HeaderSize), io.SeekStart); err != nil {
		return nil, fmt.Errorf("seek to encrypted data: %w", err)
	}
	encryptedData := make([]byte, ciphertextSize)
	if _, err := io.ReadFull(f, encryptedData); err != nil {
		return nil, fmt.Errorf("read encrypted data: %w", err)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "V3 envelope: %d blocks, ciphertext size %d, trailer %d bytes\n",
			blockCount, ciphertextSize, trailerSize)
	}

	// Create v3 decryptor (part 0 for single-PUT)
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], blockSize, crypto.Version3)
	if err != nil {
		return nil, fmt.Errorf("create v3 decryptor: %w", err)
	}

	// Decrypt with v3 semantics (part 0 for single-PUT, part number for multipart)
	plaintext, err := decryptor.DecryptV3(encryptedData, 0, blockTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt v3 blocks: %w (possible data corruption)", err)
	}

	// Verify plaintext SHA-256
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("plaintext SHA-256 verification failed: %w", err)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
	}

	return plaintext, nil
}

// decryptLocalMultipart decrypts an ADR-003 multipart object from local files:
// headerless raw ciphertext (-input) plus a JSON HMAC sidecar (-sidecar). The IV
// comes from -iv. CTR mode preserves length, so the ciphertext file size is the
// plaintext size. There is no envelope header, so there is no header plaintext
// SHA to verify (multipart objects store the empty-string placeholder — ADR-003
// gap).
func decryptLocalMultipart(src *inputSource, dek []byte) ([]byte, error) {
	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Reading multipart object (headerless + sidecar): %s\n", src.Path)
	}

	// A multipart object has no envelope header, so the IV has nowhere to live in
	// the byte stream — it must be supplied (it is the x-amz-meta-armor-iv value).
	if ivFlag == "" {
		return nil, errors.New("multipart local file requires -iv (the object's IV, hex); a multipart object has no envelope header to read it from")
	}
	iv, err := hex.DecodeString(ivFlag)
	if err != nil {
		return nil, fmt.Errorf("decode -iv hex: %w", err)
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("invalid IV length: got %d bytes, expected 16", len(iv))
	}

	// Read the entire headerless ciphertext (no 64-byte header at offset 0).
	encryptedData, err := os.ReadFile(src.Path)
	if err != nil {
		return nil, fmt.Errorf("read ciphertext file: %w", err)
	}

	// Load and parse the JSON sidecar — the HMACTableSidecar wire format the
	// server writes at .armor/hmac/<sha256(key)> on CompleteMultipartUpload.
	sidecarBytes, err := os.ReadFile(sidecarFlag)
	if err != nil {
		return nil, fmt.Errorf("read sidecar HMAC from %s: %w", sidecarFlag, err)
	}

	// Try to parse as v3 gzip-compressed sidecar first
	var version int
	var blockSize int
	var plaintext []byte

	// Attempt v3 decompression first
	gz, err := gzip.NewReader(bytes.NewReader(sidecarBytes))
	if err == nil {
		// Successfully opened gzip - try v3 format
		defer gz.Close()
		jsonData, err := io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("read gzip sidecar data: %w", err)
		}

		var sidecarV3 backend.HMACTableSidecarV3
		if err := json.Unmarshal(jsonData, &sidecarV3); err != nil {
			return nil, fmt.Errorf("parse v3 sidecar JSON: %w", err)
		}

		if sidecarV3.Version != 3 {
			return nil, fmt.Errorf("invalid v3 sidecar version: got %d, expected 3", sidecarV3.Version)
		}

		if sidecarV3.BlockSize <= 0 {
			return nil, errors.New("v3 sidecar missing block_size")
		}

		version = sidecarV3.Version
		blockSize = sidecarV3.BlockSize

		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "V3 Sidecar: block_size=%d, %d parts, version=%d\n", sidecarV3.BlockSize, len(sidecarV3.Parts), sidecarV3.Version)
		}

		// For v3, we need to build the block table from the sidecar
		// v3 sidecar has per-part block information with HMACs and lengths
		blockTable, err := buildV3BlockTable(&sidecarV3)
		if err != nil {
			return nil, fmt.Errorf("build v3 block table: %w", err)
		}

		// Create decryptor for v3
		decryptor, err := crypto.NewDecryptorWithVersion(dek, iv, blockSize, uint8(version))
		if err != nil {
			return nil, fmt.Errorf("create v3 decryptor: %w", err)
		}

		// Decrypt using v3 semantics (part 0 for single-PUT local files)
		plaintext, err = decryptor.DecryptV3(encryptedData, 0, blockTable)
		if err != nil {
			return nil, fmt.Errorf("decrypt v3 blocks: %w (possible data corruption)", err)
		}

		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "Decrypted %d bytes using v3 with block table (%d blocks)\n",
				len(plaintext), blockTable.EntryCount())
		}

		return plaintext, nil
	}

	// Fallback to v1/v2 sidecar format
	var sidecar backend.HMACTableSidecar
	if err := json.Unmarshal(sidecarBytes, &sidecar); err != nil {
		return nil, fmt.Errorf("parse sidecar JSON: %w (expected HMACTableSidecar wire format)", err)
	}
	if sidecar.BlockSize <= 0 {
		return nil, errors.New("sidecar JSON missing block_size")
	}

	// Flatten the per-block HMACs into the contiguous table the Decryptor wants.
	hmacTable := make([]byte, 0, len(sidecar.BlockHMACs)*crypto.HMACSize)
	for _, h := range sidecar.BlockHMACs {
		hmacTable = append(hmacTable, h...)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Sidecar: block_size=%d, %d block HMACs, version=%d\n", sidecar.BlockSize, len(sidecar.BlockHMACs), sidecar.Version)
	}

	// Determine the envelope version to use. Prefer the version from the sidecar
	// (written by the server on CompleteMultipartUpload), falling back to the
	// CLI flag for old sidecars that don't have the version field.
	version = sidecar.Version
	if version == 0 {
		// Old sidecar without version field - use CLI flag (default: 1)
		version = versionFlag
		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "Sidecar missing version field, using CLI flag: %d\n", versionFlag)
		}
	}

	// Create decryptor. Absolute block indices: the full-object Decrypt walks
	// block 0..N, which for a headerless multipart object are the absolute
	// indices the HMACs were keyed on during upload.
	decryptor, err := crypto.NewDecryptorWithVersion(dek, iv, sidecar.BlockSize, uint8(version))
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	plaintext, err = decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Decrypted %d bytes across %d blocks (multipart; no header SHA to verify)\n",
			len(plaintext), len(sidecar.BlockHMACs))
	}

	return plaintext, nil
}

// buildV3BlockTable constructs a v3 block table from the v3 sidecar format.
// The v3 sidecar contains per-part block information with HMACs and ciphertext lengths.
func buildV3BlockTable(sidecar *backend.HMACTableSidecarV3) (*crypto.BlockTable, error) {
	// Count total blocks across all parts
	totalBlocks := 0
	for _, part := range sidecar.Parts {
		totalBlocks += len(part.Blocks)
	}

	blockTable := crypto.NewBlockTable(sidecar.BlockSize, totalBlocks)

	// Process each part's blocks in order
	globalBlockIndex := 0
	for _, part := range sidecar.Parts {
		for _, blockData := range part.Blocks {
			// blockData is []string{"hmac_base64", "clen"}
			if len(blockData) != 2 {
				return nil, fmt.Errorf("invalid block data format: expected [hmac, clen], got %d elements", len(blockData))
			}

			// Decode HMAC
			hmacBytes, err := base64.StdEncoding.DecodeString(blockData[0])
			if err != nil {
				return nil, fmt.Errorf("decode block HMAC: %w", err)
			}
			if len(hmacBytes) != 32 {
				return nil, fmt.Errorf("invalid HMAC length: got %d bytes, expected 32", len(hmacBytes))
			}

			// Parse ciphertext length
			var hmacArray [32]byte
			copy(hmacArray[:], hmacBytes)

			// Parse clen as uint32
			clen, err := strconv.ParseUint(blockData[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parse ciphertext length: %w", err)
			}

			// Create block table entry
			entry := &crypto.BlockTableEntry{
				HMAC:             hmacArray,
				CiphertextLength: uint32(clen),
			}

			if err := blockTable.AddEntry(entry); err != nil {
				return nil, fmt.Errorf("add block %d: %w", globalBlockIndex, err)
			}

			globalBlockIndex++
		}
	}

	return blockTable, nil
}

// decryptB2 decrypts from a B2 bucket.
//
// It dispatches on the ADR-003 multipart metadata marker, exactly as the
// server's read path (internal/server/handlers) and the restore-verifier's
// direct path (internal/restoreverifier) do. Single-PUT objects carry a 64-byte
// envelope header and an inline HMAC table; multipart-completed objects are
// headerless raw ciphertext with the HMAC table in a JSON sidecar object. A
// reader that assumes every object has the envelope layout fails on every
// multipart object: it decodes a header from raw ciphertext and dies on
// "invalid ARMOR magic".
func decryptB2(ctx context.Context, src *inputSource, mek []byte, ringKeys []crypto.RingKeyEntry, keyID string) ([]byte, error) {
	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Reading from B2: %s/%s\n", src.Bucket, src.Path)
	}

	// Initialize B2 backend (env-driven in production; tests override
	// b2BackendFactory to inject a fake).
	b2Backend, err := b2BackendFactory(ctx)
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

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "ARMOR version: %d, Block size: %d, Plaintext size: %d\n",
			armorMeta.Version, armorMeta.BlockSize, armorMeta.PlaintextSize)
		if armorMeta.KeyID != "" {
			fmt.Fprintf(os.Stderr, "Using key ID: %s\n", armorMeta.KeyID)
		}
	}

	// Unwrap DEK using fingerprint-based selection or legacy trial unwrapping
	// Build lookup function for ring keys
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		// For B2 objects, check keyID matches if specified
		if keyID != "" && keyID != armorMeta.KeyID {
			return nil, false
		}

		// Check ring keys by fingerprint
		for _, rk := range ringKeys {
			if rk.Fingerprint == fingerprint && rk.MEK != nil {
				return rk.MEK, true
			}
		}
		return nil, false
	}

	// Legacy fallback: try active MEK first, then ring keys
	legacyFallback := func(wrappedBytes []byte) ([]byte, error) {
		// Try active MEK first
		dek, err := crypto.UnwrapDEK(mek, wrappedBytes)
		if err == nil {
			if decryptVerboseFlag {
				fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with active MEK (legacy format)\n")
			}
			return dek, nil
		}

		// Try ring keys
		for _, rk := range ringKeys {
			if rk.MEK == nil {
				continue
			}
			dek, err := crypto.UnwrapDEK(rk.MEK, wrappedBytes)
			if err == nil {
				if decryptVerboseFlag {
					fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with ring key %s (legacy format)\n", rk.Fingerprint)
				}
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in ring could unwrap DEK: %w", err)
	}

	// Build wrapped DEK string for UnwrapDEKByFingerprint
	// If the metadata has a fingerprint, use v2: format, otherwise plain base64
	var wrappedDEKStr string
	if armorMeta.MEKFingerprint != "" {
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint,
			base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK))
	} else {
		wrappedDEKStr = base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK)
	}

	dek, fingerprint, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w (wrong MEK or corrupted wrapped DEK)", err)
	}

	if decryptVerboseFlag {
		if fingerprint != "" {
			fmt.Fprintf(os.Stderr, "Successfully unwrapped DEK with fingerprint %s\n", fingerprint)
		} else {
			fmt.Fprintln(os.Stderr, "Successfully unwrapped DEK")
		}
	}

	// Dispatch on the ADR-003 multipart marker.
	isMultipart := info.Metadata["x-amz-meta-armor-multipart"] == "true"

	var (
		encryptedData []byte
		hmacTable     []byte
		iv            []byte
		header        *crypto.EnvelopeHeader // single-PUT only; nil for multipart
		blockTable    *crypto.BlockTable     // v3 only; nil for v1/v2
	)
	if isMultipart {
		if decryptVerboseFlag {
			fmt.Fprintln(os.Stderr, "Multipart object: headerless ciphertext + JSON HMAC sidecar")
		}
		encryptedData, hmacTable, iv, blockTable, err = readB2MultipartCiphertext(ctx, b2Backend, src, armorMeta)
	} else {
		encryptedData, hmacTable, iv, header, blockTable, err = readB2EnvelopeCiphertext(ctx, b2Backend, src, armorMeta)
	}
	if err != nil {
		return nil, err
	}

	// Create decryptor with version from metadata
	decryptor, err := crypto.NewDecryptorWithVersion(dek, iv, armorMeta.BlockSize, uint8(armorMeta.Version))
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	var plaintext []byte

	// Decrypt based on version and format
	if armorMeta.Version == 3 && blockTable != nil {
		// v3 with block table
		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "Decrypting v3 object with block table (%d blocks)\n", blockTable.EntryCount())
		}

		// For v3 single-PUT, use part 0
		part := uint16(0)
		plaintext, err = decryptor.DecryptV3(encryptedData, part, blockTable)
		if err != nil {
			return nil, fmt.Errorf("decrypt v3 blocks: %w (possible data corruption)", err)
		}
	} else {
		// v1/v2 or v3 without block table (legacy)
		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "Read %d encrypted bytes and %d HMAC entries\n", len(encryptedData), len(hmacTable)/crypto.HMACSize)
		}

		// Decrypt. For the full object the Decryptor walks block 0..N, which are the
		// absolute block indices both layouts key their HMACs on.
		plaintext, err = decryptor.Decrypt(encryptedData, hmacTable)
		if err != nil {
			return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
		}
	}

	// Verify the plaintext digest. Single-PUT objects carry the true whole-object
	// SHA in the envelope header. Multipart objects store the empty-string
	// placeholder digest (ADR-003 gap), so there is no header SHA to verify against
	// — per-block HMAC verification is the integrity guarantee.
	if header != nil {
		if err := header.VerifyPlaintextSHA(plaintext); err != nil {
			return nil, fmt.Errorf("plaintext SHA-256 verification failed: %w", err)
		}
		if decryptVerboseFlag {
			fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
		}
	} else if decryptVerboseFlag {
		fmt.Fprintln(os.Stderr, "Multipart object: no envelope header SHA to verify (placeholder digest)")
	}

	return plaintext, nil
}

// readB2EnvelopeCiphertext reads a single-PUT object: a 64-byte envelope header
// (decoded for the IV), the encrypted blocks immediately after it, and either:
// - v1/v2: inline HMAC table trailing the ciphertext
// - v3:   block table trailer at the end (per-block HMACs + compression flags)
// Returns the decoded header so the caller can run header.VerifyPlaintextSHA
// on the decrypted plaintext. For v3, also returns the block table.
func readB2EnvelopeCiphertext(ctx context.Context, b2Backend backend.Backend, src *inputSource, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, header *crypto.EnvelopeHeader, blockTable *crypto.BlockTable, err error) {
	// Envelope header (64 bytes) at offset 0.
	headerReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, 0, crypto.HeaderSize)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read envelope header from B2: %w", err)
	}
	defer headerReader.Close()
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read header bytes: %w", err)
	}
	header, err = crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("decode envelope header: %w", err)
	}

	// For v3, read block table trailer
	if header.Version == crypto.Version3 {
		return readB2V3EnvelopeCiphertext(ctx, b2Backend, src, armorMeta, header)
	}

	// v1/v2: Read encrypted data and inline HMAC table
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, crypto.HeaderSize, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read encrypted data from B2: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read encrypted bytes: %w", err)
	}

	// Inline HMAC table trailing the ciphertext: one HMACSize entry per block.
	blockCount := crypto.ComputeBlockCount(armorMeta.PlaintextSize, armorMeta.BlockSize)
	hmacSize := int64(blockCount) * crypto.HMACSize
	hmacOffset := crypto.HeaderSize + armorMeta.PlaintextSize
	hmacReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, hmacOffset, hmacSize)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read HMAC table from B2: %w", err)
	}
	defer hmacReader.Close()
	hmacTable = make([]byte, hmacSize)
	if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read HMAC bytes: %w", err)
	}

	return encryptedData, hmacTable, header.IV[:], header, nil, nil
}

// readB2V3EnvelopeCiphertext reads a v3 single-PUT object with block table trailer.
// Returns encrypted data, block table, IV, and header (hmacTable is nil for v3).
func readB2V3EnvelopeCiphertext(ctx context.Context, b2Backend backend.Backend, src *inputSource, armorMeta *backend.ARMORMetadata, header *crypto.EnvelopeHeader) (encryptedData, hmacTable, iv []byte, returnedHeader *crypto.EnvelopeHeader, blockTable *crypto.BlockTable, err error) {
	if decryptVerboseFlag {
		fmt.Fprintln(os.Stderr, "Reading v3 envelope with block table trailer from B2")
	}

	blockSize := header.BlockSize()
	blockCount := header.BlockCount()

	// Calculate block table trailer size and offset
	trailerSize := int64(blockCount) * crypto.BlockTableEntrySize

	// Get object size to calculate trailer offset
	// For v3, the object size = header + encrypted data + trailer
	// We can infer encrypted data size from the block table, so we need to read the trailer first

	// Read the trailer from the end
	trailerReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, -trailerSize, trailerSize)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read block table trailer from B2: %w", err)
	}
	defer trailerReader.Close()

	trailer := make([]byte, trailerSize)
	if _, err := io.ReadFull(trailerReader, trailer); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read trailer bytes: %w", err)
	}

	// Decode block table
	blockTable, err = crypto.DecodeBlockTable(trailer, blockSize, blockCount)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("decode block table: %w", err)
	}

	// Calculate encrypted data size from block table
	ciphertextSize := blockTable.TotalCiphertextLength()

	// Read encrypted data (comes right after header)
	encryptedData = make([]byte, ciphertextSize)
	dataReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, int64(crypto.HeaderSize), int64(ciphertextSize))
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read encrypted data from B2: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("read encrypted bytes: %w", err)
	}

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "V3 B2 envelope: %d blocks, ciphertext size %d, trailer %d bytes\n",
			blockCount, ciphertextSize, trailerSize)
	}

	return encryptedData, nil, header.IV[:], header, blockTable, nil
}

// readB2MultipartCiphertext reads an ADR-003 multipart-completed object: raw
// concatenated part ciphertext at offset 0 (no envelope header; plaintext offset
// N == ciphertext offset N) and the per-block HMAC table loaded from the JSON
// sidecar at .armor/hmac/<sha256(key)>. The IV is carried by object metadata
// (there is no header byte stream to read it from). The sidecar is loaded through
// the same MultipartStateManager the server uses, so the JSON wire format is
// shared exactly. For v3 objects, returns the block table instead of flattened HMACs.
func readB2MultipartCiphertext(ctx context.Context, b2Backend backend.Backend, src *inputSource, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, blockTable *crypto.BlockTable, err error) {
	if len(armorMeta.IV) == 0 {
		return nil, nil, nil, nil, errors.New("multipart object missing IV metadata (x-amz-meta-armor-iv)")
	}

	// Raw ciphertext at offset 0; CTR mode keeps ciphertext == plaintext size.
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, 0, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read multipart ciphertext from B2: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read multipart ciphertext bytes: %w", err)
	}

	// For v3, load the v3 sidecar format
	if armorMeta.Version == 3 {
		sidecarV3, err := backend.NewMultipartStateManager(b2Backend, src.Bucket).LoadHMACTableV3(ctx, src.Path)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("fetch v3 HMAC sidecar from .armor/hmac/<sha256(key)>: %w", err)
		}

		blockTable, err = buildV3BlockTableFromSidecar(sidecarV3)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("build v3 block table: %w", err)
		}

		return encryptedData, nil, armorMeta.IV, blockTable, nil
	}

	// For v1/v2, load the regular sidecar format
	sidecar, err := backend.NewMultipartStateManager(b2Backend, src.Bucket).LoadHMACTable(ctx, src.Path)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("fetch multipart HMAC sidecar from .armor/hmac/<sha256(key)>: %w", err)
	}
	hmacTable = make([]byte, 0, len(sidecar.BlockHMACs)*crypto.HMACSize)
	for _, h := range sidecar.BlockHMACs {
		hmacTable = append(hmacTable, h...)
	}

	return encryptedData, hmacTable, armorMeta.IV, nil, nil
}

// buildV3BlockTableFromSidecar constructs a v3 block table from the v3 sidecar format.
func buildV3BlockTableFromSidecar(sidecar *backend.HMACTableSidecarV3) (*crypto.BlockTable, error) {
	// Count total blocks across all parts
	totalBlocks := 0
	for _, part := range sidecar.Parts {
		totalBlocks += len(part.Blocks)
	}

	blockTable := crypto.NewBlockTable(sidecar.BlockSize, totalBlocks)

	// Process each part's blocks in order
	for _, part := range sidecar.Parts {
		for _, blockData := range part.Blocks {
			// blockData is []string{"hmac_base64", "clen"}
			if len(blockData) != 2 {
				return nil, fmt.Errorf("invalid block data format: expected [hmac, clen], got %d elements", len(blockData))
			}

			// Decode HMAC
			hmacBytes, err := base64.StdEncoding.DecodeString(blockData[0])
			if err != nil {
				return nil, fmt.Errorf("decode block HMAC: %w", err)
			}
			if len(hmacBytes) != 32 {
				return nil, fmt.Errorf("invalid HMAC length: got %d bytes, expected 32", len(hmacBytes))
			}

			// Parse ciphertext length
			var hmacArray [32]byte
			copy(hmacArray[:], hmacBytes)

			// Parse clen as uint32
			clen, err := strconv.ParseUint(blockData[1], 10, 32)
			if err != nil {
				return nil, fmt.Errorf("parse ciphertext length: %w", err)
			}

			// Create block table entry
			entry := &crypto.BlockTableEntry{
				HMAC:             hmacArray,
				CiphertextLength: uint32(clen),
			}

			if err := blockTable.AddEntry(entry); err != nil {
				return nil, fmt.Errorf("add block: %w", err)
			}
		}
	}

	return blockTable, nil
}

// b2BackendFactory returns the B2 backend decryptB2 reads through. Production
// uses the env-driven real backend; tests override this to inject a fake backend
// that serves fixture objects and HMAC sidecars, exercising the full decryptB2
// dispatch without B2 credentials.
var b2BackendFactory = func(ctx context.Context) (backend.Backend, error) {
	return initB2Backend()
}

// initB2Backend initializes a B2 backend from flags or environment variables.
// Flags take precedence over environment variables for break-glass recovery.
func initB2Backend() (*backend.B2Backend, error) {
	region := b2RegionFlag
	if region == "" {
		region = os.Getenv("ARMOR_B2_REGION")
	}

	endpoint := b2EndpointFlag
	if endpoint == "" {
		endpoint = os.Getenv("ARMOR_B2_ENDPOINT")
	}

	accessKey := os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	secretKey := os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	cfDomain := os.Getenv("ARMOR_CF_DOMAIN")

	if region == "" || endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, errors.New("B2 credentials not set: set ARMOR_B2_REGION, ARMOR_B2_ENDPOINT, ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY (or use -b2-region, -b2-endpoint flags)")
	}

	return backend.NewB2Backend(context.Background(), backend.B2Config{
		Region:          region,
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretKey:       secretKey,
		CFDomain:        cfDomain,
		ReadConcurrency: readConcurrency,
	})
}

func envInt(name string, defaultValue int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil || value < 1 {
		return defaultValue
	}
	return value
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

	if decryptVerboseFlag {
		fmt.Fprintf(os.Stderr, "Wrote %d bytes to: %s\n", len(data), outputFlag)
	}

	return nil
}
