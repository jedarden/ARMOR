// Multipart layout-matrix verification for armor-decrypt (bead armor-9ceb53f3).
//
// ADR-005 defines the valid multipart upload patterns (Patterns 1-4 plus edge
// cases). Once CompleteMultipartUpload assembles them, every pattern stores the
// same shape — headerless raw ciphertext + JSON HMAC sidecar + absolute block
// indices — so what distinguishes the layouts at read time is purely geometry:
// whether the total is block-aligned, whether it ends in a partial block, and
// how many blocks it spans. This matrix round-trips every geometry through both
// armor-decrypt input paths (B2 via the fake backend, local files via
// -sidecar/-iv), so the DR story holds for the full range of encrypted objects,
// not just the single shape the earlier tests happened to cover.
//
// The >256-block cases sit deliberately past the historical block-256 HMAC
// failure point that broke multipart DR four times running (armor-3761e1b7,
// armor-9243989f) — the production object verified through the live fleet
// (~864 blocks) was exercised server-side, never through this CLI.
package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/jedarden/armor/internal/backend"
)

const matrixBlockSize = 65536

// multipartLayoutCases enumerates the stored-object geometries ADR-005's valid
// upload patterns produce. Sizes are scaled down from the ADR's MiB-scale part
// examples (a stored object has no part boundaries — only total geometry) but
// preserve each pattern's alignment properties exactly.
var multipartLayoutCases = []struct {
	name string
	size int64
}{
	// Pattern 1 / zero-byte final part: every part aligned, total exactly
	// block-aligned, spanning several conceptual parts.
	{"pattern1_aligned_exact_total", 5 * matrixBlockSize},
	// Pattern 2: aligned parts with a short, non-aligned final part.
	{"pattern2_aligned_short_final", 4*matrixBlockSize + 52},
	// Pattern 3 (barman): a single non-aligned part, several blocks plus remainder.
	{"pattern3_single_non_aligned_part", 3*matrixBlockSize + 40000},
	// Pattern 3 edge: a single part smaller than one block — one partial block only.
	{"pattern3_sub_block_single_part", 40000},
	// Pattern 4: a single part that happens to be block-aligned.
	{"pattern4_single_aligned_part", 2 * matrixBlockSize},
	// Edge: total exactly one full block.
	{"edge_exactly_one_block", matrixBlockSize},
	// Edge: a lone zero-byte part — zero blocks, empty sidecar.
	{"edge_empty_object", 0},
	// Historical block-256 threshold (armor-3761e1b7 / armor-9243989f): past 256
	// blocks with a partial tail, and past it again on an exact-aligned total.
	{"block256_plus_partial_tail", 257*matrixBlockSize + 7},
	{"block256_plus_exact_aligned", 258 * matrixBlockSize},
}

// matrixPlaintext returns deterministic plaintext of the given size.
func matrixPlaintext(size int64) []byte {
	p := make([]byte, size)
	for i := range p {
		p[i] = byte(i)
	}
	return p
}

// TestDecryptB2MultipartLayoutMatrix runs every ADR-005 layout through the B2
// path — the actual break-glass DR path, exercising the full decryptB2
// dispatch: metadata marker, offset-0 ciphertext read, sidecar fetch, absolute
// block indices.
func TestDecryptB2MultipartLayoutMatrix(t *testing.T) {
	mek := makeMEK(t)

	for _, tc := range multipartLayoutCases {
		t.Run(tc.name, func(t *testing.T) {
			plaintext := matrixPlaintext(tc.size)
			ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, matrixBlockSize, plaintext)

			const key = "replica/layout-matrix.snapshot"
			objects := map[string]*fakeB2Object{}
			storeMultipartObjects(objects, key, ciphertext, sidecarJSON, wrappedDEK, iv, matrixBlockSize, int64(len(plaintext)))

			src := &inputSource{Type: "b2", Bucket: "bucket", Path: key}

			var decrypted []byte
			withFakeBackend(t, objects, func() {
				var err error
				decrypted, err = decryptB2(context.Background(), src, mek, "")
				if err != nil {
					t.Fatalf("decryptB2 multipart layout %q: %v", tc.name, err)
				}
			})

			if !bytes.Equal(decrypted, plaintext) {
				t.Fatalf("layout %q: plaintext mismatch: got %d bytes, want %d", tc.name, len(decrypted), len(plaintext))
			}
		})
	}
}

