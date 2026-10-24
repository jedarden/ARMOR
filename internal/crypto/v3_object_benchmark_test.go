package crypto

import (
	"bytes"
	"testing"
)

// Per-object v3 benchmarks. BenchmarkEncryptBlockV3/BenchmarkDecryptBlockV3 in
// v3_benchmark_test.go measure a single one-shot block call through the static
// entry points, which must build the cipher and derive the HMAC key every
// time. Production does not use them that way: one object is many blocks under
// the same DEK/IV, so the per-object state (AES cipher, HKDF-derived HMAC key,
// reusable HMAC state) can be derived once and amortized across the block
// loop. These benchmarks measure that shape — a 4 MiB object in 64 KiB
// blocks — so before/after runs show what moving the per-block rebuilds into a
// per-object context buys:
//
//	go test ./internal/crypto -bench 'Benchmark(Encrypt|Decrypt)V3' -benchmem
//
// BenchmarkCompressBlockZstd/BenchmarkDecompressBlockZstd pin the codec
// construction amortization for the compression path the block loop shares.

const benchV3ObjectSize = 4 << 20

func BenchmarkEncryptV3(b *testing.B) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(benchV3ObjectSize)
	const blockSize = 64 * 1024
	encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version3)
	if err != nil {
		b.Fatalf("NewEncryptorWithVersion: %v", err)
	}
	b.SetBytes(benchV3ObjectSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := encryptor.EncryptV3(plaintext, false)
		if err != nil {
			b.Fatalf("EncryptV3: %v", err)
		}
	}
}

func BenchmarkDecryptV3(b *testing.B) {
	dek, iv := benchV3KeyMaterial()
	plaintext := benchV3Plaintext(benchV3ObjectSize)
	const blockSize = 64 * 1024
	encryptor, err := NewEncryptorWithVersion(dek, iv, blockSize, Version3)
	if err != nil {
		b.Fatalf("NewEncryptorWithVersion: %v", err)
	}
	encrypted, blockTable, err := encryptor.EncryptV3(plaintext, false)
	if err != nil {
		b.Fatalf("EncryptV3: %v", err)
	}
	decryptor, err := NewDecryptorWithVersion(dek, iv, blockSize, Version3)
	if err != nil {
		b.Fatalf("NewDecryptorWithVersion: %v", err)
	}
	b.SetBytes(benchV3ObjectSize)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := decryptor.DecryptV3(encrypted, 0, blockTable)
		if err != nil {
			b.Fatalf("DecryptV3: %v", err)
		}
		if !bytes.Equal(got, plaintext) {
			b.Fatal("DecryptV3 roundtrip mismatch")
		}
	}
}

// benchCompressible repeats a short low-entropy pattern so the block actually
// compresses and the benchmark exercises the codec, not the pass-through.
func benchCompressible(n int) []byte {
	pattern := []byte("armor-v3-object-benchmark payload line\n")
	out := make([]byte, 0, n+len(pattern))
	for len(out) < n {
		out = append(out, pattern...)
	}
	return out[:n]
}

func BenchmarkCompressBlockZstd(b *testing.B) {
	block := benchCompressible(64 * 1024)
	b.SetBytes(int64(len(block)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressed, wasCompressed, _, err := CompressBlock(block)
		if err != nil {
			b.Fatalf("CompressBlock: %v", err)
		}
		if !wasCompressed {
			b.Fatal("benchmark block unexpectedly incompressible")
		}
		if len(compressed) >= len(block) {
			b.Fatal("CompressBlock reported compression without shrinking the block")
		}
	}
}

func BenchmarkDecompressBlockZstd(b *testing.B) {
	block := benchCompressible(64 * 1024)
	compressed, wasCompressed, _, err := CompressBlock(block)
	if err != nil {
		b.Fatalf("CompressBlock: %v", err)
	}
	if !wasCompressed {
		b.Fatal("benchmark block unexpectedly incompressible")
	}
	b.SetBytes(int64(len(block)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got, err := DecompressBlock(compressed, true)
		if err != nil {
			b.Fatalf("DecompressBlock: %v", err)
		}
		if !bytes.Equal(got, block) {
			b.Fatal("DecompressBlock roundtrip mismatch")
		}
	}
}
