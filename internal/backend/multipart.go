package backend

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"strconv"
	"strings"
	"time"
)

// EmptyPlaintextSHA256Hex is the SHA-256 of the empty byte sequence. Before
// bf-1v2ehf, CompleteMultipartUpload wrote it as the plaintext digest for every
// multipart object — a meaningless placeholder that could not be trusted as a
// real checksum. New multipart uploads now store the real combined per-part
// digest, but objects written before the fix still carry this value, so any
// content-vs-stored comparison must treat it (and an empty string) as "no
// digest declared" rather than a value to match.
const EmptyPlaintextSHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// IsPlaceholderPlaintextSHA reports whether a declared plaintext SHA-256 is
// absent or the legacy ADR-003 multipart placeholder, and therefore must not be
// enforced as a real per-object checksum (multipart objects written before
// bf-1v2ehf). Centralized here so every content-vs-stored comparison — the
// restore verifier, the streaming GET path, and any future audit — agrees on
// what "no real digest" means.
func IsPlaceholderPlaintextSHA(s string) bool {
	return s == "" || s == EmptyPlaintextSHA256Hex
}

// MultipartState represents the state of an in-progress multipart upload.
// This is stored in B2 at .armor/multipart/<upload-id>.state for format version 2,
// or at .armor/multipart/<upload-id>/meta.json for format version 3.
type MultipartState struct {
	UploadID       string    `json:"upload_id"`
	Bucket         string    `json:"bucket"`
	Key            string    `json:"key"`
	IV             []byte    `json:"iv"`
	WrappedDEK     []byte    `json:"wrapped_dek"`
	MEKFingerprint string    `json:"mek_fingerprint,omitempty"` // 16-char hex fingerprint of MEK used (v2 format)
	BlockSize      int       `json:"block_size"`
	Created        time.Time `json:"created"`
	ContentType    string    `json:"content_type"`
	KeyID          string    `json:"key_id"` // Key identifier for multi-key support

	// Per-part HMACs (part number -> HMACs for each block in that part)
	// Stored as base64-encoded concatenation of all block HMACs
	PartHMACs map[int]string `json:"part_hmacs"`

	// Per-part encrypted sizes (for range translation on completion)
	PartSizes map[int]int64 `json:"part_sizes"`

	// Per-part plaintext SHA-256 digests (hex), keyed by part number. Each
	// UploadPart records the SHA-256 of the plaintext it received so that
	// CompleteMultipartUpload can assemble a real, reproducible whole-object
	// plaintext digest (CombinePartPlaintextSHAs) instead of the empty-string
	// placeholder (bf-1v2ehf / ADR-003 residual gap). Parts arrive out of order
	// (ADR-005), so the digests must be combined in ascending part-number order
	// at Complete — hence the interaction with the part-ordering contract.
	PartPlaintextSHAs map[int]string `json:"part_plaintext_shas"`

	// PartSize is the uniform part size P pinned from part NUMBER 1 (ADR-005,
	// amended 2026-07-19 — originally "first arriving part", which failed under
	// default aws-cli concurrency where the short final part arrives first). A
	// part's CTR counter offset is a function of its part number alone: part N
	// starts at block (N-1)*P/BlockSize — computable regardless of arrival
	// order. 0 means part 1 has not arrived yet; any part >1 arriving while 0
	// is deferred by the handler with a retryable 503 SlowDown (nothing stored).
	//
	// ADR-011: When non-uniform parts are enabled, this field is used for
	// backward compatibility but offsets are calculated from cumulative part
	// sizes instead.
	PartSize int64 `json:"part_size"`

	// CumulativePartSizes tracks the cumulative byte offset where each part starts.
	// For part N, CumulativePartSizes[N] = sum of sizes of parts 1..(N-1).
	// This is used for ADR-011 non-uniform multipart uploads.
	// A nil map means the upload uses uniform part sizes (legacy mode).
	CumulativePartSizes map[int]int64 `json:"cumulative_part_sizes,omitempty"`

	// NonUniformParts indicates whether this upload uses non-uniform part sizes.
	// When true, parts may vary in size and offsets are calculated from
	// CumulativePartSizes rather than (N-1)*PartSize.
	NonUniformParts bool `json:"non_uniform_parts,omitempty"`

	// Poisoned marks an upload id as permanently failed (ADR-005 rule 4). When
	// the optimistic-P contract is contradicted (a part larger than P, two
	// presumed-final parts, or a same-part retry with a different size), the
	// offending UploadPart is rejected AND the upload id is poisoned so that
	// CompleteMultipartUpload fails with a clear retry-the-upload message,
	// never storing a violating object.
	Poisoned     bool   `json:"poisoned"`
	PoisonReason string `json:"poison_reason"`

	// FormatVersion is the ARMOR manifest format version (2 or 3). This determines
	// how the state is persisted: v2 uses a single .state file, v3 uses per-part
	// objects under a directory. The format version is fixed at CreateMultipartUpload
	// time based on the server's ARMOR_FORMAT_VERSION configuration and does not
	// change during the upload's lifetime.
	FormatVersion int `json:"format_version,omitempty"`
}

