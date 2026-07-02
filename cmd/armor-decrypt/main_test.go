// Unit tests for armor-decrypt CLI tool.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// Test helpers

func makeMEK(t *testing.T) []byte {
	t.Helper()
	mek, err := hex.DecodeString("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	return mek
}

// Test encrypt-decrypt round-trip via internal/crypto, decrypt via tool code path
func TestRoundTripSinglePart(t *testing.T) {
	mek := makeMEK(t)

	// Generate DEK and encrypt using internal/crypto
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Hello, ARMOR! This is test data for round-trip encryption.")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	// Create encryptor
	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	// Encrypt the plaintext
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Wrap the DEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	// Build envelope header
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble full envelope file
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	// Now decrypt via the tool's decryptLocal code path
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	// Create inputSource for local file
	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	// Use decryptLocal function
	ctx := context.Background()
	decrypted, err := decryptLocal(ctx, src, mek)
	if err != nil {
		t.Fatalf("decryptLocal: %v", err)
	}

	// Verify the plaintext matches
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data mismatch:\n got: %q\nwant: %q", decrypted, plaintext)
	}
}

// Test encrypt-decrypt round-trip with sidecar HMAC (multipart upload case)
func TestRoundTripSidecarHMAC(t *testing.T) {
	mek := makeMEK(t)

	// Generate DEK and encrypt
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Multipart upload test data with sidecar HMAC.")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	// Build envelope header with sidecar flag (Reserved[1] = 0x01)
	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}
	header.Reserved[1] = 0x01 // Set sidecar HMAC flag

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Write envelope without inline HMAC
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, headerBuf, 0644); err != nil {
		t.Fatalf("write header: %v", err)
	}
	f, err := os.OpenFile(encryptedFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("open for append: %v", err)
	}
	if _, err := f.Write(encrypted); err != nil {
		t.Fatalf("write encrypted data: %v", err)
	}
	f.Close()

	// Write sidecar HMAC - use the actual getSidecarHMACPath function
	sidecarPath := getSidecarHMACPath(encryptedFile)
	if err := os.MkdirAll(filepath.Dir(sidecarPath), 0755); err != nil {
		t.Fatalf("mkdir sidecar dir: %v", err)
	}
	if err := os.WriteFile(sidecarPath, hmacTable, 0644); err != nil {
		t.Fatalf("write sidecar HMAC: %v", err)
	}

	// Decrypt via tool
	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	ctx := context.Background()
	decrypted, err := decryptLocal(ctx, src, mek)
	if err != nil {
		t.Fatalf("decryptLocal with sidecar: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted data mismatch:\n got: %q\nwant: %q", decrypted, plaintext)
	}
}

// Test wrong MEK produces clear error
func TestWrongMEK(t *testing.T) {
	mek := makeMEK(t)

	// Generate DEK and encrypt with correct MEK
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Secret data")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble envelope
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	// Write encrypted file
	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	// Try to decrypt with WRONG MEK
	wrongMEK, _ := hex.DecodeString("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	ctx := context.Background()
	_, err = decryptLocal(ctx, src, wrongMEK)
	if err == nil {
		t.Fatal("expected error with wrong MEK, got nil")
	}

	// Check error message is clear
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unwrap DEK") {
		t.Errorf("error message should mention unwrap failure, got: %v", errMsg)
	}
}

// Test corrupted block produces clear error
func TestCorruptedBlock(t *testing.T) {
	mek := makeMEK(t)

	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}

	plaintext := []byte("Data that will be corrupted")
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	blockSize := 65536

	encryptor, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// CORRUPT the encrypted data
	encrypted[0] ^= 0xff

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, plaintextSHA)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("header.Encode: %v", err)
	}

	// Assemble envelope
	var envelope bytes.Buffer
	envelope.Write(headerBuf)
	envelope.Write(encrypted)
	envelope.Write(hmacTable)

	tmpDir := t.TempDir()
	encryptedFile := filepath.Join(tmpDir, "encrypted.bin")
	if err := os.WriteFile(encryptedFile, envelope.Bytes(), 0644); err != nil {
		t.Fatalf("write encrypted file: %v", err)
	}

	src := &inputSource{
		Type:       "local",
		Path:       encryptedFile,
		WrappedDEK: wrappedDEK,
	}

	ctx := context.Background()
	_, err = decryptLocal(ctx, src, mek)
	if err == nil {
		t.Fatal("expected error with corrupted block, got nil")
	}

	// Check error message mentions HMAC
	errMsg := err.Error()
	if !strings.Contains(errMsg, "HMAC") {
		t.Errorf("error message should mention HMAC failure, got: %v", errMsg)
	}
}

// Test getSidecarHMACPath
func TestGetSidecarHMACPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "data/file.bin",
			expected: "data/.armor/hmac",
		},
		{
			input:    "file.bin",
			expected: ".armor/hmac",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := getSidecarHMACPath(tt.input)
			// The path should contain .armor/hmac
			if !strings.Contains(result, ".armor/hmac") {
				t.Errorf("expected path to contain .armor/hmac, got: %s", result)
			}
		})
	}
}

