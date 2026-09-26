package handlers_test

// End-to-end pins for the README security-model guarantee: "data is encrypted
// before it leaves ARMOR; B2 only ever stores ciphertext (guaranteed for
// envelope v2/v3 objects; legacy v1 objects must be migrated first)".
//
// Each test drives the real HTTP surface (PUT; multipart initiate/part/
// complete) against a recording backend and then inspects every byte sequence
// the backend persists — object bodies, in-flight multipart part bodies, and
// metadata values — for the uploaded plaintext. The v1 exclusion (ADR-005
// keystream reuse, removed by migration) is pinned on stored bytes in
// internal/server/format_migration_ciphertext_only_test.go.

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// ciphertextOnlyPlaintext builds deterministic pseudorandom plaintext.
// Pseudorandom content keeps the scan sharp: patterned plaintext could
// legitimately reappear inside envelope structure by construction, which
// would make a substring scan noise-prone.
func ciphertextOnlyPlaintext(seed int64, size int) []byte {
	buf := make([]byte, size)
	rand.New(rand.NewSource(seed)).Read(buf)
	return buf
}

// plaintextWindows returns the whole plaintext plus fixed 32-byte windows
// spanning block boundaries and part interiors, so a partial leak (a stored
// field echoing a slice of the payload) is caught, not just a verbatim
// whole-object copy.
func plaintextWindows(plaintext []byte) [][]byte {
	windows := [][]byte{plaintext}
	if len(plaintext) <= 32 {
		return windows
	}
	offsets := []int{
		0,
		len(plaintext) / 3,
		len(plaintext) / 2,
		65536 - 16, // straddles one 64 KB block boundary
		65536,
		2 * 65536,
		len(plaintext) - 32,
	}
	seen := make(map[int]bool, len(offsets))
	for _, off := range offsets {
		if off < 0 || off+32 > len(plaintext) || seen[off] {
			continue
		}
		seen[off] = true
		windows = append(windows, plaintext[off:off+32])
	}
	return windows
}

// findPlaintextLeaks reports every place the backend currently persists the
// plaintext (whole, as a 32-byte window, or base64-encoded): object bodies,
// metadata values, and in-flight multipart part bodies. An empty result means
// the backend holds ciphertext plus key/envelope metadata only.
func findPlaintextLeaks(rb *recordingBackend, plaintext []byte) []string {
	var leaks []string
	base64Plaintext := base64.StdEncoding.EncodeToString(plaintext)

	scan := func(location string, data []byte) {
		for _, window := range plaintextWindows(plaintext) {
			if at := bytes.Index(data, window); at >= 0 {
				leaks = append(leaks, fmt.Sprintf("%s: plaintext bytes found at stored offset %d", location, at))
			}
		}
		if bytes.Contains(data, []byte(base64Plaintext)) {
			leaks = append(leaks, location+": base64(plaintext) found")
		}
	}

	rb.mu.Lock()
	for key, data := range rb.objects {
		scan("object "+key, data)
	}
	for key, meta := range rb.meta {
		for field, value := range meta {
			scan(fmt.Sprintf("metadata %s [%s]", key, field), []byte(value))
		}
	}
	rb.mu.Unlock()

	rb.rmu.Lock()
	for uploadID, parts := range rb.uploads {
		for part, data := range parts {
			scan(fmt.Sprintf("in-flight part %d of upload %s", part, uploadID), data)
		}
	}
	rb.rmu.Unlock()

	return leaks
}

func assertNoPlaintextStored(t *testing.T, rb *recordingBackend, plaintext []byte, stage string) {
	t.Helper()
	if leaks := findPlaintextLeaks(rb, plaintext); len(leaks) > 0 {
		t.Fatalf("%s: backend storage is not ciphertext-only:\n\t%s", stage, strings.Join(leaks, "\n\t"))
	}
}

