// Package backend tests for version metadata handling
package backend

import (
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// TestParseARMORMetadata_VersionParsing tests that version metadata is parsed correctly
func TestParseARMORMetadata_VersionParsing(t *testing.T) {
	tests := []struct {
		name          string
		metadata      map[string]string
		expectedOK    bool
		expectedVersion int
	}{
		{
			name: "version 1 explicitly set",
			metadata: map[string]string{
				"x-amz-meta-armor-version":      "1",
				"x-amz-meta-armor-block-size":    "65536",
				"x-amz-meta-armor-plaintext-size": "1024",
				"x-amz-meta-armor-iv":           "AQIDBAUGBwgJCgsMDQ4PEA==",
				"x-amz-meta-armor-wrapped-dek":  "ARWJxLFyfLFLbK/PRm2b1A==",
			},
			expectedOK:       true,
			expectedVersion:  1,
		},
		{
			name: "version 2 explicitly set",
			metadata: map[string]string{
				"x-amz-meta-armor-version":      "2",
				"x-amz-meta-armor-block-size":    "65536",
				"x-amz-meta-armor-plaintext-size": "1024",
				"x-amz-meta-armor-iv":           "AQIDBAUGBwgJCgsMDQ4PEA==",
				"x-amz-meta-armor-wrapped-dek":  "ARWJxLFyfLFLbK/PRm2b1A==",
			},
			expectedOK:       true,
			expectedVersion:  2,
		},
		{
			name: "missing version defaults to 1",
			metadata: map[string]string{
				"x-amz-meta-armor-block-size":    "65536",
				"x-amz-meta-armor-plaintext-size": "1024",
				"x-amz-meta-armor-iv":           "AQIDBAUGBwgJCgsMDQ4PEA==",
				"x-amz-meta-armor-wrapped-dek":  "ARWJxLFyfLFLbK/PRm2b1A==",
			},
			expectedOK:       true,
			expectedVersion:  1, // Should default to 1
		},
		{
			name: "empty version defaults to 1",
			metadata: map[string]string{
				"x-amz-meta-armor-version":      "",
				"x-amz-meta-armor-block-size":    "65536",
				"x-amz-meta-armor-plaintext-size": "1024",
				"x-amz-meta-armor-iv":           "AQIDBAUGBwgJCgsMDQ4PEA==",
				"x-amz-meta-armor-wrapped-dek":  "ARWJxLFyfLFLbK/PRm2b1A==",
			},
			expectedOK:       true,
			expectedVersion:  1, // Should default to 1
		},
		{
			name: "invalid version defaults to 1",
			metadata: map[string]string{
				"x-amz-meta-armor-version":      "invalid",
				"x-amz-meta-armor-block-size":    "65536",
				"x-amz-meta-armor-plaintext-size": "1024",
				"x-amz-meta-armor-iv":           "AQIDBAUGBwgJCgsMDQ4PEA==",
				"x-amz-meta-armor-wrapped-dek":  "ARWJxLFyfLFLbK/PRm2b1A==",
			},
			expectedOK:       true,
			expectedVersion:  1, // Should default to 1
		},
		{
			name:       "missing required fields returns false",
			metadata:   map[string]string{},
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := ParseARMORMetadata(tt.metadata)
			if ok != tt.expectedOK {
				t.Errorf("ParseARMORMetadata() ok = %v, want %v", ok, tt.expectedOK)
				return
			}
			if tt.expectedOK && meta.Version != tt.expectedVersion {
				t.Errorf("ParseARMORMetadata() version = %d, want %d", meta.Version, tt.expectedVersion)
			}
		})
	}
}

