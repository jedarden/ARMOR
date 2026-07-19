// armor-decrypt is a standalone CLI tool for decrypting ARMOR-encrypted objects offline.
// It can decrypt from B2 buckets using only a MEK, or from local files with a wrapped DEK.
package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
	inputFlag      string
	b2BucketFlag   string
	b2KeyID        string
	wrappedDEKFlag string

	// Multipart local-file inputs (ADR-003 headerless layout)
	sidecarFlag string // path to a JSON HMAC sidecar file (HMACTableSidecar wire format)
	ivFlag      string // object IV (hex), required for local multipart — no header to read it from

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
	flag.StringVar(&sidecarFlag, "sidecar", "", "Path to a JSON HMAC sidecar file for a local multipart object (ADR-003 headerless layout)")
	flag.StringVar(&ivFlag, "iv", "", "Object IV (hex, 16 bytes) for a local multipart object (required with -sidecar)")
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
  -wrapped-dek B64   Wrapped DEK base64 (required for local files)

Multipart local-file mode (ADR-003 headerless layout):
  -sidecar FILE      Path to the JSON HMAC sidecar file that sits alongside a
                     local multipart object. When set, -input is treated as
                     headerless raw ciphertext (no envelope header) and the HMAC
                     table is read from this JSON file.
  -iv HEX            Object IV (hex, 16 bytes). Required with -sidecar because a
                     multipart object has no envelope header to read the IV from.

B2 objects are dispatched automatically on the x-amz-meta-armor-multipart
metadata marker, so no special flags are needed for multipart B2 objects.

Output:
  -output FILE        Write to file instead of stdout

Options:
  -v                  Verbose output (to stderr)

Examples:
  # Decrypt from B2 with MEK from flag
  armor-decrypt -mek 0123456789abcdef... -input b2://my-bucket/path/to/file

  # Decrypt local file (requires wrapped DEK)
  armor-decrypt -mek 0123... -input encrypted.bin \
    -wrapped-dek WWF... -output plaintext.bin

  # Decrypt to stdout
  armor-decrypt -mek 0123... -input b2://bucket/file | tar -xz

  # Decrypt using specific key ID (multi-key setup)
  armor-decrypt -mek 0123... -input b2://bucket/file -key-id backup-key

  # Decrypt a local multipart object (headerless ciphertext + JSON sidecar)
  armor-decrypt -mek 0123... -input object.bin \
    -wrapped-dek WWF... -iv aabbccdd... -sidecar object.hmac.json \
    -output plaintext.bin

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
//
// It supports both on-disk layouts ARMOR writes (ADR-003):
//
//   - Single-PUT envelope: [64-byte header][encrypted blocks][inline HMAC table].
//   - Multipart: headerless raw ciphertext with the per-block HMAC table in a
//     JSON sidecar file alongside (-sidecar). The IV is supplied via -iv because
//     a multipart object has no header byte stream to read it from.
//
// The layout is selected by the -sidecar flag: its presence means multipart.
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

	// A sidecar JSON signals an ADR-003 multipart object (headerless ciphertext +
	// external HMAC table). Without it, the file is a single-PUT envelope.
	if sidecarFlag != "" {
		return decryptLocalMultipart(src, dek)
	}
	return decryptLocalEnvelope(src, dek)
}