// Test getSidecarHMACKey
func TestGetSidecarHMACKey(t *testing.T) {
	key := getSidecarHMACKey("path/to/object.bin")
	if !strings.HasPrefix(key, ".armor/hmac/") {
		t.Errorf("expected key to start with .armor/hmac/, got: %s", key)
	}
}

// Test getInputSource parsing
func TestGetInputSource(t *testing.T) {
	oldInput := inputFlag
	oldWrapped := wrappedDEKFlag
	oldBucket := b2BucketFlag
	defer func() {
		inputFlag = oldInput
		wrappedDEKFlag = oldWrapped
		b2BucketFlag = oldBucket
	}()

	t.Run("B2 URL", func(t *testing.T) {
		inputFlag = "b2://my-bucket/path/to/file"
		wrappedDEKFlag = ""
		b2BucketFlag = ""

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "b2" {
			t.Errorf("expected type b2, got %s", src.Type)
		}
		if src.Bucket != "my-bucket" {
			t.Errorf("expected bucket my-bucket, got %s", src.Bucket)
		}
		if src.Path != "path/to/file" {
			t.Errorf("expected path path/to/file, got %s", src.Path)
		}
	})

	t.Run("local file with wrapped DEK", func(t *testing.T) {
		inputFlag = "/path/to/file.bin"
		wrappedDEKFlag = "dGVzdA==" // "test" in base64
		b2BucketFlag = ""

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "local" {
			t.Errorf("expected type local, got %s", src.Type)
		}
		if src.Path != "/path/to/file.bin" {
			t.Errorf("expected path /path/to/file.bin, got %s", src.Path)
		}
		if src.WrappedDEK == nil || len(src.WrappedDEK) != 4 {
			t.Errorf("wrapped DEK not decoded correctly")
		}
	})

	t.Run("local file without wrapped DEK returns error", func(t *testing.T) {
		inputFlag = "/path/to/file.bin"
		wrappedDEKFlag = ""
		b2BucketFlag = ""

		_, err := getInputSource()
		if err == nil {
			t.Error("expected error for local file without wrapped DEK")
		}
	})

	t.Run("bucket specified separately", func(t *testing.T) {
		inputFlag = "path/to/file"
		wrappedDEKFlag = ""
		b2BucketFlag = "my-bucket"

		src, err := getInputSource()
		if err != nil {
			t.Fatalf("getInputSource: %v", err)
		}

		if src.Type != "b2" {
			t.Errorf("expected type b2, got %s", src.Type)
		}
		if src.Bucket != "my-bucket" {
			t.Errorf("expected bucket my-bucket, got %s", src.Bucket)
		}
	})
}

// Test parseB2URL
func TestParseB2URL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantErr   bool
		wantBucket string
		wantKey   string
	}{
		{
			name:      "valid URL",
			url:       "b2://my-bucket/path/to/file",
			wantErr:   false,
			wantBucket: "my-bucket",
			wantKey:   "path/to/file",
		},
		{
			name:      "URL with no path",
			url:       "b2://my-bucket",
			wantErr:   true,
		},
		{
			name:      "invalid URL format",
			url:       "https://example.com/file",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, err := parseB2URL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseB2URL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if src.Bucket != tt.wantBucket {
					t.Errorf("Bucket = %v, want %v", src.Bucket, tt.wantBucket)
				}
				if src.Path != tt.wantKey {
					t.Errorf("Path = %v, want %v", src.Path, tt.wantKey)
				}
			}
		})
	}
}

// Helper: encode to base64
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// getSidecarHMACHash returns the hash component of the sidecar path for testing
func getSidecarHMACHash(path string) string {
	// Simplified version of what getSidecarHMACPath computes
	key := filepath.Base(path)
	sum := crypto.ComputePlaintextSHA256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// TestMain sets up test flags
func TestMain(m *testing.M) {
	// Clear any flags set during normal init
	flag.Parse()
	os.Exit(m.Run())
}

// Test that we can write to stdout
func TestWriteOutput(t *testing.T) {
	data := []byte("test output data")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputFlag = "" // stdout
	err := writeOutput(data)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("writeOutput to stdout: %v", err)
	}

	// Read back
	var buf bytes.Buffer
	io.Copy(&buf, r)
	if !bytes.Equal(buf.Bytes(), data) {
		t.Errorf("stdout data mismatch: got %q, want %q", buf.Bytes(), data)
	}
}

// Test write to file
func TestWriteOutputFile(t *testing.T) {
	data := []byte("test output to file")
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	outputFlag = outputFile
	err := writeOutput(data)
	if err != nil {
		t.Fatalf("writeOutput to file: %v", err)
	}

	// Read back
	readData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	if !bytes.Equal(readData, data) {
		t.Errorf("file data mismatch: got %q, want %q", readData, data)
	}
}