// MultipartMetadataV3 represents the metadata stored in .armor/multipart/<id>/meta.json
// for format version 3. This contains only the upload-level cryptographic material and
// configuration, not per-part data (which lives in part-<n>.json files).
type MultipartMetadataV3 struct {
	UploadID       string    `json:"upload_id"`
	Bucket         string    `json:"bucket"`
	Key            string    `json:"key"`
	IV             []byte    `json:"iv"`
	WrappedDEK     []byte    `json:"wrapped_dek"`
	MEKFingerprint string    `json:"mek_fingerprint,omitempty"` // 16-char hex fingerprint of MEK used (v2 format)
	BlockSize      int       `json:"block_size"`
	Created        time.Time `json:"created"`
	ContentType    string    `json:"content_type"`
	KeyID          string    `json:"key_id"`         // Key identifier for multi-key support
	FormatVersion  int       `json:"format_version"` // Always 3 for this structure

	// PartSize is the uniform part size P pinned from part NUMBER 1 (ADR-005).
	// This field is set when part 1 arrives and is used to validate subsequent
	// parts. For v3, this is stored in meta.json so it's available without
	// reading any part files.
	PartSize int64 `json:"part_size,omitempty"`

	// NonUniformParts indicates whether this upload uses non-uniform part sizes.
	// When true, parts may vary in size and offsets are calculated from
	// cumulative part sizes instead of (N-1)*PartSize.
	NonUniformParts bool `json:"non_uniform_parts,omitempty"`

	// Poisoned marks an upload id as permanently failed (ADR-005 rule 4).
	Poisoned     bool   `json:"poisoned"`
	PoisonReason string `json:"poison_reason,omitempty"`
}

// PartDataV3 represents the data stored in .armor/multipart/<id>/part-<n>.json
// for format version 3. Each part is stored as a separate object, allowing
// concurrent UploadPart operations to proceed without touching the same file.
type PartDataV3 struct {
	PartNumber       int    `json:"part_number"`       // Part number (1-based)
	PlaintextLen     int64  `json:"plaintext_len"`     // Length of plaintext part
	CiphertextLen    int64  `json:"ciphertext_len"`    // Length of encrypted part
	BlockHMACsBase64 string `json:"block_hmacs_b64"`   // Base64-encoded concatenated HMACs for each block
	PlaintextSHAHex  string `json:"plaintext_sha_hex"` // SHA-256 of plaintext part (hex)
}

// MultipartStateManager manages multipart upload state persistence.
type MultipartStateManager struct {
	backend Backend
	bucket  string // The ARMOR bucket (where .armor/ prefix lives)
}

// NewMultipartStateManager creates a new MultipartStateManager.
func NewMultipartStateManager(backend Backend, bucket string) *MultipartStateManager {
	return &MultipartStateManager{
		backend: backend,
		bucket:  bucket,
	}
}