// TestCiphertextOnlyStorage_SinglePUT verifies the guarantee for whole-object
// PUTs in both guaranteed envelope formats: after the handler stores the
// object, neither the stored envelope, nor any metadata value, nor any other
// object the write produced contains the plaintext.
func TestCiphertextOnlyStorage_SinglePUT(t *testing.T) {
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			cfg, rb, h := recordingTestSetup(t)
			cfg.FormatWriteVersion = version

			bucket, key := "test-bucket", "ciphertext-only/single.bin"
			plaintext := ciphertextOnlyPlaintext(int64(9000+version), 2*65536+1000)

			put := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s", bucket, key), bytes.NewReader(plaintext))
			put.Header.Set("Content-Type", "application/octet-stream")
			w := httptest.NewRecorder()
			h.HandleRoot(w, put)
			if w.Code != http.StatusOK {
				t.Fatalf("PUT failed: status %d: %s", w.Code, w.Body.String())
			}

			// The scan must have a real ARMOR envelope in front of it; pin the
			// stored format and the presence of key material before trusting a
			// clean scan.
			rb.mu.Lock()
			stored, ok := rb.objects[bucket+"/"+key]
			meta := rb.meta[bucket+"/"+key]
			rb.mu.Unlock()
			if !ok {
				t.Fatal("object missing from backend after PUT")
			}
			if len(stored) <= len(plaintext) {
				t.Fatalf("stored envelope is %d bytes for %d bytes of plaintext; not an ARMOR envelope (header + HMAC table expected)", len(stored), len(plaintext))
			}
			if meta["x-amz-meta-armor-version"] != strconv.Itoa(version) {
				t.Fatalf("stored envelope version = %q, want %d; scan inspected the wrong format", meta["x-amz-meta-armor-version"], version)
			}
			if meta["x-amz-meta-armor-wrapped-dek"] == "" {
				t.Fatal("stored object carries no wrapped DEK; scan would not be inspecting an ARMOR-encrypted object")
			}

			// The stored bytes must still be the object: a full GET round-trips.
			get := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
			gw := httptest.NewRecorder()
			h.HandleRoot(gw, get)
			if gw.Code != http.StatusOK {
				t.Fatalf("GET failed: status %d: %s", gw.Code, gw.Body.String())
			}
			if !bytes.Equal(gw.Body.Bytes(), plaintext) {
				t.Fatal("GET after PUT returned bytes different from the uploaded plaintext")
			}

			assertNoPlaintextStored(t, rb, plaintext, fmt.Sprintf("single PUT, envelope v%d", version))
		})
	}
}

// TestCiphertextOnlyStorage_Multipart verifies the guarantee for multipart
// uploads in both guaranteed formats, at the strongest point of each: after
// every UploadPart the part body ARMOR has already handed to the backend must
// be ciphertext (the backend persists uploaded parts verbatim until
// completion), and after CompleteMultipartUpload every byte and metadata
// value the backend holds must be plaintext-free.
func TestCiphertextOnlyStorage_Multipart(t *testing.T) {
	const mib = 1024 * 1024
	for _, version := range []int{2, 3} {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			cfg, rb, h := recordingTestSetup(t)
			cfg.FormatWriteVersion = version

			bucket, key := "test-bucket", "ciphertext-only/multipart.bin"
			parts := [][]byte{
				ciphertextOnlyPlaintext(1101, 5*mib),
				ciphertextOnlyPlaintext(2202, 5*mib),
				ciphertextOnlyPlaintext(3303, 777), // short final part
			}

			uploadID := initiateMultipart(t, h, bucket, key)
			etags := make([]string, 0, len(parts))
			for i, part := range parts {
				etags = append(etags, uploadPart(t, h, bucket, key, uploadID, i+1, part))
				assertNoPlaintextStored(t, rb, part, fmt.Sprintf("multipart v%d after uploading part %d", version, i+1))
			}

			completeMultipart(t, h, bucket, key, uploadID, etags)

			var plaintext []byte
			for _, part := range parts {
				plaintext = append(plaintext, part...)
			}

			get := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/%s/%s", bucket, key), nil)
			gw := httptest.NewRecorder()
			h.HandleRoot(gw, get)
			if gw.Code != http.StatusOK {
				t.Fatalf("GET failed: status %d: %s", gw.Code, gw.Body.String())
			}
			if !bytes.Equal(gw.Body.Bytes(), plaintext) {
				t.Fatalf("multipart GET mismatch: got %d bytes, want %d; first divergence at %d",
					gw.Body.Len(), len(plaintext), firstDivergence(gw.Body.Bytes(), plaintext))
			}

			assertNoPlaintextStored(t, rb, plaintext, fmt.Sprintf("multipart v%d after completion", version))
		})
	}
}

// TestCiphertextOnlyStorage_ScanDetectsPlaintext is the control that keeps the
// scans above honest: a backend holding the plaintext verbatim must produce
// hits. Without this, a refactor of findPlaintextLeaks could silence the
// guarantee tests without any of them failing.
func TestCiphertextOnlyStorage_ScanDetectsPlaintext(t *testing.T) {
	_, rb, _ := recordingTestSetup(t)

	bucket, key := "test-bucket", "leaky/plaintext.bin"
	plaintext := ciphertextOnlyPlaintext(4404, 70000)
	if err := rb.Put(context.Background(), bucket, key, bytes.NewReader(plaintext), int64(len(plaintext)), nil); err != nil {
		t.Fatalf("store raw plaintext control object: %v", err)
	}

	if leaks := findPlaintextLeaks(rb, plaintext); len(leaks) == 0 {
		t.Fatal("leak scan reported nothing while the backend holds the plaintext verbatim — the scan is vacuous")
	}
}
