package restoreverifier

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
// ARMOR-serving endpoint that decrypts on GET) and counts how many times it is
// called in armorGet — the direct-only DR drill must keep this at zero, proving
// the ARMOR read path is never exercised. Head and GetRange return the raw
// ciphertext (envelope for single-PUT, bare part ciphertext for multipart) and
// metadata — exactly what restoreViaDirectDecrypt needs to decrypt without an
// ARMOR server. GetDirect serves JSON HMAC sidecars for multipart objects.
// Embedding backend.Backend satisfies the rest of the interface with nil stubs
// that verifyObject never calls.
type fakeBackend struct {
	backend.Backend
	ciphertext []byte
	plaintext  []byte
	info       *backend.ObjectInfo
	sidecars   map[string][]byte // JSON HMAC sidecars keyed by ".armor/hmac/<hex>"; multipart only
	armorGet   int               // calls to Get (the ARMOR read path); a drill run must leave this 0
}

func (f *fakeBackend) Get(_ context.Context, _, _ string) (io.ReadCloser, *backend.ObjectInfo, error) {
	f.armorGet++
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

// GetDirect serves a JSON HMAC sidecar for a multipart object. The key is the
// sidecar object name ".armor/hmac/<hex(sha256(key))>" that the verifier (via
// MultipartStateManager.LoadHMACTable) fetches without an ARMOR server.
func (f *fakeBackend) GetDirect(_ context.Context, _, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	data, ok := f.sidecars[key]
	if !ok {
		return nil, nil, fmt.Errorf("fakeBackend: no sidecar registered for %q", key)
	}
	return io.NopCloser(bytes.NewReader(data)), &backend.ObjectInfo{Key: key}, nil
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

// armorEncryptMultipart builds an ADR-003 multipart object: bare concatenated
// part ciphertext with NO envelope header and the per-block HMAC table stored as
// a JSON sidecar (the HMACTableSidecar wire format the server writes), plus the
// metadata both restore paths read. It mirrors what CompleteMultipartUpload
// produces: x-amz-meta-armor-multipart=true, IV/wrapped-DEK/sizes in metadata,
// and the empty-string placeholder plaintext digest (open gap bf-1v2ehf). The
// returned ciphertext is what a B2-like backend stores for the object body; the
// returned sidecar bytes are what lives at .armor/hmac/<sha256(key)>.
func armorEncryptMultipart(t *testing.T, mek []byte, blockSize int, key string, plaintext []byte) (ciphertext []byte, sidecar []byte, meta map[string]string) {
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
	enc, err := crypto.NewEncryptor(dek, iv, blockSize)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	encrypted, hmacTable, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Multipart body is raw ciphertext only — no header, no trailing HMAC.
	// Split the flattened HMAC table into one entry per block, exactly as the
	// server's SaveHMACTable stores them in the JSON sidecar.
	blockHMACs := make([][]byte, 0, len(hmacTable)/crypto.HMACSize)
	for i := 0; i < len(hmacTable); i += crypto.HMACSize {
		blockHMACs = append(blockHMACs, append([]byte(nil), hmacTable[i:i+crypto.HMACSize]...))
	}
	sidecarObj := backend.HMACTableSidecar{
		Key:        key,
		BlockHMACs: blockHMACs,
		BlockSize:  blockSize,
	}
	sidecar, err = json.Marshal(sidecarObj)
	if err != nil {
		t.Fatalf("marshal sidecar: %v", err)
	}

	m := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     blockSize,
		PlaintextSize: int64(len(plaintext)),
		IV:            iv,
		WrappedDEK:    wrapped,
		// Multipart completion stores the empty-string placeholder, not the true
		// whole-object SHA (bf-1v2ehf); mirror that so the test reflects reality.
		PlaintextSHA: emptyStringSHA256Hex,
	}).ToMetadata()
	m["x-amz-meta-armor-multipart"] = "true"
	return encrypted, sidecar, m
}

// sidecarKeyFor returns the .armor/hmac/<hex(sha256(key))> object name the
// verifier fetches for a multipart object's HMAC table.
func sidecarKeyFor(key string) string {
	h := sha256.Sum256([]byte(key))
	return fmt.Sprintf(".armor/hmac/%s", hex.EncodeToString(h[:]))
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
			}, ModeDual)

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

