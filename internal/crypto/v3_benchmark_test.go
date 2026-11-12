package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

// Benchmarks for the v3 block hot path. These pin the one-CTR-stream-per-
// ARMOR-block implementation against the historical one-stream-per-16-bytes
// behaviour (8220 allocs/op for a 64 KiB block): run
//
//	go test ./internal/crypto -bench 'Benchmark(Encrypt|Decrypt)BlockV3' -benchmem
//
// and compare allocs/op and B/op across the change. DEK/IV are fixed so runs
// are comparable; b.SetBytes makes the benchmark runner report throughput.

func benchV3KeyMaterial() (dek, iv []byte) {
	dek = make([]byte, 32)
	iv = make([]byte, 16)
	for i := range dek {
		dek[i] = byte(i + 1)
	}
	for i := range iv {
		iv[i] = byte(i + 10)
	}
	return dek, iv
}

func benchV3Plaintext(n int) []byte {
	// Deterministic pseudo-random-looking plaintext: incompressible enough
	// that the benchmark measures the cipher, not pattern-dependent branches.
	pt := make([]byte, n)
	x := uint32(0x9e3779b9)
	for i := range pt {
		x = x*1664525 + 1013904223
		pt[i] = byte(x >> 24)
	}
	return pt
}

func benchmarkEncryptBlockV3(b *testing.B, n int) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(n)
	blockSize := n
	if blockSize > V3MaxBlockSize {
		blockSize = V3MaxBlockSize
	}
	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ciphertext, mac, err := EncryptBlockV3(dek, iv, 0, 0, plaintext, blockSize)
		if err != nil {
			b.Fatalf("EncryptBlockV3: %v", err)
		}
		_ = ciphertext
		_ = mac
	}
}

func benchmarkDecryptBlockV3(b *testing.B, n int) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(n)
	blockSize := n
	if blockSize > V3MaxBlockSize {
		blockSize = V3MaxBlockSize
	}
	ciphertext, mac, err := EncryptBlockV3(dek, iv, 0, 0, plaintext, blockSize)
	if err != nil {
		b.Fatalf("EncryptBlockV3: %v", err)
	}
	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := DecryptBlockV3(dek, iv, 0, 0, ciphertext, mac, blockSize)
		if err != nil {
			b.Fatalf("DecryptBlockV3: %v", err)
		}
		if len(got) != n {
			b.Fatalf("length mismatch: %d != %d", len(got), n)
		}
	}
}

// benchmarkDecryptBlockV3Cryptor is the per-object shape the v3 read paths
// run after armor-61a35059: one cryptor holds the AES cipher, the derived
// HMAC key, and a Reset-reusable HMAC state for the whole block loop, and
// the plaintext destination is a caller-owned buffer reused across blocks.
// Compare against benchmarkDecryptBlockV3 (the one-shot static entry point
// the read paths used before): same DEK/IV, same block, so the delta is
// exactly the per-block key/cipher rebuilds, the second HMAC pass, and the
// fresh output buffer the one-shot path pays.
func benchmarkDecryptBlockV3Cryptor(b *testing.B, n int) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(n)
	blockSize := n
	if blockSize > V3MaxBlockSize {
		blockSize = V3MaxBlockSize
	}
	ciphertext, mac, err := EncryptBlockV3(dek, iv, 0, 0, plaintext, blockSize)
	if err != nil {
		b.Fatalf("EncryptBlockV3: %v", err)
	}
	cryptor, err := NewV3BlockCryptor(dek, iv, blockSize)
	if err != nil {
		b.Fatalf("NewV3BlockCryptor: %v", err)
	}
	dst := make([]byte, n)
	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := cryptor.DecryptBlockInto(dst, 0, 0, ciphertext, mac); err != nil {
			b.Fatalf("DecryptBlockInto: %v", err)
		}
		if len(dst) != n {
			b.Fatalf("length mismatch: %d != %d", len(dst), n)
		}
	}
}

