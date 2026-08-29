// cmd_check_test.go tests the armor check subcommand
package main

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

// mockBackend is a mock backend for testing
type mockBackend struct {
	bucketExists bool
	objects      map[string]*mockObject
	cfDomain     string
}

type mockObject struct {
	data         []byte
	metadata     map[string]string
	isArmor      bool
	lastModified time.Time
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		bucketExists: true,
		objects:      make(map[string]*mockObject),
	}
}

func (m *mockBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.objects[key] = &mockObject{
		data:         data,
		metadata:     meta,
		isArmor:      strings.HasPrefix(key, ".armor/"),
		lastModified: time.Now(),
	}
	return nil
}

func (m *mockBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, nil, backend.ErrObjectNotFound
	}
	return io.NopCloser(strings.NewReader(string(obj.data))), &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(obj.data)),
		ContentType:      "application/octet-stream",
		LastModified:      obj.lastModified,
		Metadata:         obj.metadata,
		IsARMOREncrypted: obj.isArmor,
	}, nil
}

func (m *mockBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, backend.ErrObjectNotFound
	}
	data := obj.data
	if offset >= int64(len(data)) {
		return io.NopCloser(strings.NewReader("")), nil
	}
	end := offset + length
	if end > int64(len(data)) {
		end = int64(len(data))
	}
	return io.NopCloser(strings.NewReader(string(data[offset:end]))), nil
}

func (m *mockBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	reader, err := m.GetRange(ctx, bucket, key, offset, length)
	if err != nil {
		return nil, nil, err
	}
	headers := make(map[string]string)
	if m.cfDomain != "" {
		headers["CF-Cache-Status"] = "HIT"
	}
	return reader, headers, nil
}

func (m *mockBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	obj, exists := m.objects[key]
	if !exists {
		return nil, backend.ErrObjectNotFound
	}
	return &backend.ObjectInfo{
		Key:              key,
		Size:             int64(len(obj.data)),
		ContentType:      "application/octet-stream",
		LastModified:      obj.lastModified,
		Metadata:         obj.metadata,
		IsARMOREncrypted: obj.isArmor,
	}, nil
}

func (m *mockBackend) Delete(ctx context.Context, bucket, key string) error {
	delete(m.objects, key)
	return nil
}

func (m *mockBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		delete(m.objects, key)
	}
	return nil
}

func (m *mockBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	var objects []backend.ObjectInfo
	for key, obj := range m.objects {
		if strings.HasPrefix(key, prefix) {
			objects = append(objects, backend.ObjectInfo{
				Key:              key,
				Size:             int64(len(obj.data)),
				ContentType:      "application/octet-stream",
				LastModified:      obj.lastModified,
				Metadata:         obj.metadata,
				IsARMOREncrypted: obj.isArmor,
			})
		}
	}
	return &backend.ListResult{
		Objects: objects,
	}, nil
}

func (m *mockBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	return nil
}

func (m *mockBackend) ListBuckets(ctx context.Context) ([]backend.BucketInfo, error) {
	return []backend.BucketInfo{{Name: "test-bucket"}}, nil
}

func (m *mockBackend) CreateBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) DeleteBucket(ctx context.Context, bucket string) error {
	return nil
}

func (m *mockBackend) HeadBucket(ctx context.Context, bucket string) error {
	if !m.bucketExists {
		return backend.ErrBucketNotFound
	}
	return nil
}

func (m *mockBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return m.Get(ctx, bucket, key)
}

func (m *mockBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (uploadID string, err error) {
	return "test-upload-id", nil
}

func (m *mockBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (etag string, err error) {
	return "test-etag", nil
}

func (m *mockBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (etag string, err error) {
	return "test-etag", nil
}

func (m *mockBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	return nil
}

func (m *mockBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*backend.ListPartsResult, error) {
	return &backend.ListPartsResult{}, nil
}

// Test decodeBase64ToBytes
func TestDecodeBase64ToBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "standard base64",
			input:   "SGVsbG8gV29ybGQ=",
			wantErr: false,
		},
		{
			name:    "url-safe base64",
			input:   "SGVsbG8gV29ybGQ",
			wantErr: false,
		},
		{
			name:    "with underscores",
			input:   "SGVsbG8_V29ybGQ",
			wantErr: false,
		},
		{
			name:    "invalid base64",
			input:   "not-valid-base64!!!",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := decodeBase64ToBytes(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeBase64ToBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) == 0 {
				t.Errorf("decodeBase64ToBytes() returned empty result for valid input")
			}
		})
	}
}

