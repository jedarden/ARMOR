package restoreverifier

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// fixture reads a committed testdata fixture. Test files run with their package
// directory as the working directory, so "testdata/..." resolves correctly.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

// mustPass asserts the assertion accepts the given plaintext.
func mustPass(t *testing.T, a ArtifactAssertion, plaintext []byte, meta map[string]string) {
	t.Helper()
	if err := a.Verify(plaintext, meta); err != nil {
		t.Fatalf("%s assertion rejected a valid fixture: %v", a.Type(), err)
	}
}

// mustFail asserts the assertion rejects the plaintext and that the error
// carries the expected substring, proving the corruption was reported (not
// swallowed as a nil error or an unrelated status).
func mustFail(t *testing.T, a ArtifactAssertion, plaintext []byte, meta map[string]string, wantSub string) {
	t.Helper()
	err := a.Verify(plaintext, meta)
	if err == nil {
		t.Fatalf("%s assertion accepted a corrupt fixture (want failure mentioning %q)", a.Type(), wantSub)
	}
	if wantSub != "" && !strings.Contains(err.Error(), wantSub) {
		t.Fatalf("%s assertion failed with %q; want an error mentioning %q", a.Type(), err.Error(), wantSub)
	}
}

// ---------------------------------------------------------------------------
// SQLite
// ---------------------------------------------------------------------------

func TestSQLiteAssertion(t *testing.T) {
	a := &SQLiteAssertion{}

	t.Run("valid", func(t *testing.T) {
		mustPass(t, a, fixture(t, "valid.sqlite"), nil)
	})

	t.Run("corrupt_detected_not_swallowed", func(t *testing.T) {
		// The fixture keeps the 16-byte magic header intact, so this proves the
		// real PRAGMA integrity_check (not the cheap structural pre-check) is
		// what catches mid-file corruption.
		mustFail(t, a, fixture(t, "corrupt.sqlite"), nil, "integrity_check")
	})

	t.Run("empty_rejected", func(t *testing.T) {
		mustFail(t, a, nil, nil, "empty")
	})

	t.Run("bad_magic_rejected", func(t *testing.T) {
		// Valid bytes but a clobbered magic — caught by the structural pre-check.
		data := append([]byte(nil), fixture(t, "valid.sqlite")...)
		copy(data, []byte("NotASQLiteFormatX"))
		mustFail(t, a, data, nil, "magic")
	})
}

func TestSQLiteAssertionRowCountProbe(t *testing.T) {
	a := &SQLiteAssertion{}
	valid := fixture(t, "valid.sqlite")

	t.Run("declared_table_present", func(t *testing.T) {
		// "events" is the table the valid fixture creates; the probe must pass.
		mustPass(t, a, valid, map[string]string{"x-amz-meta-armor-sqlite-table": "events"})
	})

	t.Run("declared_table_missing", func(t *testing.T) {
		mustFail(t, a, valid, map[string]string{"x-amz-meta-armor-sqlite-table": "no_such_table"}, "not present")
	})

	t.Run("unsafe_table_name_rejected", func(t *testing.T) {
		// A NUL in the table name must be refused rather than interpolated.
		mustFail(t, a, valid, map[string]string{"x-amz-meta-armor-sqlite-table": "ev\"x"}, "unsafe")
	})
}

// ---------------------------------------------------------------------------
// Parquet
// ---------------------------------------------------------------------------

