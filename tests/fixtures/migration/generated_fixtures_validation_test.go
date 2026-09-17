// Manifest-driven validation of the committed regeneration set
// (generated_fixtures/): every fixture is checked against
// generated_fixtures/manifest.json as recorded data and decrypted with the
// legacy V1/V2 read path.
//
// Layers:
//   - TestRegenFixturesMatchManifestAndDecrypt   for EVERY manifest entry:
//     the artifacts on disk still hash to the recorded sizes and SHA-256s,
//     the per-fixture metadata.json agrees with the manifest row, the stored
//     bytes have the V1/V2 layout the recorded v3_expected fields describe
//     (single: envelope header + data + embedded HMAC table; multipart:
//     headerless body + sidecar sized to the recorded blocks_per_part), and
//     the legacy reader (backend.ParseARMORMetadata + internal/crypto
//     UnwrapDEK/DecodeHeader/NewDecryptorWithVersion/Decrypt -- the same
//     primitives the production V1/V2 read path uses) decrypts every valid
//     fixture to exactly the recorded plaintext SHA-256 and length.
//   - TestRegenFixtureTamperIsDetected   the loud-failure spot-check: a
//     single flipped bit in a ciphertext body or an HMAC table must fail the
//     legacy read (documented in README.md).
//
// Expectation discipline: the v3_expected fields are consumed as recorded
// data -- they are only ever compared against facts derived from the stored
// artifacts and object metadata, never recomputed by running the migration
// implementation. The generator stays independent of internal/ (see
// standalone_generator.go); only this test file imports ARMOR packages,
// because the point is to prove the committed bytes satisfy the reader.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// regenMEK is the deterministic fixture MEK (0x01, 0x02, ..., 0x20) documented
// in README.md under "Deterministic Generation" -- the key every committed
// regeneration fixture was wrapped against.
func regenMEK() []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i + 1)
	}
	return mek
}

// loadRegenManifest reads generated_fixtures/manifest.json.
func loadRegenManifest(t *testing.T) *RegenManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("generated_fixtures", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest RegenManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(manifest.Entries) == 0 {
		t.Fatal("manifest carries no entries")
	}
	return &manifest
}

// regenFixture is one fixture's on-disk material, keyed by the manifest row.
type regenFixture struct {
	Stored     []byte
	Sidecar    []byte // nil for single-PUT fixtures
	ObjectMeta map[string]string
	Meta       FixtureMetadata
}

// loadRegenFixture reads one fixture directory and checks every recorded
// artifact hash, so a tampered or truncated file fails before any crypto runs.
func loadRegenFixture(t *testing.T, entry RegenManifestEntry) regenFixture {
	t.Helper()
	dir := filepath.Join("generated_fixtures", entry.FixtureID)
	artifacts := map[string][]byte{}
	for _, art := range entry.Artifacts {
		data, err := os.ReadFile(filepath.Join(dir, art.Name))
		if err != nil {
			t.Fatalf("%s: %v", art.Name, err)
		}
		sum := sha256.Sum256(data)
		if int64(len(data)) != art.Bytes || hex.EncodeToString(sum[:]) != art.SHA256 {
			t.Fatalf("%s/%s: tampered or regenerated on disk (manifest records %d bytes sha256 %s, disk has %d bytes sha256 %s)",
				entry.FixtureID, art.Name, art.Bytes, art.SHA256, len(data), hex.EncodeToString(sum[:]))
		}
		artifacts[art.Name] = data
	}

	stored, ok := artifacts["stored_ciphertext.bin"]
	if !ok {
		t.Fatalf("%s: no stored_ciphertext.bin artifact", entry.FixtureID)
	}
	f := regenFixture{Stored: stored, Sidecar: artifacts["sidecar.bin"]}

	metaRaw, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatalf("metadata.json: %v", err)
	}
	if err := json.Unmarshal(metaRaw, &f.Meta); err != nil {
		t.Fatalf("metadata.json: %v", err)
	}
	objRaw, err := os.ReadFile(filepath.Join(dir, "object_metadata.json"))
	if err != nil {
		t.Fatalf("object_metadata.json: %v", err)
	}
	if err := json.Unmarshal(objRaw, &f.ObjectMeta); err != nil {
		t.Fatalf("object_metadata.json: %v", err)
	}
	return f
}

