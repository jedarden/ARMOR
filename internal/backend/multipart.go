package backend

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// MultipartState represents the state of an in-progress multipart upload.
// This is stored in B2 at .armor/multipart/<upload-id>.state
type MultipartState struct {
	UploadID     string    `json:"upload_id"`
	Bucket       string    `json:"bucket"`
	Key          string    `json:"key"`
	IV           []byte    `json:"iv"`
	WrappedDEK   []byte    `json:"wrapped_dek"`
	BlockSize    int       `json:"block_size"`
	Created      time.Time `json:"created"`
	ContentType  string    `json:"content_type"`
	KeyID        string    `json:"key_id"` // Key identifier for multi-key support

	// Track cumulative encrypted bytes for CTR counter offset
	EncryptedBytes int64 `json:"encrypted_bytes"`

	// Per-part HMACs (part number -> HMACs for each block in that part)
	// Stored as base64-encoded concatenation of all block HMACs
	PartHMACs map[int]string `json:"part_hmacs"`

	// Per-part encrypted sizes (for range translation on completion)
	PartSizes map[int]int64 `json:"part_sizes"`
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
func (m *MultipartStateManager) DeleteState(ctx context.Context, uploadID string) error {
	key := fmt.Sprintf(".armor/multipart/%s.state", uploadID)
	if err := m.backend.Delete(ctx, m.bucket, key); err != nil {
		return fmt.Errorf("failed to delete multipart state: %w", err)
	}
	return nil
}

// HMACTableSidecar represents the HMAC table stored as a sidecar object.
// For multipart uploads, the HMAC table is stored at .armor/hmac/<sha256(key)>
type HMACTableSidecar struct {
	Key       string   `json:"key"`        // Object key
	BlockHMACs [][]byte `json:"block_hmacs"` // HMAC for each block
	BlockSize int      `json:"block_size"`
}

// SaveHMACTable saves the HMAC table as a sidecar object.
func (m *MultipartStateManager) SaveHMACTable(ctx context.Context, key string, hmacs [][]byte, blockSize int) error {
	// Compute SHA-256 of the key for the sidecar name
	keyHash := sha256.Sum256([]byte(key))
	sidecarKey := fmt.Sprintf(".armor/hmac/%x", keyHash)

	sidecar := HMACTableSidecar{
		Key:        key,
		BlockHMACs: hmacs,
		BlockSize:  blockSize,
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
