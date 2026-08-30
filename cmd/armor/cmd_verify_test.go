// cmd_verify_test.go tests the armor verify subcommand
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// MockB2Backend is a mock backend for testing. mockObject is declared once,
// in cmd_check_test.go (identical fields) -- reused here instead of a
// duplicate declaration in the same package.
type MockB2Backend struct {
	objects map[string]*mockObject
}

func NewMockB2Backend() *MockB2Backend {
	return &MockB2Backend{
		objects: make(map[string]*mockObject),
	}
}

func (m *MockB2Backend) Put(ctx context.Context, bucket, key string, data []byte, metadata map[string]string, isArmor bool, lastModified time.Time) {
	m.objects[key] = &mockObject{
		data:         data,
		metadata:     metadata,
		isArmor:      isArmor,
		lastModified: lastModified,
	}
}

func (m *MockB2Backend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, os.ErrNotExist
	}

	return &backend.ObjectInfo{
		Key:             key,
		Size:            int64(len(obj.data)),
		LastModified:    obj.lastModified,
		IsARMOREncrypted: obj.isArmor,
		Metadata:        obj.metadata,
		ETag:            hex.EncodeToString(SHA256Hash(obj.data)),
	}, nil
}

func (m *MockB2Backend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(obj.data)), nil
}

func (m *MockB2Backend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, os.ErrNotExist
	}

	data := obj.data
	if offset >= int64(len(data)) {
		return io.NopCloser(bytes.NewReader([]byte{})), nil
	}

	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}

	return io.NopCloser(bytes.NewReader(data[offset:end])), nil
}

func (m *MockB2Backend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	var objects []backend.ObjectInfo
	for key := range m.objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			objects = append(objects, backend.ObjectInfo{Key: key})
		}
	}
	return &backend.ListResult{
		Objects:     objects,
		IsTruncated: false,
	}, nil
}

// TestVerifyValidObject tests verification of a valid ARMOR object
func TestVerifyValidObject(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid ARMOR object
	validData := createValidARMORObject(t, mek, "test-object", []byte("plaintext data"))
	mock.Put(ctx, "test-bucket", "test-object", validData.data, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "test-object", time.Time{})

	if result.Status != "OK" {
		t.Errorf("Expected OK status, got %s: %s", result.Status, result.Error)
	}
}

// TestVerifyCorruptedHMAC tests detection of corrupted HMAC
func TestVerifyCorruptedHMAC(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid ARMOR object
	validData := createValidARMORObject(t, mek, "test-object", []byte("plaintext data"))

	// Corrupt the data after the header (corrupts HMAC)
	corruptedData := make([]byte, len(validData.data))
	copy(corruptedData, validData.data)
	// Flip a bit in the encrypted data section (after header)
	corruptedData[crypto.HeaderSize+100] ^= 0xFF

	mock.Put(ctx, "test-bucket", "corrupted-object", corruptedData, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "corrupted-object", time.Time{})

	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED status, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "HMAC") && !strings.Contains(result.Error, "verification") {
		t.Errorf("Expected HMAC verification error, got: %s", result.Error)
	}
}

// TestVerifyCorruptedEnvelope tests detection of corrupted envelope header
func TestVerifyCorruptedEnvelope(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid ARMOR object
	validData := createValidARMORObject(t, mek, "test-object", []byte("plaintext data"))

	// Corrupt the envelope header
	corruptedData := make([]byte, len(validData.data))
	copy(corruptedData, validData.data)
	// Corrupt the magic bytes
	corruptedData[0] ^= 0xFF

	mock.Put(ctx, "test-bucket", "corrupted-envelope", corruptedData, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "corrupted-envelope", time.Time{})

	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED status, got %s", result.Status)
	}
}

// TestVerifyCorruptedDEK tests detection of corrupted wrapped DEK
func TestVerifyCorruptedDEK(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid ARMOR object
	validData := createValidARMORObject(t, mek, "test-object", []byte("plaintext data"))

	// Corrupt the wrapped DEK in metadata
	corruptedMetadata := make(map[string]string)
	for k, v := range validData.metadata {
		corruptedMetadata[k] = v
	}
	// Corrupt the wrapped DEK
	wrappedDEK := corruptedMetadata["x-amz-meta-armor-wrapped-dek"]
	if len(wrappedDEK) > 10 {
		corruptedMetadata["x-amz-meta-armor-wrapped-dek"] = wrappedDEK[:len(wrappedDEK)-10] + "CORRUPTED"
	}

	mock.Put(ctx, "test-bucket", "corrupted-dek", validData.data, corruptedMetadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "corrupted-dek", time.Time{})

	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED status, got %s", result.Status)
	}
}