// BenchmarkV3KeystreamReference measures the historical keystream
// construction — one cipher.NewCTR per 16 bytes — so the amplification it
// caused stays visible next to the optimized numbers without resurrecting
// the old code path anywhere in production. It isolates the construction
// itself (the amplification source), so it excludes the ciphertext
// allocation and HMAC work the full historical path also paid: its 8192
// allocs/op (2 per 16-byte AES block) at 64 KiB is a lower bound on the
// 8220 allocs/op the full historical encrypt path measured on 2026-09-13.
// A first pass is cross-checked byte-for-byte against EncryptBlockV3
// output so the reference cannot silently drift from the production
// keystream the characterization oracle pins.
func benchmarkV3KeystreamReference(b *testing.B, n int) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(n)
	blockSize := n
	if blockSize > V3MaxBlockSize {
		blockSize = V3MaxBlockSize
	}
	block, err := aes.NewCipher(dek)
	if err != nil {
		b.Fatalf("aes.NewCipher: %v", err)
	}
	wantCiphertext, _, err := EncryptBlockV3(dek, iv, 0, 0, plaintext, blockSize)
	if err != nil {
		b.Fatalf("EncryptBlockV3: %v", err)
	}
	dst := make([]byte, len(plaintext))
	runKeystreamReference := func() {
		numAESBlocks := (len(plaintext) + 15) / 16
		for aesBlockIdx := 0; aesBlockIdx < numAESBlocks; aesBlockIdx++ {
			counter := make([]byte, 16)
			copy(counter[0:8], iv[0:8])
			binary.BigEndian.PutUint16(counter[8:10], 0)
			binary.BigEndian.PutUint32(counter[10:14], 0)
			binary.BigEndian.PutUint16(counter[14:16], uint16(aesBlockIdx))
			stream := cipher.NewCTR(block, counter)
			start := aesBlockIdx * 16
			end := start + 16
			if end > len(plaintext) {
				end = len(plaintext)
			}
			stream.XORKeyStream(dst[start:end], plaintext[start:end])
		}
	}
	runKeystreamReference()
	if !bytes.Equal(dst, wantCiphertext) {
		b.Fatalf("reference keystream diverges from EncryptBlockV3 output")
	}
	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		runKeystreamReference()
	}
}

// benchmarkDecryptBlockV3HandlerShape mirrors the per-block work the v3
// full-GET loop paid before armor-61a35059: a manual HMAC pass over the
// block (hmac.New, prefix writes — handlers.go had it inline) followed by a
// one-shot DecryptBlockV3 that verified the same HMAC a second time and
// rebuilt the cipher and HMAC key. The read call sites run neither of those
// anymore; this benchmark keeps the removed cost visible next to the
// cryptor numbers rather than resurrecting the shape anywhere in production.
func benchmarkDecryptBlockV3HandlerShape(b *testing.B, n int) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(n)
	blockSize := n
	if blockSize > V3MaxBlockSize {
		blockSize = V3MaxBlockSize
	}
	ciphertext, mac, err := EncryptBlockV3(dek, iv, 0, 0, plaintext, blockSize)
	if err != nil {
		b.Fatalf("EncryptBlockV3: %v", err)
	}
	hmacKey, err := DeriveHMACKey(dek)
	if err != nil {
		b.Fatalf("DeriveHMACKey: %v", err)
	}
	b.SetBytes(int64(n))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m := hmac.New(sha256.New, hmacKey)
		var partBytes [2]byte
		var blockBytes [4]byte
		binary.BigEndian.PutUint16(partBytes[:], 0) // part=0 for single-PUT
		m.Write(partBytes[:])
		binary.BigEndian.PutUint32(blockBytes[:], 0)
		m.Write(blockBytes[:])
		m.Write(ciphertext)
		if !hmac.Equal(m.Sum(nil), mac) {
			b.Fatal("manual HMAC pass mismatch")
		}
		got, err := DecryptBlockV3(dek, iv, 0, 0, ciphertext, mac, blockSize)
		if err != nil {
			b.Fatalf("DecryptBlockV3: %v", err)
		}
		if len(got) != n {
			b.Fatalf("length mismatch: %d != %d", len(got), n)
		}
	}
}

func BenchmarkEncryptBlockV3_64KiB(b *testing.B) { benchmarkEncryptBlockV3(b, 64*1024) }
func BenchmarkDecryptBlockV3_64KiB(b *testing.B) { benchmarkDecryptBlockV3(b, 64*1024) }
func BenchmarkEncryptBlockV3_1MiB(b *testing.B)  { benchmarkEncryptBlockV3(b, V3MaxBlockSize) }
func BenchmarkDecryptBlockV3_1MiB(b *testing.B)  { benchmarkDecryptBlockV3(b, V3MaxBlockSize) }

func BenchmarkDecryptBlockV3Cryptor_64KiB(b *testing.B) { benchmarkDecryptBlockV3Cryptor(b, 64*1024) }
func BenchmarkDecryptBlockV3Cryptor_1MiB(b *testing.B) {
	benchmarkDecryptBlockV3Cryptor(b, V3MaxBlockSize)
}

func BenchmarkDecryptBlockV3HandlerShape_64KiB(b *testing.B) {
	benchmarkDecryptBlockV3HandlerShape(b, 64*1024)
}

func BenchmarkV3KeystreamReference_64KiB(b *testing.B) { benchmarkV3KeystreamReference(b, 64*1024) }
func BenchmarkV3KeystreamReference_1MiB(b *testing.B) {
	benchmarkV3KeystreamReference(b, V3MaxBlockSize)
}
