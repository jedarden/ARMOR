package crypto

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"testing"
)

func TestEnvelopeHeader(t *testing.T) {
	iv := make([]byte, 16)
	rand.Read(iv)

	plaintext := []byte("Hello, ARMOR!")
	plaintextSHA := ComputePlaintextSHA256(plaintext)

	header, err := NewEnvelopeHeader(iv, int64(len(plaintext)), 65536, plaintextSHA)
	if err != nil {
		t.Fatalf("Failed to create header: %v", err)
	}

	encoded, err := header.Encode()
	if err != nil {
		t.Fatalf("Failed to encode header: %v", err)
	}

	if len(encoded) != HeaderSize {
		t.Errorf("Expected header size %d, got %d", HeaderSize, len(encoded))
	}

	decoded, err := DecodeHeader(encoded)
	if err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	if string(decoded.Magic[:]) != Magic {
		t.Errorf("Expected magic %s, got %s", Magic, decoded.Magic[:])
	}

	if decoded.Version != Version1 {
		t.Errorf("Expected version %d, got %d", Version1, decoded.Version)
	}

	if decoded.BlockSize() != 65536 {
		t.Errorf("Expected block size 65536, got %d", decoded.BlockSize())
	}

	if decoded.PlaintextSize != uint64(len(plaintext)) {
		t.Errorf("Expected plaintext size %d, got %d", len(plaintext), decoded.PlaintextSize)
	}
}

func TestKeyWrapUnwrap(t *testing.T) {
	// Generate MEK and DEK
	mek := make([]byte, 32)
	rand.Read(mek)

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("Failed to generate DEK: %v", err)
	}

	// Wrap DEK
	wrapped, err := WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	if len(wrapped) != 40 {
		t.Errorf("Expected wrapped DEK size 40, got %d", len(wrapped))
	}

	// Unwrap DEK
	unwrapped, err := UnwrapDEK(mek, wrapped)
	if err != nil {
		t.Fatalf("Failed to unwrap DEK: %v", err)
	}

	if !bytes.Equal(dek, unwrapped) {
		t.Errorf("Unwrapped DEK doesn't match original")
	}
}

func TestKeyWrapUnwrapWrongMEK(t *testing.T) {
	mek1 := make([]byte, 32)
	mek2 := make([]byte, 32)
	rand.Read(mek1)
	rand.Read(mek2)

	dek, _ := GenerateDEK()

	wrapped, err := WrapDEK(mek1, dek)
	if err != nil {
		t.Fatalf("Failed to wrap DEK: %v", err)
	}

	// Try to unwrap with wrong MEK
	_, err = UnwrapDEK(mek2, wrapped)
	if err == nil {
		t.Error("Expected error when unwrapping with wrong MEK")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	dek, _ := GenerateDEK()
	iv, _ := GenerateIV()

	plaintext := make([]byte, 200000) // ~3 blocks
	rand.Read(plaintext)

	encryptor, err := NewEncryptor(dek, iv, 65536)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	decryptor, err := NewDecryptor(dek, iv, 65536)
	if err != nil {
		t.Fatalf("Failed to create decryptor: %v", err)
	}

	decrypted, err := decryptor.Decrypt(encrypted, hmacTable)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Decrypted data doesn't match original")
	}
}

func TestEncryptDecryptEmptyData(t *testing.T) {
	dek, _ := GenerateDEK()
	iv, _ := GenerateIV()

	plaintext := []byte{}

	encryptor, _ := NewEncryptor(dek, iv, 65536)
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Failed to encrypt empty data: %v", err)
	}

	decryptor, _ := NewDecryptor(dek, iv, 65536)
	decrypted, err := decryptor.Decrypt(encrypted, hmacTable)
	if err != nil {
		t.Fatalf("Failed to decrypt empty data: %v", err)
	}

	if len(decrypted) != 0 {
		t.Errorf("Expected empty decrypted data, got %d bytes", len(decrypted))
	}
}

func TestHMACKeyDerivation(t *testing.T) {
	dek := make([]byte, 32)
	rand.Read(dek)

	hmacKey1 := DeriveHMACKey(dek)
	hmacKey2 := DeriveHMACKey(dek)

	if !bytes.Equal(hmacKey1, hmacKey2) {
		t.Error("Same DEK should produce same HMAC key")
	}

	// Different DEK should produce different HMAC key
	dek2 := make([]byte, 32)
	rand.Read(dek2)
	hmacKey3 := DeriveHMACKey(dek2)

	if bytes.Equal(hmacKey1, hmacKey3) {
		t.Error("Different DEK should produce different HMAC key")
	}
}