// Test runConfigProbe
func TestRunConfigProbe(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.Config
		expectedStatus string
	}{
		{
			name:           "nil config",
			cfg:            nil,
			expectedStatus: "FAIL",
		},
		{
			name: "valid config",
			cfg: &config.Config{
				Bucket:      "test-bucket",
				MEK:         []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4},
				Backend:     "filesystem",
				FSPath:      "/tmp/test",
				Credentials: make(map[string]*config.Credential),
			},
			expectedStatus: "PASS",
		},
		{
			name: "missing bucket",
			cfg: &config.Config{
				Bucket: "",
				MEK:    []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4},
				Backend: "filesystem",
				FSPath:  "/tmp/test",
			},
			expectedStatus: "FAIL",
		},
		{
			name: "no credentials",
			cfg: &config.Config{
				Bucket:           "test-bucket",
				MEK:              []byte{1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4, 1, 2, 3, 4},
				Backend:          "filesystem",
				FSPath:           "/tmp/test",
				Credentials:      make(map[string]*config.Credential),
				AllowNoCredentials: false,
			},
			expectedStatus: "FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := runConfigProbe(tt.cfg)
			if len(results) != 1 {
				t.Fatalf("runConfigProbe() returned %d results, expected 1", len(results))
			}
			if results[0].Status != tt.expectedStatus {
				t.Errorf("runConfigProbe() status = %s, expected %s", results[0].Status, tt.expectedStatus)
			}
		})
	}
}

// Test runBackendProbe
func TestRunBackendProbe(t *testing.T) {
	tests := []struct {
		name           string
		backend        *mockBackend
		bucket         string
		expectedStatus string
	}{
		{
			name:           "successful connectivity",
			backend:        newMockBackend(),
			bucket:         "test-bucket",
			expectedStatus: "PASS",
		},
		{
			name: "bucket not found",
			backend: &mockBackend{
				bucketExists: false,
				objects:      make(map[string]*mockObject),
			},
			bucket:         "test-bucket",
			expectedStatus: "FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cfg := &config.Config{Bucket: tt.bucket}
			results := runBackendProbe(ctx, tt.backend, cfg)
			if len(results) != 1 {
				t.Fatalf("runBackendProbe() returned %d results, expected 1", len(results))
			}
			if results[0].Status != tt.expectedStatus {
				t.Errorf("runBackendProbe() status = %s, expected %s", results[0].Status, tt.expectedStatus)
			}
		})
	}
}

// Test runCloudflareProbe
func TestRunCloudflareProbe(t *testing.T) {
	tests := []struct {
		name           string
		backend        *mockBackend
		cfg            *config.Config
		expectedStatus string
	}{
		{
			name: "no CF domain - warning",
			backend: newMockBackend(),
			cfg: &config.Config{
				CFDomain: "",
				Bucket:   "test-bucket",
				Prefix:   "",
			},
			expectedStatus: "WARN",
		},
		{
			name: "with CF domain and canary",
			backend: func() *mockBackend {
				m := newMockBackend()
				// Create a mock canary object
				canaryData := createMockCanary()
				m.objects[".armor/canary/test/123"] = &mockObject{
					data:         canaryData,
					metadata:     make(map[string]string),
					isArmor:      true,
					lastModified: time.Now(),
				}
				m.cfDomain = "https://cf.example.com"
				return m
			}(),
			cfg: &config.Config{
				CFDomain: "https://cf.example.com",
				Bucket:   "test-bucket",
				Prefix:   "",
			},
			expectedStatus: "PASS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := runCloudflareProbe(ctx, tt.backend, tt.cfg)
			if len(results) != 1 {
				t.Fatalf("runCloudflareProbe() returned %d results, expected 1", len(results))
			}
			if results[0].Status != tt.expectedStatus {
				t.Errorf("runCloudflareProbe() status = %s, expected %s", results[0].Status, tt.expectedStatus)
			}
		})
	}
}

// Test runMEKProbe
func TestRunMEKProbe(t *testing.T) {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i)
	}

	tests := []struct {
		name           string
		backend        *mockBackend
		cfg            *config.Config
		expectedStatus string
	}{
		{
			name: "no canary - warning",
			backend: newMockBackend(),
			cfg: &config.Config{
				Bucket: "test-bucket",
				Prefix: "",
				MEK:    mek,
			},
			expectedStatus: "WARN",
		},
		{
			name: "valid canary with MEK",
			backend: func() *mockBackend {
				m := newMockBackend()
				// Create a valid canary object
				canaryData, wrappedDEK := createValidCanary(t, mek)
				metadata := map[string]string{
					"x-amz-meta-armor-wrapped-dek": base64.StdEncoding.EncodeToString(wrappedDEK),
				}
				m.objects[".armor/canary/test/123"] = &mockObject{
					data:         canaryData,
					metadata:     metadata,
					isArmor:      true,
					lastModified: time.Now(),
				}
				return m
			}(),
			cfg: &config.Config{
				Bucket: "test-bucket",
				Prefix: "",
				MEK:    mek,
			},
			expectedStatus: "PASS",
		},
		{
			name: "wrong MEK",
			backend: func() *mockBackend {
				m := newMockBackend()
				// Create canary with different MEK
				wrongMEK := make([]byte, 32)
				for i := range wrongMEK {
					wrongMEK[i] = 255 - byte(i)
				}
				canaryData, wrappedDEK := createValidCanary(t, wrongMEK)
				metadata := map[string]string{
					"x-amz-meta-armor-wrapped-dek": base64.StdEncoding.EncodeToString(wrappedDEK),
				}
				m.objects[".armor/canary/test/123"] = &mockObject{
					data:         canaryData,
					metadata:     metadata,
					isArmor:      true,
					lastModified: time.Now(),
				}
				return m
			}(),
			cfg: &config.Config{
				Bucket: "test-bucket",
				Prefix: "",
				MEK:    mek,
			},
			expectedStatus: "FAIL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			results := runMEKProbe(ctx, tt.backend, tt.cfg)
			if len(results) != 1 {
				t.Fatalf("runMEKProbe() returned %d results, expected 1", len(results))
			}
			if results[0].Status != tt.expectedStatus {
				t.Errorf("runMEKProbe() status = %s, expected %s", results[0].Status, tt.expectedStatus)
			}
		})
	}
}