// SaveState saves the multipart upload state to B2.
func (m *MultipartStateManager) SaveState(ctx context.Context, state *MultipartState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal multipart state: %w", err)
	}

	key := fmt.Sprintf(".armor/multipart/%s.state", state.UploadID)
	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), nil); err != nil {
		return fmt.Errorf("failed to save multipart state: %w", err)
	}

	return nil
}

// LoadState loads the multipart upload state from B2.
func (m *MultipartStateManager) LoadState(ctx context.Context, uploadID string) (*MultipartState, error) {
	key := fmt.Sprintf(".armor/multipart/%s.state", uploadID)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load multipart state: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read multipart state: %w", err)
	}

	var state MultipartState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal multipart state: %w", err)
	}

	return &state, nil
}

// DeleteState deletes the multipart upload state from B2.
// For v2 uploads, deletes .armor/multipart/<id>.state
// For v3 uploads, deletes the entire .armor/multipart/<id>/ directory
func (m *MultipartStateManager) DeleteState(ctx context.Context, uploadID string) error {
	// Try v3 format first (directory)
	prefix := fmt.Sprintf(".armor/multipart/%s/", uploadID)
	listResult, err := m.backend.List(ctx, m.bucket, prefix, "", "", 1000)
	if err == nil && len(listResult.Objects) > 0 {
		// V3 format: delete all objects in the directory
		var keys []string
		for _, obj := range listResult.Objects {
			keys = append(keys, obj.Key)
		}
		// Also check if there are more objects
		for listResult.IsTruncated {
			listResult, err = m.backend.List(ctx, m.bucket, prefix, "", listResult.NextToken, 1000)
			if err != nil {
				break
			}
			for _, obj := range listResult.Objects {
				keys = append(keys, obj.Key)
			}
		}
		if len(keys) > 0 {
			if err := m.backend.DeleteObjects(ctx, m.bucket, keys); err != nil {
				return fmt.Errorf("failed to delete multipart v3 state directory: %w", err)
			}
		}
		return nil
	}

	// Fall back to v2 format (single .state file)
	key := fmt.Sprintf(".armor/multipart/%s.state", uploadID)
	if err := m.backend.Delete(ctx, m.bucket, key); err != nil {
		return fmt.Errorf("failed to delete multipart state: %w", err)
	}
	return nil
}

// SaveMetadataV3 saves the multipart upload metadata to .armor/multipart/<id>/meta.json
// for format version 3.
func (m *MultipartStateManager) SaveMetadataV3(ctx context.Context, metadata *MultipartMetadataV3) error {
	data, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal multipart metadata: %w", err)
	}

	key := fmt.Sprintf(".armor/multipart/%s/meta.json", metadata.UploadID)
	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), nil); err != nil {
		return fmt.Errorf("failed to save multipart metadata: %w", err)
	}

	return nil
}

// LoadMetadataV3 loads the multipart upload metadata from .armor/multipart/<id>/meta.json
// for format version 3.
func (m *MultipartStateManager) LoadMetadataV3(ctx context.Context, uploadID string) (*MultipartMetadataV3, error) {
	key := fmt.Sprintf(".armor/multipart/%s/meta.json", uploadID)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load multipart metadata: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read multipart metadata: %w", err)
	}

	var metadata MultipartMetadataV3
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal multipart metadata: %w", err)
	}

	return &metadata, nil
}

// SavePartV3 saves a single part's data to .armor/multipart/<id>/part-<n>.json
// for format version 3.
func (m *MultipartStateManager) SavePartV3(ctx context.Context, uploadID string, partData *PartDataV3) error {
	data, err := json.Marshal(partData)
	if err != nil {
		return fmt.Errorf("failed to marshal part data: %w", err)
	}

	key := fmt.Sprintf(".armor/multipart/%s/part-%d.json", uploadID, partData.PartNumber)
	if err := m.backend.Put(ctx, m.bucket, key, bytes.NewReader(data), int64(len(data)), nil); err != nil {
		return fmt.Errorf("failed to save part data: %w", err)
	}

	return nil
}