// ---------------------------------------------------------------------------
// DR-drill (direct-only): ModeDRDrill runs ONLY the armor-decrypt direct path
// and must never touch the ARMOR read path. These tests prove the
// "ARMOR-server-is-gone" recovery on both on-B2 layouts (single-PUT envelope
// and ADR-003 multipart sidecar) and that corruption is still caught — all with
// the ARMOR read path call counter pinned at zero.
// ---------------------------------------------------------------------------

// TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath is the core DR-drill
// acceptance test. For each on-B2 layout it asserts that ModeDRDrill recovers
// the object (or catches a corrupt artifact) through the direct path alone —
// the ARMOR read path (Get) is invoked zero times, proving recovery works with
// the server deliberately gone. The multipart case is the ADR-003 honoring
// case: no envelope header, ciphertext at offset 0, HMAC table in the JSON
// sidecar at .armor/hmac/<sha256(key)>.
func TestVerifyObject_DRDrill_DirectOnlyExcludesARMORReadPath(t *testing.T) {
	const blockSize = 4096
	// A fixed 32-byte MEK keeps the test deterministic; WrapDEK requires 32 bytes.
	mek := bytes.Repeat([]byte{0xA5}, 32)

	cases := []struct {
		name       string
		setup      func(t *testing.T) (*fakeBackend, ObjectSample)
		wantStatus VerificationStatus
		wantPass   bool
	}{
		{
			name: "single_put_valid_recovers_direct_only",
			setup: func(t *testing.T) (*fakeBackend, ObjectSample) {
				plaintext := fixture(t, "valid.sqlite")
				ct, meta := armorEncrypt(t, mek, blockSize, plaintext)
				key := "valid.sqlite"
				fb := &fakeBackend{
					ciphertext: ct, plaintext: plaintext,
					info: &backend.ObjectInfo{Key: key, Size: int64(len(plaintext)), Metadata: meta},
				}
				return fb, ObjectSample{Key: key, Bucket: "b", ArtifactType: ArtifactSQLite, Metadata: meta}
			},
			wantStatus: StatusPass,
			wantPass:   true,
		},
		{
			name: "single_put_corrupt_artifact_caught_direct_only",
			setup: func(t *testing.T) (*fakeBackend, ObjectSample) {
				plaintext := fixture(t, "corrupt.sqlite")
				ct, meta := armorEncrypt(t, mek, blockSize, plaintext)
				key := "corrupt.sqlite"
				fb := &fakeBackend{
					ciphertext: ct, plaintext: plaintext,
					info: &backend.ObjectInfo{Key: key, Size: int64(len(plaintext)), Metadata: meta},
				}
				return fb, ObjectSample{Key: key, Bucket: "b", ArtifactType: ArtifactSQLite, Metadata: meta}
			},
			wantStatus: StatusAssertionError,
			wantPass:   false,
		},
		{
			name: "multipart_sidecar_recovers_direct_only_adr003",
			setup: func(t *testing.T) (*fakeBackend, ObjectSample) {
				plaintext := fixture(t, "valid.sqlite")
				// A litestream-style multipart key exercises the sidecar path.
				key := "litestream/db.snap"
				ct, sidecar, meta := armorEncryptMultipart(t, mek, blockSize, key, plaintext)
				fb := &fakeBackend{
					ciphertext: ct, plaintext: plaintext,
					info:     &backend.ObjectInfo{Key: key, Size: int64(len(ct)), Metadata: meta},
					sidecars: map[string][]byte{sidecarKeyFor(key): sidecar},
				}
				return fb, ObjectSample{Key: key, Bucket: "b", ArtifactType: ArtifactSQLite, Metadata: meta}
			},
			wantStatus: StatusPass,
			wantPass:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb, obj := tc.setup(t)
			v := New(fb, mek, blockSize, nil, Config{})

			result := v.verifyObject(context.Background(), obj, ModeDRDrill)

			// The defining guarantee of the drill: the ARMOR read path (Get) is
			// never exercised — recovery is proven with the server gone.
			if fb.armorGet != 0 {
				t.Fatalf("DR drill invoked the ARMOR read path %d time(s); the direct-only path must never call Get", fb.armorGet)
			}
			if result.Path != PathDirect {
				t.Fatalf("result.Path = %q, want %q (status=%q, error=%q)",
					result.Path, PathDirect, result.Status, result.Error)
			}
			if result.Status != tc.wantStatus {
				t.Fatalf("status = %q, want %q (error=%q)", result.Status, tc.wantStatus, result.Error)
			}
			if result.AssertionPassed != tc.wantPass {
				t.Fatalf("assertion_passed = %v, want %v (error=%q)", result.AssertionPassed, tc.wantPass, result.Error)
			}
			// A direct-only run must populate the direct-path SHA and latency.
			if result.DirectSHA256 == "" {
				t.Fatalf("DirectSHA256 not populated for a direct-only run")
			}
			if result.DirectPathLatency <= 0 {
				t.Fatalf("DirectPathLatency not recorded for a direct-only run")
			}
		})
	}
}