// decryptLocalEnvelope decrypts a single-PUT envelope file: a 64-byte envelope
// header, the encrypted blocks, and the inline HMAC table trailing them. The IV
// and plaintext SHA are read from the header.
func decryptLocalEnvelope(src *inputSource, dek []byte) ([]byte, error) {
	if verboseFlag {
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

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Envelope: version=%d, block_size=%d, plaintext_size=%d\n",
			header.Version, header.BlockSize(), header.PlaintextSize)
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

	// Read inline HMAC table
	hmacTable := make([]byte, int64(blockCount)*crypto.HMACSize)
	if _, err := io.ReadFull(f, hmacTable); err != nil {
		return nil, fmt.Errorf("read HMAC table: %w", err)
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

// decryptLocalMultipart decrypts an ADR-003 multipart object from local files:
// headerless raw ciphertext (-input) plus a JSON HMAC sidecar (-sidecar). The IV
// comes from -iv. CTR mode preserves length, so the ciphertext file size is the
// plaintext size. There is no envelope header, so there is no header plaintext
// SHA to verify (multipart objects store the empty-string placeholder — ADR-003
// gap bf-1v2ehf).
func decryptLocalMultipart(src *inputSource, dek []byte) ([]byte, error) {
	if verboseFlag {
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

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Sidecar: block_size=%d, %d block HMACs\n", sidecar.BlockSize, len(sidecar.BlockHMACs))
	}

	// Create decryptor. Absolute block indices: the full-object Decrypt walks
	// block 0..N, which for a headerless multipart object are the absolute
	// indices the HMACs were keyed on during upload.
	decryptor, err := crypto.NewDecryptor(dek, iv, sidecar.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Decrypted %d bytes across %d blocks (multipart; no header SHA to verify)\n",
			len(plaintext), len(sidecar.BlockHMACs))
	}

	return plaintext, nil
}

// decryptB2 decrypts from a B2 bucket.
//
// It dispatches on the ADR-003 multipart metadata marker, exactly as the
// server's read path (internal/server/handlers) and the restore-verifier's
// direct path (internal/restoreverifier) do. Single-PUT objects carry a 64-byte
// envelope header and an inline HMAC table; multipart-completed objects are
// headerless raw ciphertext with the HMAC table in a JSON sidecar object. A
// reader that assumes every object has the envelope layout fails on every
// multipart object (bf-24sxh7): it decodes a header from raw ciphertext and dies
// on "invalid ARMOR magic".
func decryptB2(ctx context.Context, src *inputSource, mek []byte, keyID string) ([]byte, error) {
	if verboseFlag {
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

	// Dispatch on the ADR-003 multipart marker.
	isMultipart := info.Metadata["x-amz-meta-armor-multipart"] == "true"

	var (
		encryptedData []byte
		hmacTable     []byte
		iv            []byte
		header        *crypto.EnvelopeHeader // single-PUT only; nil for multipart
	)
	if isMultipart {
		if verboseFlag {
			fmt.Fprintln(os.Stderr, "Multipart object: headerless ciphertext + JSON HMAC sidecar")
		}
		encryptedData, hmacTable, iv, err = readB2MultipartCiphertext(ctx, b2Backend, src, armorMeta)
	} else {
		encryptedData, hmacTable, iv, header, err = readB2EnvelopeCiphertext(ctx, b2Backend, src, armorMeta)
	}
	if err != nil {
		return nil, err
	}

	if verboseFlag {
		fmt.Fprintf(os.Stderr, "Read %d encrypted bytes and %d HMAC entries\n", len(encryptedData), len(hmacTable)/crypto.HMACSize)
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(dek, iv, armorMeta.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}

	// Decrypt. For the full object the Decryptor walks block 0..N, which are the
	// absolute block indices both layouts key their HMACs on.
	plaintext, err := decryptor.Decrypt(encryptedData, hmacTable)
	if err != nil {
		return nil, fmt.Errorf("decrypt blocks: %w (possible data corruption)", err)
	}

	// Verify the plaintext digest. Single-PUT objects carry the true whole-object
	// SHA in the envelope header. Multipart objects store the empty-string
	// placeholder digest (ADR-003 gap bf-1v2ehf), so there is no header SHA to
	// verify against — per-block HMAC verification is the integrity guarantee.
	if header != nil {
		if err := header.VerifyPlaintextSHA(plaintext); err != nil {
			return nil, fmt.Errorf("plaintext SHA-256 verification failed: %w", err)
		}
		if verboseFlag {
			fmt.Fprintf(os.Stderr, "Verified plaintext SHA-256: %s\n", header.PlaintextSHA256Hex())
		}
	} else if verboseFlag {
		fmt.Fprintln(os.Stderr, "Multipart object: no envelope header SHA to verify (placeholder digest)")
	}

	return plaintext, nil
}

// readB2EnvelopeCiphertext reads a single-PUT object: a 64-byte envelope header
// (decoded for the IV), the encrypted blocks immediately after it, and the
// inline HMAC table trailing the ciphertext. Returns the decoded header so the
// caller can run header.VerifyPlaintextSHA on the decrypted plaintext.
func readB2EnvelopeCiphertext(ctx context.Context, b2Backend backend.Backend, src *inputSource, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, header *crypto.EnvelopeHeader, err error) {
	// Envelope header (64 bytes) at offset 0.
	headerReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, 0, crypto.HeaderSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read envelope header from B2: %w", err)
	}
	defer headerReader.Close()
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read header bytes: %w", err)
	}
	header, err = crypto.DecodeHeader(headerBuf)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("decode envelope header: %w", err)
	}

	// Encrypted data at offset HeaderSize; CTR mode keeps ciphertext == plaintext size.
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, crypto.HeaderSize, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read encrypted data from B2: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read encrypted bytes: %w", err)
	}

	// Inline HMAC table trailing the ciphertext: one HMACSize entry per block.
	blockCount := crypto.ComputeBlockCount(armorMeta.PlaintextSize, armorMeta.BlockSize)
	hmacSize := int64(blockCount) * crypto.HMACSize
	hmacOffset := crypto.HeaderSize + armorMeta.PlaintextSize
	hmacReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, hmacOffset, hmacSize)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read HMAC table from B2: %w", err)
	}
	defer hmacReader.Close()
	hmacTable = make([]byte, hmacSize)
	if _, err := io.ReadFull(hmacReader, hmacTable); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("read HMAC bytes: %w", err)
	}

	return encryptedData, hmacTable, header.IV[:], header, nil
}