// LoadPartV3 loads a single part's data from .armor/multipart/<id>/part-<n>.json
// for format version 3.
func (m *MultipartStateManager) LoadPartV3(ctx context.Context, uploadID string, partNumber int) (*PartDataV3, error) {
	key := fmt.Sprintf(".armor/multipart/%s/part-%d.json", uploadID, partNumber)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load part data: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read part data: %w", err)
	}

	var partData PartDataV3
	if err := json.Unmarshal(data, &partData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal part data: %w", err)
	}

	return &partData, nil
}

// ListPartsV3 lists all parts for a v3 multipart upload by reading the part-<n>.json files.
func (m *MultipartStateManager) ListPartsV3(ctx context.Context, uploadID string) (map[int]*PartDataV3, error) {
	prefix := fmt.Sprintf(".armor/multipart/%s/part-", uploadID)

	listResult, err := m.backend.List(ctx, m.bucket, prefix, "", "", 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list parts: %w", err)
	}

	parts := make(map[int]*PartDataV3)

	// Load each part file
	for _, obj := range listResult.Objects {
		// Extract part number from key: .armor/multipart/<id>/part-<n>.json
		// Key format: .armor/multipart/{uploadID}/part-{partNumber}.json
		partStr := obj.Key[len(prefix):]
		partStr = strings.TrimPrefix(partStr, "part-")
		if idx := strings.LastIndex(partStr, ".json"); idx != -1 {
			partStr = partStr[:idx]
		}
		partNumber, err := strconv.Atoi(partStr)
		if err != nil {
			continue // Skip malformed keys
		}

		partData, err := m.LoadPartV3(ctx, uploadID, partNumber)
		if err != nil {
			continue // Skip parts that fail to load
		}

		parts[partNumber] = partData
	}

	// Also check if there are more objects
	for listResult.IsTruncated {
		listResult, err = m.backend.List(ctx, m.bucket, prefix, "", listResult.NextToken, 1000)
		if err != nil {
			break
		}
		for _, obj := range listResult.Objects {
			partStr := obj.Key[len(prefix):]
			partStr = strings.TrimPrefix(partStr, "part-")
			if idx := strings.LastIndex(partStr, ".json"); idx != -1 {
				partStr = partStr[:idx]
			}
			partNumber, err := strconv.Atoi(partStr)
			if err != nil {
				continue
			}

			partData, err := m.LoadPartV3(ctx, uploadID, partNumber)
			if err != nil {
				continue
			}

			parts[partNumber] = partData
		}
	}

	return parts, nil
}

// HMACTableSidecar represents the HMAC table stored as a sidecar object.
// For multipart uploads, the HMAC table is stored at .armor/hmac/<sha256(key)>
type HMACTableSidecar struct {
	Key        string   `json:"key"`         // Object key
	BlockHMACs [][]byte `json:"block_hmacs"` // HMAC for each block (v1/v2 format)
	BlockSize  int      `json:"block_size"`
	Version    int      `json:"version"` // Envelope version (1 or 2)
}

// HMACTableSidecarV3 represents the v3 HMAC table stored as a gzip-compressed JSON sidecar.
// For v3 multipart uploads, the sidecar contains per-part block information with HMACs and
// ciphertext lengths, stored at .armor/hmac/<sha256(key)>.
type HMACTableSidecarV3 struct {
	Version   int          `json:"version"`    // Always 3 for v3 format
	BlockSize int          `json:"block_size"` // Block size in bytes
	Parts     []HMACPartV3 `json:"parts"`      // Array of part information
}

// HMACPartV3 represents a single part's HMAC and length information in the v3 sidecar.
type HMACPartV3 struct {
	N             int        `json:"n"`              // Part number (1-based)
	PlaintextLen  int64      `json:"plaintext_len"`  // Length of plaintext part
	CiphertextLen int64      `json:"ciphertext_len"` // Length of encrypted part
	Blocks        [][]string `json:"blocks"`         // Array of [hmac_base64, clen] for each block
}

