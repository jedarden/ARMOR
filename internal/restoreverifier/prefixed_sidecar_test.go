package restoreverifier

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// The multipart HMAC sidecar is named sha256 of the CLIENT key — the key the
// uploading client used — while the assembled ciphertext object is stored under
// the PREFIXED key (CompleteMultipartUpload saves the sidecar with the
// unprefixed key and stores the ciphertext under applyPrefix(key)). The
// verifier sees only stored keys, because that is what backend.List returns and
// what Head/GetRange address, so it has to strip the bucket's ADR-001 prefix
// before hashing. These tests prove both restore paths do, on a prefixed
// bucket, for both sidecar formats; the sidecar is deliberately registered ONLY
// under the client-key name the server writes, so a prefixed-name lookup fails
// loudly instead of silently passing.

const prefixedSidecarTestPrefix = "commitgraph/"

// armorEncryptV3Multipart builds a v3 ADR-005 multipart-completed object the way
// the server writes it: raw concatenated block ciphertext (no envelope header),
// a gzip-compressed JSON sidecar carrying [hmac, clen] per block, and metadata
// carrying the IV, the wrapped DEK and the multipart dispatch marker.
//
// It writes exactly ONE part covering the whole object (part N=1), which is what
// a single-part multipart upload produces. Both verifier restore paths decrypt
// the merged sidecar table with part=1 (verifier.go), so a multi-part fixture
// would fail HMAC verification for the later parts — a separate limitation, not
// the sidecar-naming defect under test here.
func armorEncryptV3Multipart(t *testing.T, mek []byte, blockSize int, plaintext []byte) (ciphertext, sidecarJSON []byte, meta map[string]string) {
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

	blockCount := crypto.ComputeBlockCount(int64(len(plaintext)), blockSize)
	blocks := make([][]string, 0, blockCount)
	for blockIdx := uint32(0); blockIdx < blockCount; blockIdx++ {
		start := int64(blockIdx) * int64(blockSize)
		end := start + int64(blockSize)
		if end > int64(len(plaintext)) {
			end = int64(len(plaintext))
		}
		blockCT, blockHMAC, err := crypto.EncryptBlockV3(dek, iv, 1, blockIdx, plaintext[start:end], blockSize)
		if err != nil {
			t.Fatalf("EncryptBlockV3 block %d: %v", blockIdx, err)
		}
		ciphertext = append(ciphertext, blockCT...)
		// Uncompressed blocks: clen is the plain ciphertext length (no flag bit).
		blocks = append(blocks, []string{
			base64.StdEncoding.EncodeToString(blockHMAC),
			strconv.FormatUint(uint64(len(blockCT)), 10),
		})
	}

	sidecarV3 := &backend.HMACTableSidecarV3{
		Version:   3,
		BlockSize: blockSize,
		Parts: []backend.HMACPartV3{{
			N:             1,
			PlaintextLen:  int64(len(plaintext)),
			CiphertextLen: int64(len(ciphertext)),
			Blocks:        blocks,
		}},
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(sidecarV3); err != nil {
		t.Fatalf("marshal v3 sidecar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	meta = (&backend.ARMORMetadata{
		Version:        3,
		BlockSize:      blockSize,
		PlaintextSize:  int64(len(plaintext)),
		IV:             iv,
		WrappedDEK:     wrapped,
		MEKFingerprint: crypto.MEKFingerprint(mek),
		// Legacy multipart placeholder (bf-1v2ehf): "no digest declared", so the
		// checksum comparison is skipped and only the dual-path agreement and the
		// artifact assertion are enforced.
		PlaintextSHA: emptyStringSHA256Hex,
	}).ToMetadata()
	meta["x-amz-meta-armor-multipart"] = "true"
	meta["x-amz-meta-armor-part-size"] = strconv.FormatInt(int64(blockSize), 10)

	return ciphertext, buf.Bytes(), meta
}

// TestVerifyObject_PrefixedBucket_MultipartSidecarNamedByClientKey is the
// restoreverifier half of the armor-1b272971 defect: on a bucket with an
// ADR-001 prefix, hashing the stored key names a sidecar the server never
// wrote, so every multipart verify reported a false restore failure. Both
// restore paths (ARMOR read path and direct decrypt) must fetch the sidecar by
// the client key while still addressing the ciphertext by the stored key.
func TestVerifyObject_PrefixedBucket_MultipartSidecarNamedByClientKey(t *testing.T) {
	const (
		bucket    = "test-bucket"
		storedKey = prefixedSidecarTestPrefix + "backups/db.snapshot"
		clientKey = "backups/db.snapshot"
		blockSize = 4096
	)

	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")

	testCases := []struct {
		name  string
		setup func(t *testing.T) (ciphertext []byte, sidecarJSON []byte, meta map[string]string)
	}{
		{
			name: "v1_v2_multipart_json_sidecar",
			setup: func(t *testing.T) ([]byte, []byte, map[string]string) {
				// The server saves the sidecar with the client key, so the
				// sidecar's embedded Key field carries it too.
				ct, sidecar, meta := armorEncryptMultipart(t, mek, blockSize, clientKey, plaintext)
				return ct, sidecar, meta
			},
		},
		{
			name: "v3_multipart_gzip_sidecar",
			setup: func(t *testing.T) ([]byte, []byte, map[string]string) {
				return armorEncryptV3Multipart(t, mek, blockSize, plaintext)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, sidecarJSON, meta := tc.setup(t)

			fb := &fakeBackend{
				ciphertext: ciphertext,
				plaintext:  plaintext,
				info: &backend.ObjectInfo{
					// The stored key: what List returns and what Head/GetRange
					// must be given on a prefixed bucket.
					Key:      storedKey,
					Size:     int64(len(plaintext)),
					Metadata: meta,
				},
				// Exactly one sidecar, under the name the server writes:
				// sha256 of the CLIENT key.
				sidecars: map[string][]byte{sidecarKeyFor(clientKey): sidecarJSON},
			}

			v := New(fb, mek, nil, blockSize, nil, Config{
				Buckets: []BucketConfig{{
					Bucket:  bucket,
					Prefix:  prefixedSidecarTestPrefix,
					Enabled: true,
				}},
			})

			result := v.verifyObject(context.Background(), ObjectSample{
				Key:          storedKey,
				Bucket:       bucket,
				ArtifactType: ArtifactSQLite,
				Metadata:     meta,
			}, ModeDual)

			// The sidecar must have been fetched by the client-key name, once
			// per restore path, and never by the prefixed-key name.
			wantLookups := []string{sidecarKeyFor(clientKey), sidecarKeyFor(clientKey)}
			if len(fb.sidecarLookups) != len(wantLookups) {
				t.Fatalf("sidecar lookups = %v, want %v", fb.sidecarLookups, wantLookups)
			}
			for i, got := range fb.sidecarLookups {
				if got != wantLookups[i] {
					t.Fatalf("sidecar lookup %d = %q, want %q (the prefixed key must not be hashed)",
						i, got, wantLookups[i])
				}
			}

			if result.Status != StatusPass || result.Path != PathDualMatch {
				t.Fatalf("expected StatusPass/PathDualMatch on a prefixed bucket, got status=%q path=%q error=%q",
					result.Status, result.Path, result.Error)
			}
			if result.ARMORSHA256 == "" || result.DirectSHA256 == "" {
				t.Fatalf("both restore paths must produce a digest: ARMOR=%q Direct=%q",
					result.ARMORSHA256, result.DirectSHA256)
			}
			if result.ARMORSHA256 != result.DirectSHA256 {
				t.Fatalf("dual-path digest mismatch: ARMOR=%s Direct=%s",
					result.ARMORSHA256, result.DirectSHA256)
			}
		})
	}
}

// TestClientKeyStripsOnlyTheBucketPrefix pins the mapping the sidecar lookups
// above depend on: the stored key is the client key with the bucket's prefix in
// front of it, and an unprefixed deployment's keys are already client keys.
func TestClientKeyStripsOnlyTheBucketPrefix(t *testing.T) {
	const bucket = "test-bucket"

	testCases := []struct {
		prefix    string
		storedKey string
		want      string
	}{
		{prefix: "", storedKey: "a/b.sqlite", want: "a/b.sqlite"},
		{prefix: prefixedSidecarTestPrefix, storedKey: prefixedSidecarTestPrefix + "a/b.sqlite", want: "a/b.sqlite"},
		// A key that merely contains the prefix text is not a prefixed key.
		{prefix: prefixedSidecarTestPrefix, storedKey: "other/" + prefixedSidecarTestPrefix + "a.sqlite", want: "other/" + prefixedSidecarTestPrefix + "a.sqlite"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("prefix=%q", tc.prefix), func(t *testing.T) {
			v := New(&fakeBackend{}, bytes.Repeat([]byte{0xA5}, 32), nil, 4096, nil, Config{
				Buckets: []BucketConfig{{Bucket: bucket, Prefix: tc.prefix, Enabled: true}},
			})
			if got := v.clientKey(bucket, tc.storedKey); got != tc.want {
				t.Fatalf("clientKey(%q) = %q, want %q", tc.storedKey, got, tc.want)
			}
		})
	}
}