// TestDecryptLocalMultipartLayoutMatrix runs the same layouts through the local
// file path (-sidecar + -iv), the recovery mode for objects already downloaded
// or copied out of B2.
func TestDecryptLocalMultipartLayoutMatrix(t *testing.T) {
	mek := makeMEK(t)

	for _, tc := range multipartLayoutCases {
		t.Run(tc.name, func(t *testing.T) {
			plaintext := matrixPlaintext(tc.size)
			ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, matrixBlockSize, plaintext)

			tmpDir := t.TempDir()
			ctFile := filepath.Join(tmpDir, "object.bin")
			sidecarFile := filepath.Join(tmpDir, "object.hmac.json")
			if err := os.WriteFile(ctFile, ciphertext, 0644); err != nil {
				t.Fatalf("write ciphertext: %v", err)
			}
			if err := os.WriteFile(sidecarFile, sidecarJSON, 0644); err != nil {
				t.Fatalf("write sidecar: %v", err)
			}

			sidecarFlag = sidecarFile
			ivFlag = hex.EncodeToString(iv)
			defer func() { sidecarFlag = ""; ivFlag = "" }()

			src := &inputSource{Type: "local", Path: ctFile, WrappedDEK: wrappedDEK}

			decrypted, err := decryptLocal(context.Background(), src, mek)
			if err != nil {
				t.Fatalf("decryptLocal multipart layout %q: %v", tc.name, err)
			}
			if !bytes.Equal(decrypted, plaintext) {
				t.Fatalf("layout %q: plaintext mismatch: got %d bytes, want %d", tc.name, len(decrypted), len(plaintext))
			}
		})
	}
}

// TestDecryptB2MultipartLayoutMatrixCorrupted spot-checks that corruption
// detection survives on both sides of the historical block-256 boundary: a
// flipped byte inside block 0 and one deep inside the object (block 257+),
// where the pre-fix read path failed with "block 256: HMAC verification failed".
func TestDecryptB2MultipartLayoutMatrixCorrupted(t *testing.T) {
	mek := makeMEK(t)
	plaintext := matrixPlaintext(258*matrixBlockSize + 100)

	for _, tc := range []struct {
		name  string
		index int64
	}{
		{"corrupt_block_0", 42},
		{"corrupt_past_block_256", 256*matrixBlockSize + 10},
		{"corrupt_final_partial_block", 258*matrixBlockSize + 50},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, mek, matrixBlockSize, plaintext)
			ciphertext[tc.index] ^= 0xff

			const key = "replica/layout-matrix.snapshot"
			objects := map[string]*fakeB2Object{}
			storeMultipartObjects(objects, key, ciphertext, sidecarJSON, wrappedDEK, iv, matrixBlockSize, int64(len(plaintext)))

			src := &inputSource{Type: "b2", Bucket: "bucket", Path: key}

			withFakeBackend(t, objects, func() {
				_, err := decryptB2(context.Background(), src, mek, "")
				if err == nil {
					t.Fatalf("%s: expected HMAC error, got nil", tc.name)
				}
			})
		})
	}
}

// Verify the fixture builder's metadata matches what the server writes for a
// multipart-completed object: the placeholder plaintext SHA (ADR-003 gap
// bf-1v2ehf), never a true digest a reader could mistake for verifiable.
func TestMultipartFixtureMetadataMatchesServer(t *testing.T) {
	objects := map[string]*fakeB2Object{}
	ciphertext, sidecarJSON, wrappedDEK, iv := buildMultipartFixture(t, makeMEK(t), matrixBlockSize, []byte("x"))
	storeMultipartObjects(objects, "k", ciphertext, sidecarJSON, wrappedDEK, iv, matrixBlockSize, 1)

	meta := objects["k"].metadata
	if meta["x-amz-meta-armor-multipart"] != "true" {
		t.Errorf("fixture missing multipart marker: %v", meta)
	}

	// The sidecar key must be exactly where MultipartStateManager looks.
	if _, ok := objects[sidecarKeyFor("k")]; !ok {
		t.Error("fixture sidecar not stored under .armor/hmac/<sha256(key)>")
	}

	// The metadata must round-trip through the parser decryptB2 uses.
	parsed, ok := backend.ParseARMORMetadata(meta)
	if !ok {
		t.Fatal("fixture metadata does not parse as ARMOR metadata")
	}
	if len(parsed.IV) != 16 {
		t.Errorf("IV did not round-trip: got %d bytes", len(parsed.IV))
	}
}