// TestARMORMetadata_ToMetadata_VersionWriting tests that version is written correctly
func TestARMORMetadata_ToMetadata_VersionWriting(t *testing.T) {
	tests := []struct {
		name            string
		metadata        ARMORMetadata
		expectedVersion string
	}{
		{
			name: "version 1",
			metadata: ARMORMetadata{
				Version:       1,
				BlockSize:     65536,
				PlaintextSize: 1024,
				IV:            []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				WrappedDEK:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				PlaintextSHA:  "abcd",
				ETag:          "etag1",
				ContentType:   "text/plain",
			},
			expectedVersion: "1",
		},
		{
			name: "version 2",
			metadata: ARMORMetadata{
				Version:       2,
				BlockSize:     65536,
				PlaintextSize: 1024,
				IV:            []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				WrappedDEK:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				PlaintextSHA:  "abcd",
				ETag:          "etag2",
				ContentType:   "text/plain",
			},
			expectedVersion: "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := tt.metadata.ToMetadata()
			if version := meta["x-amz-meta-armor-version"]; version != tt.expectedVersion {
				t.Errorf("ToMetadata() version = %s, want %s", version, tt.expectedVersion)
			}
		})
	}
}

// TestARMORMetadata_RoundTrip tests that metadata round-trips correctly through parse/write
func TestARMORMetadata_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		metadata ARMORMetadata
	}{
		{
			name: "version 1 metadata",
			metadata: ARMORMetadata{
				Version:       1,
				BlockSize:     65536,
				PlaintextSize: 1024,
				IV:            []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				WrappedDEK:    []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
				PlaintextSHA:  "abcd",
				ETag:          "etag1",
				ContentType:   "text/plain",
				KeyID:         "test-key",
				Compressed:    true,
				CompressionType: CompressionZstd,
			},
		},
		{
			name: "version 2 metadata",
			metadata: ARMORMetadata{
				Version:       2,
				BlockSize:     32768,
				PlaintextSize: 2048,
				IV:            []byte{0x10, 0xf, 0xe, 0xd, 0xc, 0xb, 0xa, 0x9, 0x8, 0x7, 0x6, 0x5, 0x4, 0x3, 0x2, 0x1},
				WrappedDEK:    []byte{0x10, 0xf, 0xe, 0xd, 0xc, 0xb, 0xa, 0x9, 0x8, 0x7, 0x6, 0x5, 0x4, 0x3, 0x2, 0x1},
				PlaintextSHA:  "efgh",
				ETag:          "etag2",
				ContentType:   "application/json",
				KeyID:         "another-key",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to S3 metadata
			s3Meta := tt.metadata.ToMetadata()

			// Parse back
			parsed, ok := ParseARMORMetadata(s3Meta)
			if !ok {
				t.Fatal("ParseARMORMetadata() returned false")
			}

			// Verify version matches
			if parsed.Version != tt.metadata.Version {
				t.Errorf("RoundTrip version = %d, want %d", parsed.Version, tt.metadata.Version)
			}

			// Verify other fields match
			if parsed.BlockSize != tt.metadata.BlockSize {
				t.Errorf("RoundTrip BlockSize = %d, want %d", parsed.BlockSize, tt.metadata.BlockSize)
			}
			if parsed.PlaintextSize != tt.metadata.PlaintextSize {
				t.Errorf("RoundTrip PlaintextSize = %d, want %d", parsed.PlaintextSize, tt.metadata.PlaintextSize)
			}
			if parsed.ContentType != tt.metadata.ContentType {
				t.Errorf("RoundTrip ContentType = %s, want %s", parsed.ContentType, tt.metadata.ContentType)
			}
			if parsed.ETag != tt.metadata.ETag {
				t.Errorf("RoundTrip ETag = %s, want %s", parsed.ETag, tt.metadata.ETag)
			}
			if parsed.KeyID != tt.metadata.KeyID {
				t.Errorf("RoundTrip KeyID = %s, want %s", parsed.KeyID, tt.metadata.KeyID)
			}
			if parsed.Compressed != tt.metadata.Compressed {
				t.Errorf("RoundTrip Compressed = %v, want %v", parsed.Compressed, tt.metadata.Compressed)
			}
			if parsed.CompressionType != tt.metadata.CompressionType {
				t.Errorf("RoundTrip CompressionType = %s, want %s", parsed.CompressionType, tt.metadata.CompressionType)
			}
		})
	}
}

// TestVersionConstants verifies that version constants match expected values
func TestVersionConstants(t *testing.T) {
	if crypto.Version1 != 0x01 {
		t.Errorf("Version1 = %d, want %d", crypto.Version1, 0x01)
	}
	if crypto.Version2 != 0x02 {
		t.Errorf("Version2 = %d, want %d", crypto.Version2, 0x02)
	}
}