// formatVersionByte maps a manifest format_version to the envelope version
// byte it records.
func formatVersionByte(t *testing.T, formatVersion string) uint8 {
	t.Helper()
	switch formatVersion {
	case "v1":
		return crypto.Version1
	case "v2":
		return crypto.Version2
	default:
		t.Fatalf("unknown format_version %q", formatVersion)
		return 0
	}
}

// unwrapFixtureDEK runs the reader's DEK unwrap over the fixture's parsed
// object metadata.
func unwrapFixtureDEK(t *testing.T, armorMeta *backend.ARMORMetadata) []byte {
	t.Helper()
	dek, err := crypto.UnwrapDEK(regenMEK(), armorMeta.WrappedDEK)
	if err != nil {
		t.Fatalf("UnwrapDEK: %v", err)
	}
	return dek
}

// headerBlockCount is the block count implied by an envelope header.
func headerBlockCount(t *testing.T, header *crypto.EnvelopeHeader) int {
	t.Helper()
	bs := int64(header.BlockSize())
	if bs <= 0 {
		t.Fatalf("header block size %d", header.BlockSize())
	}
	return int((int64(header.PlaintextSize) + bs - 1) / bs)
}

// decryptRegenSingle decrypts a single-PUT fixture exactly as the legacy read
// path does: the envelope header's version byte selects the counter
// derivation, the HMAC table is embedded after the ciphertext.
func decryptRegenSingle(t *testing.T, dek []byte, stored []byte) ([]byte, *crypto.EnvelopeHeader) {
	t.Helper()
	header, err := crypto.DecodeHeader(stored[:crypto.HeaderSize])
	if err != nil {
		t.Fatalf("DecodeHeader: %v", err)
	}
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
	if err != nil {
		t.Fatalf("NewDecryptorWithVersion: %v", err)
	}
	body := stored[crypto.HeaderSize:]
	tableSize := headerBlockCount(t, header) * crypto.HMACSize
	plaintext, err := decryptor.Decrypt(body[:len(body)-tableSize], body[len(body)-tableSize:])
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	return plaintext, header
}

// decryptRegenMultipart decrypts a multipart fixture as the legacy read path
// does: no envelope header, IV/version/block size from the object metadata,
// HMAC table from the sidecar.
func decryptRegenMultipart(t *testing.T, dek []byte, stored, sidecar []byte, armorMeta *backend.ARMORMetadata) []byte {
	t.Helper()
	decryptor, err := crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
	if err != nil {
		t.Fatalf("NewDecryptorWithVersion: %v", err)
	}
	plaintext, err := decryptor.Decrypt(stored, sidecar)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	return plaintext
}

// requirePlaintextFacts asserts the decrypted plaintext carries exactly the
// manifest's recorded plaintext facts.
func requirePlaintextFacts(t *testing.T, entry RegenManifestEntry, plaintext []byte) {
	t.Helper()
	sum := sha256.Sum256(plaintext)
	if got := hex.EncodeToString(sum[:]); got != entry.PlaintextSHA256 {
		t.Errorf("decrypted plaintext sha256 = %s, manifest records %s", got, entry.PlaintextSHA256)
	}
	if int64(len(plaintext)) != entry.PlaintextLength {
		t.Errorf("decrypted plaintext length = %d, manifest records %d", len(plaintext), entry.PlaintextLength)
	}
}