func TestParquetAssertion(t *testing.T) {
	a := &ParquetAssertion{}
	valid := fixture(t, "valid.parquet")

	t.Run("valid", func(t *testing.T) {
		mustPass(t, a, valid, nil)
	})

	t.Run("row_count_matches_metadata", func(t *testing.T) {
		// valid.parquet holds 20 rows; declaring the same count must pass.
		mustPass(t, a, valid, map[string]string{"x-amz-meta-armor-parquet-rows": "20"})
	})

	t.Run("row_count_mismatch_detected", func(t *testing.T) {
		mustFail(t, a, valid, map[string]string{"x-amz-meta-armor-parquet-rows": "999"}, "row count mismatch")
	})

	t.Run("corrupt_footer_detected_not_swallowed", func(t *testing.T) {
		// Both PAR1 magics intact; only the footer-length field is clobbered, so
		// the magic pre-check passes and the footer-parse check is what fails.
		mustFail(t, a, fixture(t, "corrupt.parquet"), nil, "footer parse")
	})

	t.Run("too_small_rejected", func(t *testing.T) {
		mustFail(t, a, []byte("PAR1"), nil, "too small")
	})
}

// ---------------------------------------------------------------------------
// tar.gz
// ---------------------------------------------------------------------------

func TestTarGzAssertion(t *testing.T) {
	a := &TarGzAssertion{}

	t.Run("valid", func(t *testing.T) {
		mustPass(t, a, fixture(t, "valid.tar.gz"), nil)
	})

	t.Run("corrupt_detected_not_swallowed", func(t *testing.T) {
		// A mid-payload byte flip breaks the compressed stream; the listing or a
		// sampled extraction must surface the failure rather than pass silently.
		mustFail(t, a, fixture(t, "corrupt.tar.gz"), nil, "tar.gz assertion")
	})

	t.Run("bad_gzip_header_rejected", func(t *testing.T) {
		mustFail(t, a, []byte("not-a-gzip-stream-at-all!!!!!!!!"), nil, "gzip header")
	})

	t.Run("empty_archive_rejected", func(t *testing.T) {
		// A well-formed gzip stream wrapping a tar with zero entries.
		var buf bytes.Buffer
		gz := newGzipWriter(&buf)
		tw := newTarWriter(gz)
		_ = tw.Close()
		_ = gz.Close()
		mustFail(t, a, buf.Bytes(), nil, "no entries")
	})
}

// ---------------------------------------------------------------------------
// Dual-path end-to-end: corrupted artifact must be caught through the real
// verifyObject flow that both restore paths feed into.
// ---------------------------------------------------------------------------

// fakeBackend serves one ARMOR object. Get returns the *plaintext* (mirroring an
// ARMOR-serving endpoint that decrypts on GET), while Head and GetRange return
// the raw ciphertext envelope and metadata — exactly what restoreViaDirectDecrypt
// needs to decrypt without an ARMOR server. Embedding backend.Backend satisfies
// the rest of the interface with nil stubs that verifyObject never calls.
type fakeBackend struct {
	backend.Backend
	ciphertext []byte
	plaintext  []byte
	info       *backend.ObjectInfo
}

func (f *fakeBackend) Get(_ context.Context, _, _ string) (io.ReadCloser, *backend.ObjectInfo, error) {
	return io.NopCloser(bytes.NewReader(f.plaintext)), f.info, nil
}

func (f *fakeBackend) Head(_ context.Context, _, _ string) (*backend.ObjectInfo, error) {
	return f.info, nil
}

func (f *fakeBackend) GetRange(_ context.Context, _, _ string, offset, length int64) (io.ReadCloser, error) {
	end := offset + length
	if offset < 0 {
		offset = 0
	}
	if end > int64(len(f.ciphertext)) {
		end = int64(len(f.ciphertext))
	}
	if offset > int64(len(f.ciphertext)) || offset > end {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	return io.NopCloser(bytes.NewReader(f.ciphertext[offset:end])), nil
}

// armorEncrypt builds a real ARMOR envelope (header + ciphertext + inline HMAC
// table) around plaintext, plus the object metadata both restore paths read. The
// returned ciphertext is what a B2-like backend would store; the returned
// metadata carries the wrapped DEK, sizes, and plaintext SHA.
func armorEncrypt(t *testing.T, mek []byte, blockSize int, plaintext []byte) (ciphertext []byte, meta map[string]string) {
	t.Helper()
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}
	wrapped, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}
	sha := crypto.ComputePlaintextSHA256(plaintext)

	header, err := crypto.NewEnvelopeHeader(iv, int64(len(plaintext)), blockSize, sha)
	if err != nil {
		t.Fatalf("NewEnvelopeHeader: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("header encode: %v", err)
	}
	enc, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(hmacTable))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	m := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: int64(len(plaintext)),
		IV:            iv,
		WrappedDEK:    wrapped,
		PlaintextSHA:  hexEncode(sha[:]),
	}).ToMetadata()
	return envelope, m
}