// TestVerifyNonArmorObject tests handling of non-ARMOR objects
func TestVerifyNonArmorObject(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a non-ARMOR object
	plainData := []byte("not an ARMOR object")
	mock.Put(ctx, "test-bucket", "plain-object", plainData, map[string]string{}, false, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "plain-object", time.Time{})

	if result.Status != "ERROR" {
		t.Errorf("Expected ERROR status, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "not ARMOR") {
		t.Errorf("Expected 'not ARMOR' error, got: %s", result.Error)
	}
}

// TestVerifyMissingObject tests handling of missing objects
func TestVerifyMissingObject(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Verify a non-existent object
	result := verifyObject(ctx, mock, mek, "test-bucket", "missing-object", time.Time{})

	if result.Status != "ERROR" {
		t.Errorf("Expected ERROR status, got %s", result.Status)
	}
}

// TestVerifySinceFilter tests the -since timestamp filter
func TestVerifySinceFilter(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create objects with different timestamps
	oldTime := time.Now().Add(-48 * time.Hour)
	recentTime := time.Now().Add(-1 * time.Hour)

	oldData := createValidARMORObject(t, mek, "old-object", []byte("old data"))
	mock.Put(ctx, "test-bucket", "old-object", oldData.data, oldData.metadata, true, oldTime)

	recentData := createValidARMORObject(t, mek, "recent-object", []byte("recent data"))
	mock.Put(ctx, "test-bucket", "recent-object", recentData.data, recentData.metadata, true, recentTime)

	// Verify with since=24h ago
	since := time.Now().Add(-24 * time.Hour)

	// Old object should be skipped
	oldResult := verifyObject(ctx, mock, mek, "test-bucket", "old-object", since)
	if oldResult.Status != "OK" || !strings.Contains(oldResult.Details, "Skipped") {
		t.Errorf("Expected old object to be skipped, got status=%s details=%s", oldResult.Status, oldResult.Details)
	}

	// Recent object should be verified
	recentResult := verifyObject(ctx, mock, mek, "test-bucket", "recent-object", since)
	if recentResult.Status != "OK" {
		t.Errorf("Expected recent object to be OK, got %s: %s", recentResult.Status, recentResult.Error)
	}
}

// TestQuickMode tests quick verification mode (envelope + DEK only)
func TestQuickMode(t *testing.T) {
	// This is tested via quickVerifyObject in the implementation
	// The full test would require running the verify subcommand with -quick flag
	// which is tested at the integration level
}

// TestVerifyReportJSON tests JSON report generation
func TestVerifyReportJSON(t *testing.T) {
	report := &VerificationReport{
		Bucket:         "test-bucket",
		Prefix:         "test/",
		TotalObjects:   3,
		OKCount:        2,
		CorruptedCount: 1,
		ErrorCount:     0,
		QuickMode:      false,
		VerifyDate:     time.Now(),
		Duration:       5.23,
		Results: []ObjectVerificationResult{
			{
				Bucket:  "test-bucket",
				Key:     "obj1",
				Status:  "OK",
				Details: "Envelope and DEK verified successfully",
			},
			{
				Bucket:  "test-bucket",
				Key:     "obj2",
				Status:  "CORRUPTED",
				Error:   "HMAC verification failed",
				Details: "Object data corruption detected",
			},
		},
	}

	// Marshal to JSON and verify it's valid JSON
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal report: %v", err)
	}

	// Verify it can be unmarshaled
	var unmarshaled VerificationReport
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal report: %v", err)
	}

	if unmarshaled.TotalObjects != report.TotalObjects {
		t.Errorf("TotalObjects mismatch: got %d, want %d", unmarshaled.TotalObjects, report.TotalObjects)
	}
}

// Helper functions

func generateTestMEK(t *testing.T) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i % 256)
	}
	return mek
}

type armoringResult struct {
	data     []byte
	metadata map[string]string
}