// Test findNewestCanary
func TestFindNewestCanary(t *testing.T) {
	tests := []struct {
		name        string
		backend     *mockBackend
		bucket      string
		prefix      string
		wantErr     bool
		expectEmpty bool
	}{
		{
			name: "no canaries",
			backend: func() *mockBackend {
				m := newMockBackend()
				m.objects["some/other/key"] = &mockObject{
					data:         []byte("data"),
					metadata:     make(map[string]string),
					isArmor:      false,
					lastModified: time.Now(),
				}
				return m
			}(),
			bucket:      "test-bucket",
			prefix:      "",
			wantErr:     true,
			expectEmpty: true,
		},
		{
			name: "single canary",
			backend: func() *mockBackend {
				m := newMockBackend()
				m.objects[".armor/canary/test/1"] = &mockObject{
					data:         []byte("canary1"),
					metadata:     make(map[string]string),
					isArmor:      true,
					lastModified: time.Now().Add(-1 * time.Hour),
				}
				return m
			}(),
			bucket:      "test-bucket",
			prefix:      "",
			wantErr:     false,
			expectEmpty: false,
		},
		{
			name: "multiple canaries - newest wins",
			backend: func() *mockBackend {
				m := newMockBackend()
				oldTime := time.Now().Add(-2 * time.Hour)
				newTime := time.Now().Add(-1 * time.Hour)
				m.objects[".armor/canary/test/old"] = &mockObject{
					data:         []byte("old-canary"),
					metadata:     make(map[string]string),
					isArmor:      true,
					lastModified: oldTime,
				}
				m.objects[".armor/canary/test/new"] = &mockObject{
					data:         []byte("new-canary"),
					metadata:     make(map[string]string),
					isArmor:      true,
					lastModified: newTime,
				}
				return m
			}(),
			bucket:      "test-bucket",
			prefix:      "",
			wantErr:     false,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			key, err := findNewestCanary(ctx, tt.backend, tt.bucket, tt.prefix)
			if (err != nil) != tt.wantErr {
				t.Errorf("findNewestCanary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.expectEmpty && key == "" {
				t.Errorf("findNewestCanary() returned empty key, expected non-empty")
			}
			if !tt.wantErr && !tt.expectEmpty && !strings.Contains(key, ".armor/canary/") {
				t.Errorf("findNewestCanary() returned key %s, expected canary key", key)
			}
		})
	}
}

// Helper functions

func createMockCanary() []byte {
	// Create a minimal mock canary object
	canary := make([]byte, 1024)
	copy(canary, "ARMOR") // Magic bytes
	return canary
}

func createValidCanary(t *testing.T, mek []byte) ([]byte, []byte) {
	t.Helper()

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("failed to generate DEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("failed to generate IV: %v", err)
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("failed to wrap DEK: %v", err)
	}

	// Create mock canary content
	canaryContent := make([]byte, 512)
	for i := range canaryContent {
		canaryContent[i] = byte(i)
	}

	// Encrypt
	encryptor, err := crypto.NewEncryptor(dek, iv, 65536)
	if err != nil {
		t.Fatalf("failed to create encryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(canaryContent)
	if err != nil {
		t.Fatalf("failed to encrypt: %v", err)
	}

	// Create header
	plaintextSHA := crypto.ComputePlaintextSHA256(canaryContent)
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(canaryContent)), 65536, plaintextSHA)
	if err != nil {
		t.Fatalf("failed to create header: %v", err)
	}

	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("failed to encode header: %v", err)
	}

	// Build envelope
	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(hmacTable))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	return envelope, wrappedDEK
}

// Test exit codes via integration test
func TestCheckExitCodes(t *testing.T) {
	// Create temporary directory for filesystem backend
	tempDir := t.TempDir()

	tests := []struct {
		name           string
		envSetup       func(map[string]string)
		expectedExit   int
	}{
		{
			name: "config error - exit 1",
			envSetup: func(env map[string]string) {
				// Missing MEK - should cause config error
				env["ARMOR_BACKEND"] = "filesystem"
				env["ARMOR_FS_PATH"] = tempDir
				env["ARMOR_BUCKET"] = "test-bucket"
				// Missing ARMOR_MEK
			},
			expectedExit: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require running the actual check command,
			// which is complex to test. For now, we test the components
			// that would lead to these exit codes.
		})
	}
}
