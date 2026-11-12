package crypto

import "testing"

func TestOpenDecryptorUsesEnvelopeHeader(t *testing.T) {
	dek := make([]byte, 32)
	iv := make([]byte, 16)

	for _, tc := range []struct {
		name    string
		version uint8
	}{
		{name: "v1", version: Version1},
		{name: "v2", version: Version2},
		{name: "v3", version: Version3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			header, err := NewEnvelopeHeaderWithVersion(iv, 0, DefaultBlockSize, [32]byte{}, tc.version)
			if err != nil {
				t.Fatalf("NewEnvelopeHeaderWithVersion() error = %v", err)
			}

			decryptor, err := OpenDecryptor(header, dek)
			if err != nil {
				t.Fatalf("OpenDecryptor() error = %v", err)
			}
			if decryptor.version != tc.version {
				t.Errorf("decryptor version = %d, want %d", decryptor.version, tc.version)
			}
			if decryptor.BlockSize() != DefaultBlockSize {
				t.Errorf("decryptor block size = %d, want %d", decryptor.BlockSize(), DefaultBlockSize)
			}
		})
	}
}

func TestOpenDecryptorRejectsNilHeader(t *testing.T) {
	if _, err := OpenDecryptor(nil, make([]byte, 32)); err != ErrInvalidHeader {
		t.Fatalf("OpenDecryptor(nil, ...) error = %v, want %v", err, ErrInvalidHeader)
	}
}