// TestVerifyObject_DRDrill_ChecksumMismatch confirms a direct-only run enforces
// a declared (non-placeholder) plaintext SHA-256 and reports a checksum error
// when the recovered plaintext does not match it — without ever calling the
// ARMOR read path.
func TestVerifyObject_DRDrill_ChecksumMismatch(t *testing.T) {
	const blockSize = 4096
	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")
	ct, meta := armorEncrypt(t, mek, blockSize, plaintext)
	key := "valid.sqlite"

	// Declare a real (non-placeholder) digest that does NOT match the recovered
	// plaintext, so verifyObjectDirectOnly's checksum branch is exercised.
	objMeta := make(map[string]string, len(meta))
	for k, v := range meta {
		objMeta[k] = v
	}
	objMeta["x-amz-meta-armor-plaintext-sha256"] = strings.Repeat("a", 64)

	fb := &fakeBackend{
		ciphertext: ct, plaintext: plaintext,
		info: &backend.ObjectInfo{Key: key, Size: int64(len(plaintext)), Metadata: meta},
	}
	v := New(fb, mek, blockSize, nil, Config{})

	result := v.verifyObject(context.Background(), ObjectSample{
		Key: key, Bucket: "b", ArtifactType: ArtifactSQLite, Metadata: objMeta,
	}, ModeDRDrill)

	if fb.armorGet != 0 {
		t.Fatalf("DR drill invoked the ARMOR read path %d time(s)", fb.armorGet)
	}
	if result.Path != PathDirect {
		t.Fatalf("result.Path = %q, want %q", result.Path, PathDirect)
	}
	if result.Status != StatusChecksumError {
		t.Fatalf("status = %q, want %q (error=%q)", result.Status, StatusChecksumError, result.Error)
	}
	if !strings.Contains(result.Error, "SHA256 mismatch") {
		t.Fatalf("error %q does not mention the checksum mismatch", result.Error)
	}
}

// TestVerifyObject_DualPathExercisesARMORReadPath is the contrast to the drill
// tests above: ModeDual MUST call the ARMOR read path (Get). It proves armorGet
// is a real counter and that the drill's zero is a genuine mode-specific
// exclusion, not a broken probe.
func TestVerifyObject_DualPathExercisesARMORReadPath(t *testing.T) {
	const blockSize = 4096
	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")
	ct, meta := armorEncrypt(t, mek, blockSize, plaintext)
	key := "valid.sqlite"
	fb := &fakeBackend{
		ciphertext: ct, plaintext: plaintext,
		info: &backend.ObjectInfo{Key: key, Size: int64(len(plaintext)), Metadata: meta},
	}
	v := New(fb, mek, blockSize, nil, Config{})

	result := v.verifyObject(context.Background(), ObjectSample{
		Key: key, Bucket: "b", ArtifactType: ArtifactSQLite, Metadata: meta,
	}, ModeDual)

	if fb.armorGet == 0 {
		t.Fatalf("dual-path run never called the ARMOR read path (Get); expected at least one call")
	}
	if result.Path != PathDualMatch {
		t.Fatalf("result.Path = %q, want %q (status=%q, error=%q)",
			result.Path, PathDualMatch, result.Status, result.Error)
	}
	if result.Status != StatusPass {
		t.Fatalf("status = %q, want %q (error=%q)", result.Status, StatusPass, result.Error)
	}
}