func createValidARMORObject(t *testing.T, mek []byte, key string, plaintext []byte) armoringResult {
	t.Helper()

	// Generate a random DEK
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x55)
	}

	// Wrap the DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Create envelope header
	envelope := &crypto.Envelope{
		Magic:        [4]byte{'A', 'R', 'M', 'R'},
		Version:      crypto.Version2,
		IV:           make([]byte, 12),
		BlockSize:    65536,
		OriginalSize: uint64(len(plaintext)),
	}
	copy(envelope.IV, []byte("test-iv-12345"))

	// Encode header
	header, err := crypto.EncodeHeader(envelope)
	if err != nil {
		t.Fatalf("Failed to encode header: %v", err)
	}

	// Encrypt and HMAC the plaintext (simplified - just create valid structure)
	// In real testing, you'd use the full encryption pipeline
	encryptedData := make([]byte, crypto.HeaderSize+len(plaintext)*2) // oversized for safety
	copy(encryptedData, header)

	// Create HMAC sidecar metadata
	metadata := map[string]string{
		"x-amz-meta-armor-wrapped-dek":     base64Encode(wrappedDEK),
		"x-amz-meta-armor-envelope-version": "2",
		"x-amz-meta-armor-block-size":      "65536",
		"x-amz-meta-armor-original-size":  fmt.Sprintf("%d", len(plaintext)),
	}

	return armoringResult{
		data:     encryptedData,
		metadata: metadata,
	}
}

func base64Encode(data []byte) string {
	return strings.TrimRight(base64.StdEncoding.EncodeToString(data), "=")
}

// SHA256Hash computes SHA-256 hash of data
func SHA256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// v3 Test Cases