// ToBlockTable converts a v3 sidecar to a crypto.BlockTable for decryption.
// This method processes all parts in the sidecar and creates a flat block table
// with entries for all blocks across all parts.
func (s *HMACTableSidecarV3) ToBlockTable(blockSize int) *crypto.BlockTable {
	// Count total blocks across all parts
	totalBlocks := 0
	for _, part := range s.Parts {
		totalBlocks += len(part.Blocks)
	}

	table := crypto.NewBlockTable(blockSize, totalBlocks)

	// Process each part's blocks
	for _, part := range s.Parts {
		for _, blockData := range part.Blocks {
			if len(blockData) != 2 {
				continue // Skip malformed entries
			}

			// Decode HMAC from base64
			hmacBytes, err := base64.StdEncoding.DecodeString(blockData[0])
			if err != nil {
				continue // Skip entries with invalid HMAC
			}

			// Parse ciphertext length (with compression flag)
			var clen uint32
			if _, err := fmt.Sscanf(blockData[1], "%d", &clen); err != nil {
				continue // Skip entries with invalid length
			}

			// Create block table entry
			entry := &crypto.BlockTableEntry{}
			copy(entry.HMAC[:], hmacBytes)
			entry.CiphertextLength = clen

			// Add to table
			table.AddEntry(entry)
		}
	}

	return table
}

// SaveHMACTable saves the HMAC table as a sidecar object.
func (m *MultipartStateManager) SaveHMACTable(ctx context.Context, key string, hmacs [][]byte, blockSize int, version int) error {
	// Compute SHA-256 of the key for the sidecar name
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	sidecar := HMACTableSidecar{
		Key:        key,
		BlockHMACs: hmacs,
		BlockSize:  blockSize,
		Version:    version,
	}

	data, err := json.Marshal(sidecar)
	if err != nil {
		return fmt.Errorf("failed to marshal HMAC table: %w", err)
	}

	if err := m.backend.Put(ctx, m.bucket, sidecarKey, bytes.NewReader(data), int64(len(data)), nil); err != nil {
		return fmt.Errorf("failed to save HMAC table: %w", err)
	}

	return nil
}