func TestRangeTranslation(t *testing.T) {
	blockSize := 65536
	plaintextSize := int64(200000) // ~3 blocks
	headerSize := 64

	// Request bytes 100000-150000 (should span blocks 1-2)
	translation, err := TranslateRange(100000, 150000, plaintextSize, blockSize, headerSize)
	if err != nil {
		t.Fatalf("Failed to translate range: %v", err)
	}

	if translation.BlockStart != 1 {
		t.Errorf("Expected block start 1, got %d", translation.BlockStart)
	}

	if translation.BlockEnd != 2 {
		t.Errorf("Expected block end 2, got %d", translation.BlockEnd)
	}

	// Verify data offset
	expectedDataOffset := int64(headerSize) + int64(1*blockSize)
	if translation.DataOffset != expectedDataOffset {
		t.Errorf("Expected data offset %d, got %d", expectedDataOffset, translation.DataOffset)
	}
}

func TestHMACVerificationFailure(t *testing.T) {
	dek, _ := GenerateDEK()
	iv, _ := GenerateIV()

	plaintext := []byte("test data for HMAC verification")

	encryptor, _ := NewEncryptor(dek, iv, 65536)
	encrypted, hmacTable, _ := encryptor.Encrypt(plaintext)

	// Tamper with encrypted data
	if len(encrypted) > 0 {
		encrypted[0] ^= 0xFF
	}

	decryptor, _ := NewDecryptor(dek, iv, 65536)
	_, err := decryptor.Decrypt(encrypted, hmacTable)
	if err == nil {
		t.Error("Expected HMAC verification to fail for tampered data")
	}
}

func TestComputeBlockCount(t *testing.T) {
	tests := []struct {
		plaintextSize int64
		blockSize     int
		expected      uint32
	}{
		{0, 65536, 0},
		{1, 65536, 1},
		{65535, 65536, 1},
		{65536, 65536, 1},
		{65537, 65536, 2},
		{131072, 65536, 2},
		{131073, 65536, 3},
	}

	for _, tt := range tests {
		result := ComputeBlockCount(tt.plaintextSize, tt.blockSize)
		if result != tt.expected {
			t.Errorf("ComputeBlockCount(%d, %d) = %d, expected %d",
				tt.plaintextSize, tt.blockSize, result, tt.expected)
		}
	}
}

func TestFullObjectSize(t *testing.T) {
	plaintextSize := int64(1000000) // 1 MB
	blockSize := 65536

	blockCount := ComputeBlockCount(plaintextSize, blockSize)
	expected := int64(HeaderSize) + plaintextSize + int64(blockCount*HMACSize)

	result := FullObjectSize(plaintextSize, blockSize)
	if result != expected {
		t.Errorf("FullObjectSize(%d, %d) = %d, expected %d",
			plaintextSize, blockSize, result, expected)
	}
}

// Property-based test: encrypt then decrypt always equals original
func TestEncryptDecryptProperty(t *testing.T) {
	for i := 0; i < 100; i++ {
		dek, _ := GenerateDEK()
		iv, _ := GenerateIV()

		// Random size between 0 and 1MB
		size := i * 10000
		plaintext := make([]byte, size)
		rand.Read(plaintext)

		encryptor, _ := NewEncryptor(dek, iv, 65536)
		encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Iteration %d: encrypt failed: %v", i, err)
		}

		decryptor, _ := NewDecryptor(dek, iv, 65536)
		decrypted, err := decryptor.Decrypt(encrypted, hmacTable)
		if err != nil {
			t.Fatalf("Iteration %d: decrypt failed: %v", i, err)
		}

		if !bytes.Equal(plaintext, decrypted) {
			t.Errorf("Iteration %d: decrypted != plaintext", i)
		}
	}
}

func TestHexEncoding(t *testing.T) {
	data := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef}
	hexStr := hex.EncodeToString(data)

	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		t.Fatalf("Failed to decode hex: %v", err)
	}

	if !bytes.Equal(data, decoded) {
		t.Error("Hex encode/decode roundtrip failed")
	}
}