// TestVerifyObject_DualPathDetectsCorruption is the core ADR-004 acceptance
// test: a corrupted artifact is ARMOR-encrypted faithfully, so both restore
// paths (ARMOR read path + direct-to-ciphertext decrypt) recover *identical*
// corrupt plaintext, agree on the SHA, and then the application-level assertion
// — the only check beyond SHA-256 comparison — must catch it. This proves
// corruption is detected on the code path both restore paths feed, not
// swallowed as a pass.
func TestVerifyObject_DualPathDetectsCorruption(t *testing.T) {
	const blockSize = 4096
	// A fixed 32-byte MEK keeps the test deterministic; WrapDEK requires 32 bytes.
	mek := bytes.Repeat([]byte{0xA5}, 32)

	cases := []struct {
		name       string
		fixture    string
		atype      ArtifactType
		wantStatus VerificationStatus
		wantPass   bool
		wantSub    string // non-empty: assertion error must mention this on failure
	}{
		{
			name:       "corrupt_sqlite_caught_after_dual_path_agrees",
			fixture:    "corrupt.sqlite",
			atype:      ArtifactSQLite,
			wantStatus: StatusAssertionError,
			wantPass:   false,
			wantSub:    "integrity_check",
		},
		{
			name:       "valid_sqlite_passes_both_paths",
			fixture:    "valid.sqlite",
			atype:      ArtifactSQLite,
			wantStatus: StatusPass,
			wantPass:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plaintext := fixture(t, tc.fixture)
			ciphertext, meta := armorEncrypt(t, mek, blockSize, plaintext)

			fb := &fakeBackend{
				ciphertext: ciphertext,
				plaintext:  plaintext,
				info: &backend.ObjectInfo{
					Key:      tc.fixture,
					Size:     int64(len(plaintext)),
					Metadata: meta,
				},
			}

			v := New(fb, mek, blockSize, nil, Config{})

			result := v.verifyObject(context.Background(), ObjectSample{
				Key:          tc.fixture,
				Bucket:       "test-bucket",
				ArtifactType: tc.atype,
				Metadata:     meta,
			})

			// Both restore paths must have succeeded and agreed — the corruption
			// is at the artifact level, not the transport/decrypt level, so the
			// failure must be classified as an assertion error, never a restore
			// error or a dual-path conflict.
			if result.Path != PathDualMatch {
				t.Fatalf("expected both paths to agree (PathDualMatch), got %q (status=%q, error=%q)",
					result.Path, result.Status, result.Error)
			}
			if result.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q (error=%q)", result.Status, tc.wantStatus, result.Error)
			}
			if result.AssertionPassed != tc.wantPass {
				t.Fatalf("assertion_passed = %v, want %v (error=%q)", result.AssertionPassed, tc.wantPass, result.Error)
			}
			if !tc.wantPass {
				if result.AssertionError == "" {
					t.Fatalf("corruption swallowed: AssertionError is empty for a failed assertion")
				}
				if tc.wantSub != "" && !strings.Contains(result.AssertionError, tc.wantSub) {
					t.Fatalf("assertion error %q does not mention %q", result.AssertionError, tc.wantSub)
				}
				// The full error string must also be propagated to result.Error.
				if !strings.Contains(result.Error, "Assertion failed") {
					t.Fatalf("result.Error %q does not record the assertion failure", result.Error)
				}
			}
		})
	}
}
