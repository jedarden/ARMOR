package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

func TestFormatMigrationRejectsEnvelopeMetadataVersionMismatch(t *testing.T) {
	tests := []struct {
		name          string
		headerVersion uint8
		metaVersion   string
		v2WrappedDEK  bool
	}{
		{name: "envelope_version_mismatch", headerVersion: crypto.Version1, metaVersion: "2"},
		{name: "v1_object_v2_metadata", headerVersion: crypto.Version1, metaVersion: "2"},
		{name: "v2_object_v1_metadata", headerVersion: crypto.Version2, metaVersion: "1", v2WrappedDEK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			backend := NewMockBackend()
			mek := bytes.Repeat([]byte{0x11}, 32)
			dek := bytes.Repeat([]byte{0x22}, 32)
			wrapped, err := crypto.WrapDEK(mek, dek)
			if err != nil {
				t.Fatalf("wrap DEK: %v", err)
			}

			iv := bytes.Repeat([]byte{0x33}, 16)
			plaintext := []byte("mismatch")
			plaintextSHA := sha256.Sum256(plaintext)
			header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), 4096, plaintextSHA, tc.headerVersion)
			if err != nil {
				t.Fatalf("build header: %v", err)
			}
			body, err := header.Encode()
			if err != nil {
				t.Fatalf("encode header: %v", err)
			}

			wrappedMetadata := base64.StdEncoding.EncodeToString(wrapped)
			if tc.v2WrappedDEK {
				wrappedMetadata = "v2:0123456789abcdef:" + wrappedMetadata
			}
			metadata := map[string]string{
				armorMetaVersion:       tc.metaVersion,
				armorMetaWrappedDEK:    wrappedMetadata,
				armorMetaIV:            base64.StdEncoding.EncodeToString(iv),
				armorMetaBlockSize:     "4096",
				armorMetaPlaintextSize: "8",
				armorMetaPlaintextSHA:  "unused",
			}
			key := "mismatch/" + tc.name
			backend.objects[key] = &MockObject{Data: body, Metadata: metadata}

			result, err := NewFormatMigrator(backend, "test-bucket", mek, "default", crypto.Version3, []string{"1", "2"}, nil).Migrate(ctx, true, 1)
			if err != nil {
				t.Fatalf("dry-run migration: %v", err)
			}
			if result.FailedObjects != 1 || len(result.Failures) != 1 {
				t.Fatalf("failure result = %d/%d, want exactly one failure", result.FailedObjects, len(result.Failures))
			}
			if result.ProcessedObjects != 0 {
				t.Fatalf("ProcessedObjects = %d, want 0", result.ProcessedObjects)
			}
			if !strings.Contains(result.Failures[0].Reason, "header version disagrees with metadata version") {
				t.Fatalf("failure reason = %q, want header/metadata mismatch", result.Failures[0].Reason)
			}
			if got := backend.objects[key].Data; !bytes.Equal(got, body) {
				t.Fatal("dry run changed the mismatched object")
			}
		})
	}
}