// SaveHMACTableV3 saves the v3 HMAC table as a gzip-compressed JSON sidecar object.
// The sidecar format is: {"version":3,"block_size":...,"parts":[{"n","plaintext_len","ciphertext_len","blocks":[[hmac_b64, clen],...]}]}
func (m *MultipartStateManager) SaveHMACTableV3(ctx context.Context, key string, blockSize int, parts []HMACPartV3) error {
	// Compute SHA-256 of the key for the sidecar name
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	sidecar := HMACTableSidecarV3{
		Version:   3,
		BlockSize: blockSize,
		Parts:     parts,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(sidecar)
	if err != nil {
		return fmt.Errorf("failed to marshal v3 HMAC table: %w", err)
	}

	// Compress with gzip
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(jsonData); err != nil {
		return fmt.Errorf("failed to compress v3 HMAC table: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Save the compressed data
	if err := m.backend.Put(ctx, m.bucket, sidecarKey, bytes.NewReader(buf.Bytes()), int64(buf.Len()), nil); err != nil {
		return fmt.Errorf("failed to save v3 HMAC table: %w", err)
	}

	return nil
}

// LoadHMACTable loads the HMAC table from a sidecar object.
func (m *MultipartStateManager) LoadHMACTable(ctx context.Context, key string) (*HMACTableSidecar, error) {
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, sidecarKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load HMAC table: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("failed to read HMAC table: %w", err)
	}

	var sidecar HMACTableSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, fmt.Errorf("failed to unmarshal HMAC table: %w", err)
	}

	return &sidecar, nil
}

// LoadHMACTableV3 loads the v3 HMAC table from a gzip-compressed JSON sidecar object.
func (m *MultipartStateManager) LoadHMACTableV3(ctx context.Context, key string) (*HMACTableSidecarV3, error) {
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	body, _, err := m.backend.GetDirect(ctx, m.bucket, sidecarKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load v3 HMAC table: %w", err)
	}
	defer body.Close()

	// Try to decompress with gzip (v3 format)
	gz, err := gzip.NewReader(body)
	if err != nil {
		return nil, fmt.Errorf("failed to open gzip stream: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("failed to read v3 HMAC table: %w", err)
	}

	var sidecar HMACTableSidecarV3
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return nil, fmt.Errorf("failed to unmarshal v3 HMAC table: %w", err)
	}

	return &sidecar, nil
}

// DeleteHMACTable deletes the HMAC table sidecar object.
func (m *MultipartStateManager) DeleteHMACTable(ctx context.Context, key string) error {
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	if err := m.backend.Delete(ctx, m.bucket, sidecarKey); err != nil {
		return fmt.Errorf("failed to delete HMAC table: %w", err)
	}
	return nil
}

// ComputeBlockHMACs computes HMACs for each block in the data.
func ComputeBlockHMACs(data []byte, blockSize int, hmacKey []byte, startBlockIndex uint32) ([][]byte, error) {
	blockCount := (len(data) + blockSize - 1) / blockSize
	hmacs := make([][]byte, blockCount)

	for i := 0; i < blockCount; i++ {
		start := i * blockSize
		end := start + blockSize
		if end > len(data) {
			end = len(data)
		}
		block := data[start:end]
		blockIndex := startBlockIndex + uint32(i)

		mac := hmac.New(sha256.New, hmacKey)
		indexBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(indexBytes, blockIndex)
		mac.Write(indexBytes)
		mac.Write(block)
		hmacs[i] = mac.Sum(nil)
	}

	return hmacs, nil
}

// EncodeHMACToBase64 encodes a slice of HMACs to a single base64 string.
func EncodeHMACToBase64(hmacs [][]byte) string {
	// Concatenate all HMACs
	total := make([]byte, 0, len(hmacs)*32)
	for _, h := range hmacs {
		total = append(total, h...)
	}
	return base64.StdEncoding.EncodeToString(total)
}

// DecodeHMACFromBase64 decodes a base64 string to a slice of HMACs.
func DecodeHMACFromBase64(encoded string) ([][]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 HMACs: %w", err)
	}

	if len(data)%32 != 0 {
		return nil, fmt.Errorf("invalid HMAC data length: %d", len(data))
	}

	count := len(data) / 32
	hmacs := make([][]byte, count)
	for i := 0; i < count; i++ {
		hmacs[i] = data[i*32 : (i+1)*32]
	}

	return hmacs, nil
}