// requireSidecarPathShape checks the recorded sidecar_path is the read path's
// .armor/hmac/<sha256(key)> shape.
func requireSidecarPathShape(t *testing.T, entry RegenManifestEntry) {
	t.Helper()
	const prefix = ".armor/hmac/"
	if !strings.HasPrefix(entry.V3Expected.SidecarPath, prefix) {
		t.Errorf("sidecar_path = %q, want %q prefix", entry.V3Expected.SidecarPath, prefix)
		return
	}
	digest := strings.TrimPrefix(entry.V3Expected.SidecarPath, prefix)
	if len(digest) != 64 {
		t.Errorf("sidecar_path digest %q is %d chars, want 64 hex chars", digest, len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Errorf("sidecar_path digest %q is not hex: %v", digest, err)
	}
}

// TestRegenFixturesMatchManifestAndDecrypt validates every manifest entry
// against the committed bytes: recorded artifact hashes, recorded plaintext
// facts after a legacy-reader decrypt, and the stored layout the recorded
// v3_expected fields describe.
func TestRegenFixturesMatchManifestAndDecrypt(t *testing.T) {
	manifest := loadRegenManifest(t)
	for _, entry := range manifest.Entries {
		entry := entry
		t.Run(entry.FixtureID, func(t *testing.T) {
			f := loadRegenFixture(t, entry)

			// The manifest row must agree with the per-fixture
			// metadata.json (both are recorded data; they must tell
			// one story).
			if f.Meta.PlaintextSHA256 != entry.PlaintextSHA256 ||
				f.Meta.PlaintextLength != entry.PlaintextLength ||
				f.Meta.SourceVersion != entry.FormatVersion {
				t.Errorf("metadata.json disagrees with the manifest row on the recorded facts")
			}
			if !reflect.DeepEqual(f.Meta.V3Expected, entry.V3Expected) {
				t.Errorf("v3_expected recorded in metadata.json (%+v) differs from the manifest row (%+v)",
					f.Meta.V3Expected, entry.V3Expected)
			}
			if f.Meta.ExpectedMigrationOutcome != "success" {
				t.Errorf("expected_migration_outcome = %q, harness only validates success fixtures",
					f.Meta.ExpectedMigrationOutcome)
			}

			armorMeta, ok := backend.ParseARMORMetadata(f.ObjectMeta)
			if !ok {
				t.Fatalf("object_metadata.json did not parse as ARMOR metadata")
			}

			// The recorded compression flag must match the object
			// metadata the reader actually parses.
			if armorMeta.Compressed != entry.V3Expected.CompressionUsed {
				t.Errorf("compression_used recorded as %v but object metadata parses Compressed=%v",
					entry.V3Expected.CompressionUsed, armorMeta.Compressed)
			}

			if entry.V3Expected.IsMultipart {
				validateRegenMultipart(t, entry, f, armorMeta)
			} else {
				validateRegenSingle(t, entry, f, armorMeta)
			}
		})
	}
}

// validateRegenMultipart checks a multipart fixture's stored layout against
// the recorded v3_expected fields, then decrypts it with the legacy reader.
func validateRegenMultipart(t *testing.T, entry RegenManifestEntry, f regenFixture, armorMeta *backend.ARMORMetadata) {
	t.Helper()

	// A multipart fixture must say so in its object metadata and carry
	// a sidecar artifact, and the recorded layout must be derivable
	// from the recorded sizes.
	if f.ObjectMeta["x-amz-meta-armor-multipart"] != "true" {
		t.Errorf("is_multipart recorded true but object metadata lacks x-amz-meta-armor-multipart=true")
	}
	if f.Sidecar == nil {
		t.Fatalf("multipart fixture has no sidecar.bin artifact")
	}
	if uint8(armorMeta.Version) != formatVersionByte(t, entry.FormatVersion) {
		t.Errorf("object metadata version = %d, format_version records %q", armorMeta.Version, entry.FormatVersion)
	}
	if armorMeta.PlaintextSize != entry.PlaintextLength {
		t.Errorf("object metadata plaintext size = %d, manifest records %d", armorMeta.PlaintextSize, entry.PlaintextLength)
	}
	if armorMeta.BlockSize <= 0 {
		t.Fatalf("object metadata block size = %d", armorMeta.BlockSize)
	}
	var partSize int64
	if _, err := fmt.Sscanf(f.ObjectMeta["x-amz-meta-armor-part-size"], "%d", &partSize); err != nil || partSize <= 0 {
		t.Fatalf("object metadata part size %q", f.ObjectMeta["x-amz-meta-armor-part-size"])
	}

	totalBlocks := (entry.PlaintextLength + int64(armorMeta.BlockSize) - 1) / int64(armorMeta.BlockSize)
	wantPartCount := (entry.PlaintextLength + partSize - 1) / partSize
	if int64(entry.V3Expected.PartCount) != wantPartCount {
		t.Errorf("part_count recorded %d, plaintext_length/part_size derive %d",
			entry.V3Expected.PartCount, wantPartCount)
	}
	if len(entry.V3Expected.BlocksPerPart) != entry.V3Expected.PartCount {
		t.Errorf("blocks_per_part has %d entries, part_count records %d",
			len(entry.V3Expected.BlocksPerPart), entry.V3Expected.PartCount)
	}
	var recordedBlocks int64
	for i, blocks := range entry.V3Expected.BlocksPerPart {
		if blocks <= 0 {
			t.Errorf("blocks_per_part[%d] = %d, want > 0", i, blocks)
		}
		recordedBlocks += int64(blocks)
	}
	if recordedBlocks != totalBlocks {
		t.Errorf("blocks_per_part sums to %d, plaintext_length/block_size derive %d",
			recordedBlocks, totalBlocks)
	}
	if wantSidecarBytes := totalBlocks * crypto.HMACSize; int64(len(f.Sidecar)) != wantSidecarBytes {
		t.Errorf("sidecar.bin is %d bytes, recorded blocks_per_part imply %d",
			len(f.Sidecar), wantSidecarBytes)
	}
	requireSidecarPathShape(t, entry)

	dek := unwrapFixtureDEK(t, armorMeta)
	plaintext := decryptRegenMultipart(t, dek, f.Stored, f.Sidecar, armorMeta)
	requirePlaintextFacts(t, entry, plaintext)
}

// validateRegenSingle checks a single-PUT fixture's stored layout against the
// recorded v3_expected fields, then decrypts it with the legacy reader.
func validateRegenSingle(t *testing.T, entry RegenManifestEntry, f regenFixture, armorMeta *backend.ARMORMetadata) {
	t.Helper()

	// A single-PUT fixture must not record multipart layout fields.
	if entry.V3Expected.PartCount != 0 || len(entry.V3Expected.BlocksPerPart) != 0 || entry.V3Expected.SidecarPath != "" {
		t.Errorf("single-PUT fixture records multipart layout fields: %+v", entry.V3Expected)
	}
	if f.ObjectMeta["x-amz-meta-armor-multipart"] == "true" {
		t.Errorf("single-PUT fixture carries x-amz-meta-armor-multipart=true")
	}

	// Stored layout: header + ciphertext + embedded HMAC table, all
	// three sized by the header's recorded facts.
	if int64(len(f.Stored)) < crypto.HeaderSize {
		t.Fatalf("stored_ciphertext.bin is %d bytes, shorter than the %d-byte header",
			len(f.Stored), crypto.HeaderSize)
	}
	dek := unwrapFixtureDEK(t, armorMeta)
	plaintext, header := decryptRegenSingle(t, dek, f.Stored)

	if header.Version != formatVersionByte(t, entry.FormatVersion) {
		t.Errorf("envelope header version byte = %#x, format_version records %q",
			header.Version, entry.FormatVersion)
	}
	if int64(header.PlaintextSize) != entry.PlaintextLength {
		t.Errorf("envelope header plaintext size = %d, manifest records %d",
			header.PlaintextSize, entry.PlaintextLength)
	}
	if got := hex.EncodeToString(header.PlaintextSHA[:]); got != entry.PlaintextSHA256 {
		t.Errorf("envelope header plaintext sha256 = %s, manifest records %s", got, entry.PlaintextSHA256)
	}
	wantStored := crypto.HeaderSize + int64(header.PlaintextSize) + int64(headerBlockCount(t, header)*crypto.HMACSize)
	if int64(len(f.Stored)) != wantStored {
		t.Errorf("stored_ciphertext.bin is %d bytes, header-derived single-PUT layout is %d",
			len(f.Stored), wantStored)
	}
	if armorMeta.BlockSize != header.BlockSize() {
		t.Errorf("object metadata block size = %d, envelope header records %d",
			armorMeta.BlockSize, header.BlockSize())
	}

	requirePlaintextFacts(t, entry, plaintext)
}

// TestRegenFixtureTamperIsDetected is the loud-failure spot-check documented
// in README.md: flipping one bit of a ciphertext body or of an HMAC table
// must make the legacy read fail. If any tampered copy still decrypts, the
// harness (and the reader) would not catch corruption of the committed
// artifacts.
func TestRegenFixtureTamperIsDetected(t *testing.T) {
	manifest := loadRegenManifest(t)
	for _, entry := range manifest.Entries {
		entry := entry
		t.Run(entry.FixtureID, func(t *testing.T) {
			f := loadRegenFixture(t, entry)
			armorMeta, ok := backend.ParseARMORMetadata(f.ObjectMeta)
			if !ok {
				t.Fatalf("object_metadata.json did not parse as ARMOR metadata")
			}
			dek := unwrapFixtureDEK(t, armorMeta)

			decrypt := func(stored, sidecar []byte) error {
				t.Helper()
				if entry.V3Expected.IsMultipart {
					return multipartDecryptErr(dek, stored, sidecar, armorMeta)
				}
				_, err := decryptSingleForTamper(dek, stored)
				return err
			}

			// Baseline: the committed bytes decrypt.
			if err := decrypt(f.Stored, f.Sidecar); err != nil {
				t.Fatalf("committed bytes fail the legacy read: %v", err)
			}

			// Spot-check 1: one flipped bit in the ciphertext body
			// must fail HMAC verification. Flip the middle of the
			// body (everything after the envelope header, for
			// single-PUT).
			bodyStart := int64(0)
			if !entry.V3Expected.IsMultipart {
				bodyStart = crypto.HeaderSize
			}
			tamperedBody := append([]byte(nil), f.Stored...)
			tamperedBody[bodyStart+(int64(len(tamperedBody))-bodyStart)/2] ^= 0x01
			if err := decrypt(tamperedBody, f.Sidecar); err == nil {
				t.Errorf("flipping a ciphertext bit still decrypted -- tampering would go undetected")
			}

			// Spot-check 2: one flipped bit in the HMAC table (the
			// sidecar for multipart, the embedded table for
			// single-PUT) must fail verification.
			if entry.V3Expected.IsMultipart {
				tamperedSidecar := append([]byte(nil), f.Sidecar...)
				tamperedSidecar[len(tamperedSidecar)-1] ^= 0x01
				if err := decrypt(f.Stored, tamperedSidecar); err == nil {
					t.Errorf("flipping a sidecar HMAC bit still decrypted -- tampering would go undetected")
				}
			} else {
				tamperedStored := append([]byte(nil), f.Stored...)
				tamperedStored[len(tamperedStored)-1] ^= 0x01
				if err := decrypt(tamperedStored, nil); err == nil {
					t.Errorf("flipping an embedded HMAC bit still decrypted -- tampering would go undetected")
				}
			}
		})
	}
}

// decryptSingleForTamper is decryptRegenSingle without Fatalf, so the tamper
// spot-check can observe the reader's rejection instead of aborting the
// subtest.
func decryptSingleForTamper(dek, stored []byte) ([]byte, error) {
	header, err := crypto.DecodeHeader(stored[:crypto.HeaderSize])
	if err != nil {
		return nil, err
	}
	blockSize := header.BlockSize()
	if blockSize <= 0 {
		return nil, fmt.Errorf("invalid block size: %d", blockSize)
	}
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], blockSize, header.Version)
	if err != nil {
		return nil, err
	}
	tableSize := ((int64(header.PlaintextSize) + int64(blockSize) - 1) / int64(blockSize)) * crypto.HMACSize
	body := stored[crypto.HeaderSize:]
	if int64(len(body)) < tableSize {
		return nil, fmt.Errorf("ciphertext too short to contain HMAC table: got %d, need %d", len(body), tableSize)
	}
	return decryptor.Decrypt(body[:int64(len(body))-tableSize], body[int64(len(body))-tableSize:])
}

// multipartDecryptErr is decryptRegenMultipart without Fatalf, for the same
// reason.
func multipartDecryptErr(dek, stored, sidecar []byte, armorMeta *backend.ARMORMetadata) error {
	decryptor, err := crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
	if err != nil {
		return err
	}
	_, err = decryptor.Decrypt(stored, sidecar)
	return err
}
