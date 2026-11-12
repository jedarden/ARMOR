package crypto

import (
	"bytes"
	"testing"
)

func TestDecodeV3CompressedBlockTableWhenTrailerExceedsCiphertext(t *testing.T) {
	const (
		blockSize     = 64 * 1024
		plaintextSize = 1 * 1024 * 1024
		maxBlockCount = plaintextSize / blockSize
	)

	pattern := []byte("ARMOR compressed v3 trailer regression\n")
	plaintext := bytes.Repeat(pattern, plaintextSize/len(pattern)+1)[:plaintextSize]
	compressed, compressedOK, _, err := Compress(plaintext)
	if err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if !compressedOK {
		t.Fatal("regression fixture did not compress")
	}

	dek := bytes.Repeat([]byte{0x11}, 32)
	iv := bytes.Repeat([]byte{0x22}, 16)
	encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version3)
	if err != nil {
		t.Fatalf("NewEncryptorWithVersion: %v", err)
	}
	encrypted, expectedTable, err := encryptor.EncryptV3(compressed, false)
	if err != nil {
		t.Fatalf("EncryptV3: %v", err)
	}
	trailer, err := expectedTable.Encode()
	if err != nil {
		t.Fatalf("Encode trailer: %v", err)
	}
	header, err := NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, ComputePlaintextSHA256(plaintext), Version3)
	if err != nil {
		t.Fatalf("NewEnvelopeHeaderWithVersion: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("Encode header: %v", err)
	}
	envelope := append(append(append([]byte{}, headerBytes...), encrypted...), trailer...)

	if len(encrypted) >= maxBlockCount*BlockTableEntrySize {
		t.Fatalf("fixture does not exercise tiny ciphertext: ciphertext=%d, header-derived trailer=%d", len(encrypted), maxBlockCount*BlockTableEntrySize)
	}
	got, err := DecodeV3CompressedBlockTable(envelope, blockSize, maxBlockCount)
	if err != nil {
		t.Fatalf("DecodeV3CompressedBlockTable: %v", err)
	}
	if got.EntryCount() != expectedTable.EntryCount() {
		t.Fatalf("entry count = %d, want %d", got.EntryCount(), expectedTable.EntryCount())
	}
	if got.TotalCiphertextLength64() != int64(len(encrypted)) {
		t.Fatalf("ciphertext length = %d, want %d", got.TotalCiphertextLength64(), len(encrypted))
	}
	decryptor, err := NewDecryptorWithVersion(dek, iv, blockSize, Version3)
	if err != nil {
		t.Fatalf("NewDecryptorWithVersion: %v", err)
	}
	decrypted, err := decryptor.DecryptV3(encrypted, 0, got)
	if err != nil {
		t.Fatalf("DecryptV3: %v", err)
	}
	decompressed, err := DecompressData(decrypted)
	if err != nil {
		t.Fatalf("DecompressData: %v", err)
	}
	if !bytes.Equal(decompressed, plaintext) {
		t.Fatalf("decompressed plaintext mismatch: got %d bytes, want %d", len(decompressed), len(plaintext))
	}
}

func TestDecodeV3CompressedBlockTableRejectsMalformedTrailer(t *testing.T) {
	data := append(make([]byte, HeaderSize), bytes.Repeat([]byte{0xA5}, BlockTableEntrySize)...)
	if _, err := DecodeV3CompressedBlockTable(data, 64*1024, 16); err == nil {
		t.Fatal("malformed compressed v3 trailer was accepted")
	}
}