// readB2MultipartCiphertext reads an ADR-003 multipart-completed object: raw
// concatenated part ciphertext at offset 0 (no envelope header; plaintext offset
// N == ciphertext offset N) and the per-block HMAC table loaded from the JSON
// sidecar at .armor/hmac/<sha256(key)>. The IV is carried by object metadata
// (there is no header byte stream to read it from). The sidecar is loaded through
// the same MultipartStateManager the server uses, so the JSON wire format is
// shared exactly; its per-block HMACs are flattened into the contiguous table the
// Decryptor consumes.
func readB2MultipartCiphertext(ctx context.Context, b2Backend backend.Backend, src *inputSource, armorMeta *backend.ARMORMetadata) (encryptedData, hmacTable, iv []byte, err error) {
	if len(armorMeta.IV) == 0 {
		return nil, nil, nil, errors.New("multipart object missing IV metadata (x-amz-meta-armor-iv)")
	}

	// Raw ciphertext at offset 0; CTR mode keeps ciphertext == plaintext size.
	encryptedData = make([]byte, armorMeta.PlaintextSize)
	dataReader, err := b2Backend.GetRange(ctx, src.Bucket, src.Path, 0, armorMeta.PlaintextSize)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read multipart ciphertext from B2: %w", err)
	}
	defer dataReader.Close()
	if _, err := io.ReadFull(dataReader, encryptedData); err != nil {
		return nil, nil, nil, fmt.Errorf("read multipart ciphertext bytes: %w", err)
	}

	// HMAC table from the JSON sidecar, flattened to one HMACSize entry per block.
	sidecar, err := backend.NewMultipartStateManager(b2Backend, src.Bucket).LoadHMACTable(ctx, src.Path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch multipart HMAC sidecar from .armor/hmac/<sha256(key)>: %w", err)
	}
	hmacTable = make([]byte, 0, len(sidecar.BlockHMACs)*crypto.HMACSize)
	for _, h := range sidecar.BlockHMACs {
		hmacTable = append(hmacTable, h...)
	}

	return encryptedData, hmacTable, armorMeta.IV, nil
}

// b2BackendFactory returns the B2 backend decryptB2 reads through. Production
// uses the env-driven real backend; tests override this to inject a fake backend
// that serves fixture objects and HMAC sidecars, exercising the full decryptB2
// dispatch without B2 credentials.
var b2BackendFactory = func(ctx context.Context) (backend.Backend, error) {
	return initB2Backend()
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