// CombinePartPlaintextSHAs assembles the per-part plaintext SHA-256 digests
// (hex) into a single whole-object digest. Because parts arrive out of order
// (ADR-005), the per-part digests cannot be streamed into one SHA-256 during
// upload; instead each part's plaintext is hashed at UploadPart time and the
// digests are combined here, in ascending part-number order, by feeding the
// raw 32-byte digests through a single SHA-256 hasher. The result is
// deterministic and reproducible: a reader that splits the decrypted plaintext
// at the uniform part-size P boundaries, hashes each chunk, and hashes the
// concatenated digests arrives at the same value (see ComputeMultipartDigest).
// partNumbers is the authoritative, already-sorted set of part numbers to
// include; any number missing from partSHAs yields an error so a gap can never
// silently produce a wrong digest.
func CombinePartPlaintextSHAs(partSHAs map[int]string, partNumbers []int) (string, error) {
	h := sha256.New()
	for _, n := range partNumbers {
		hexDigest, ok := partSHAs[n]
		if !ok {
			return "", fmt.Errorf("missing plaintext SHA-256 for part %d", n)
		}
		digest, err := hex.DecodeString(hexDigest)
		if err != nil || len(digest) != sha256.Size {
			return "", fmt.Errorf("invalid plaintext SHA-256 for part %d: %q", n, hexDigest)
		}
		h.Write(digest)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeMultipartDigest recomputes the multipart whole-object plaintext
// digest from the full decrypted plaintext by splitting it at uniform
// part-size P boundaries (the last chunk may be short) and hashing the
// concatenated per-chunk SHA-256 digests. This mirrors CombinePartPlaintextSHAs
// and lets a verifier (which only sees the assembled plaintext, not the upload
// parts) recompute the exact digest CompleteMultipartUpload stored. P must be
// the same uniform part size the upload pinned (ADR-005); P <= 0 yields the
// plain SHA-256 of the whole plaintext (single-part / unknown-part-size case).
func ComputeMultipartDigest(plaintext []byte, partSize int64) string {
	if partSize <= 0 {
		sum := sha256.Sum256(plaintext)
		return hex.EncodeToString(sum[:])
	}
	h := sha256.New()
	for off := int64(0); off < int64(len(plaintext)); {
		end := off + partSize
		if end > int64(len(plaintext)) {
			end = int64(len(plaintext))
		}
		partHash := sha256.Sum256(plaintext[off:end])
		h.Write(partHash[:])
		off = end
	}
	return hex.EncodeToString(h.Sum(nil))
}

// MultipartDigestAccumulator incrementally reproduces the combined per-part
// plaintext digest — the value CompleteMultipartUpload stores via
// CombinePartPlaintextSHAs and that ComputeMultipartDigest recomputes from full
// plaintext — from a stream of decrypted, block-aligned chunks. It exists so
// the streaming GET path (handleFullObjectStream) can verify multipart objects
// without buffering their (potentially many-gigabyte) plaintext.
//
// The uniform part size P is block-aligned (ADR-005 — non-block-aligned parts
// are rejected at upload), so each part spans exactly P/blockSize blocks (the
// final part may be shorter). As each block's plaintext arrives it is folded
// into the current part's hash; when a part boundary is reached, or the final
// block lands, the part digest is fed into the combined hasher — exactly what
// CombinePartPlaintextSHAs does at Complete in ascending part order. The result
// therefore equals both ComputeMultipartDigest(fullPlaintext, P) and the stored
// metadata digest, giving the read path a like-for-like integrity check.
//
// Callers MUST pass isLastBlock=true on the final block so the (possibly short)
// final part is finalized; otherwise its digest is dropped and Sum() is wrong.
type MultipartDigestAccumulator struct {
	combined      hash.Hash
	part          hash.Hash
	blocksPerPart int64
	blocksInPart  int64
}

// NewMultipartDigestAccumulator builds an accumulator for a uniform part size P
// (bytes) and the given encryption block size. P must be a positive multiple of
// blockSize; the caller (handleFullObjectStream) guards that and falls back to
// the plain whole-object digest otherwise, so this constructor assumes valid
// inputs.
func NewMultipartDigestAccumulator(partSize int64, blockSize int) *MultipartDigestAccumulator {
	return &MultipartDigestAccumulator{
		combined:      sha256.New(),
		part:          sha256.New(),
		blocksPerPart: partSize / int64(blockSize),
	}
}

// WriteBlock folds one decrypted block's plaintext into the accumulator. Pass
// isLastBlock=true only for the final block of the object so the trailing
// (possibly short) part is finalized. A block that both completes a part and is
// the last block finalizes exactly one part (the conditions share one branch).
func (a *MultipartDigestAccumulator) WriteBlock(plaintext []byte, isLastBlock bool) {
	a.part.Write(plaintext)
	a.blocksInPart++
	if a.blocksInPart == a.blocksPerPart || isLastBlock {
		a.combined.Write(a.part.Sum(nil))
		a.part.Reset()
		a.blocksInPart = 0
	}
}

// Sum returns the hex-encoded combined per-part digest.
func (a *MultipartDigestAccumulator) Sum() string {
	return hex.EncodeToString(a.combined.Sum(nil))
}