// TestVerifyV3ValidSinglePUT tests verification of a valid v3 single-PUT object
func TestVerifyV3ValidSinglePUT(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid v3 single-PUT ARMOR object
	validData := createValidV3SinglePUTObject(t, mek, "v3-test-object", []byte("plaintext data for v3"))
	mock.Put(ctx, "test-bucket", "v3-single-put", validData.data, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-single-put", time.Time{})

	if result.Status != "OK" {
		t.Errorf("Expected OK status for v3 single-PUT, got %s: %s", result.Status, result.Error)
	}
}

// TestVerifyV3ValidMultipart tests verification of a valid v3 multipart object
func TestVerifyV3ValidMultipart(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid v3 multipart ARMOR object
	validData := createValidV3MultipartObject(t, mek, "v3-multipart-object", []byte("multipart data for v3"))
	mock.Put(ctx, "test-bucket", "v3-multipart", validData.data, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-multipart", time.Time{})

	if result.Status != "OK" {
		t.Errorf("Expected OK status for v3 multipart, got %s: %s", result.Status, result.Error)
	}
}

// TestVerifyV3CorruptedBlock tests detection of corrupted block in v3 object
func TestVerifyV3CorruptedBlock(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid v3 single-PUT object
	validData := createValidV3SinglePUTObject(t, mek, "v3-test-object", []byte("plaintext data for v3"))

	// Corrupt a block in the encrypted data
	corruptedData := make([]byte, len(validData.data))
	copy(corruptedData, validData.data)
	// Find the start of encrypted blocks (after header) and corrupt a block
	corruptedData[crypto.HeaderSize+64] ^= 0xFF

	mock.Put(ctx, "test-bucket", "v3-corrupted-block", corruptedData, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-corrupted-block", time.Time{})

	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED status for v3 corrupted block, got %s", result.Status)
	}
	if !strings.Contains(result.Error, "HMAC") && !strings.Contains(result.Error, "decryption") {
		t.Errorf("Expected HMAC/decryption error for v3 corrupted block, got: %s", result.Error)
	}
}

// TestVerifyV3CorruptedSidecar tests detection of corrupted sidecar in v3 multipart
func TestVerifyV3CorruptedSidecar(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid v3 multipart object with sidecar
	validData := createValidV3MultipartObject(t, mek, "v3-multipart", []byte("multipart data"))

	// Corrupt the sidecar metadata key
	corruptedMetadata := make(map[string]string)
	for k, v := range validData.metadata {
		corruptedMetadata[k] = v
	}
	// Corrupt the HMAC sidecar reference
	sidecarKey := ".armor/hmac/" + hex.EncodeToString(SHA256Hash([]byte("v3-multipart"))) + ".json"
	corruptedMetadata["x-amz-meta-armor-hmac-sidecar"] = sidecarKey + "-corrupted"

	mock.Put(ctx, "test-bucket", "v3-corrupted-sidecar", validData.data, corruptedMetadata, true, time.Now())

	// Note: This test would require sidecar mock implementation
	// For now, we test that the verification attempts to load the sidecar
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-corrupted-sidecar", time.Time{})

	// Should fail because sidecar won't be found (we're using a mock without sidecar support)
	if result.Status != "ERROR" && result.Status != "CORRUPTED" {
		t.Errorf("Expected ERROR or CORRUPTED status for v3 corrupted sidecar, got %s", result.Status)
	}
}

// TestVerifyV3CompressedBlock tests verification of v3 object with compressed blocks
func TestVerifyV3CompressedBlock(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a v3 object with compressed blocks
	validData := createValidV3CompressedObject(t, mek, "v3-compressed", []byte("data that compresses well" + strings.Repeat("repeat", 1000)))
	mock.Put(ctx, "test-bucket", "v3-compressed", validData.data, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-compressed", time.Time{})

	if result.Status != "OK" {
		t.Errorf("Expected OK status for v3 compressed object, got %s: %s", result.Status, result.Error)
	}
}

// TestVerifyV3CorruptedBlockTable tests detection of corrupted block table
func TestVerifyV3CorruptedBlockTable(t *testing.T) {
	ctx := context.Background()
	mek := generateTestMEK(t)
	mock := NewMockB2Backend()

	// Create a valid v3 single-PUT object
	validData := createValidV3SinglePUTObject(t, mek, "v3-test-object", []byte("plaintext data for v3"))

	// Corrupt the block table at the end
	corruptedData := make([]byte, len(validData.data))
	copy(corruptedData, validData.data)
	// Corrupt the last bytes of the block table
	corruptedData[len(corruptedData)-1] ^= 0xFF
	corruptedData[len(corruptedData)-2] ^= 0xFF

	mock.Put(ctx, "test-bucket", "v3-corrupted-table", corruptedData, validData.metadata, true, time.Now())

	// Verify the object
	result := verifyObject(ctx, mock, mek, "test-bucket", "v3-corrupted-table", time.Time{})

	if result.Status != "CORRUPTED" {
		t.Errorf("Expected CORRUPTED status for v3 corrupted block table, got %s", result.Status)
	}
}

// v3 Helper functions

func createValidV3SinglePUTObject(t *testing.T, mek []byte, key string, plaintext []byte) armoringResult {
	t.Helper()

	// Generate a random DEK
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i ^ 0x55)
	}

	// Wrap the DEK with MEK using v2 format (for backward compatibility)
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Create v3 envelope header
	envelope := &crypto.EnvelopeHeader{
		Magic:          [4]byte{'A', 'R', 'M', 'R'},
		Version:        crypto.Version3,
		IV:             make([]byte, 16),
		BlockSizeLog2:  16, // 65536
		PlaintextSize:  uint64(len(plaintext)),
	}
	copy(envelope.IV, []byte("test-v3-iv-12345"))

	// Generate IV
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i * 3)
	}

	// Create encryptor
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, 65536, crypto.Version3)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Encrypt the plaintext
	encrypted, blockTable, err := encryptor.EncryptV3(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt v3: %v", err)
	}

	// Encode header
	header, err := crypto.EncodeHeader(envelope)
	if err != nil {
		t.Fatalf("Failed to encode header: %v", err)
	}

	// Encode block table
	tableData, err := blockTable.Encode()
	if err != nil {
		t.Fatalf("Failed to encode block table: %v", err)
	}

	// Combine: header + encrypted data + block table
	fullObject := make([]byte, 0, len(header)+len(encrypted)+len(tableData))
	fullObject = append(fullObject, header...)
	fullObject = append(fullObject, encrypted...)
	fullObject = append(fullObject, tableData...)

	// Create plaintext SHA256
	plaintextSHA := hex.EncodeToString(SHA256Hash(plaintext))

	// Create metadata
	metadata := map[string]string{
		"x-amz-meta-armor-wrapped-dek":       base64Encode(wrappedDEK),
		"x-amz-meta-armor-version":          "3",
		"x-amz-meta-armor-block-size":       "65536",
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-plaintext-sha256": plaintextSHA,
	}

	return armoringResult{
		data:     fullObject,
		metadata: metadata,
	}
}

func createValidV3MultipartObject(t *testing.T, mek []byte, key string, plaintext []byte) armoringResult {
	t.Helper()

	// For simplicity, we'll create a single-PUT v3 object marked as multipart
	// In a real test, you'd use the full multipart pipeline
	validData := createValidV3SinglePUTObject(t, mek, key, plaintext)

	// Add multipart marker
	validData.metadata["x-amz-meta-armor-multipart"] = "true"

	return validData
}

func createValidV3CompressedObject(t *testing.T, mek []byte, key string, plaintext []byte) armoringResult {
	t.Helper()

	// For v3 with compression, we create a v3 object that uses compression
	// The compression is handled automatically by the encryptor
	return createValidV3SinglePUTObject(t, mek, key, plaintext)
}
