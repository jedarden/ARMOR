package canary

import (
	"fmt"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// encryptCanaryEnvelope uses the same envelope layout as a single PUT.
func encryptCanaryEnvelope(plaintext, dek, iv []byte, blockSize int, version uint8) ([]byte, error) {
	sha := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, sha, version)
	if err != nil {
		return nil, fmt.Errorf("create header: %w", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("encode header: %w", err)
	}
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, blockSize, version)
	if err != nil {
		return nil, fmt.Errorf("create encryptor: %w", err)
	}
	var encrypted, trailer []byte
	if version == crypto.Version3 {
		var table *crypto.BlockTable
		encrypted, table, err = encryptor.EncryptV3(plaintext, false)
		if err == nil {
			trailer, err = table.Encode()
		}
	} else {
		encrypted, trailer, err = encryptor.Encrypt(plaintext)
	}
	if err != nil {
		return nil, fmt.Errorf("encrypt canary: %w", err)
	}
	envelope := make([]byte, 0, len(headerBytes)+len(encrypted)+len(trailer))
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, trailer...)
	return envelope, nil
}

// decryptCanaryEnvelope follows the normal single-object read format: the
// stored metadata identifies the key, the header identifies the cipher, and
// v3 reads its trailer block table before authenticating/decrypting blocks.
func decryptCanaryEnvelope(envelope []byte, metadata map[string]string, mek []byte) ([]byte, error) {
	meta, ok := backend.ParseARMORMetadata(metadata)
	if !ok {
		return nil, fmt.Errorf("canary ARMOR metadata is missing")
	}
	header, err := crypto.DecodeHeader(envelope)
	if err != nil {
		return nil, fmt.Errorf("decode header: %w", err)
	}
	if int(header.Version) != meta.Version || header.BlockSize() != meta.BlockSize ||
		int64(header.PlaintextSize) != meta.PlaintextSize {
		return nil, fmt.Errorf("canary header and metadata disagree")
	}
	dek, err := crypto.UnwrapDEK(mek, meta.WrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	decryptor, err := crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
	if err != nil {
		return nil, fmt.Errorf("create decryptor: %w", err)
	}
	dataStart := crypto.HeaderSize
	if len(envelope) < dataStart {
		return nil, fmt.Errorf("canary envelope is shorter than its header")
	}
	var plaintext []byte
	if header.Version == crypto.Version3 {
		blockCount := header.BlockCount()
		trailerSize := int(blockCount) * crypto.BlockTableEntrySize
		dataEnd := len(envelope) - trailerSize
		if dataEnd < dataStart {
			return nil, fmt.Errorf("canary envelope is shorter than its v3 block table")
		}
		table, err := crypto.DecodeBlockTable(envelope[dataEnd:], header.BlockSize(), blockCount)
		if err != nil {
			return nil, fmt.Errorf("decode v3 block table: %w", err)
		}
		plaintext, err = decryptor.DecryptV3(envelope[dataStart:dataEnd], 0, table)
		if err != nil {
			return nil, fmt.Errorf("decrypt v3 canary: %w", err)
		}
	} else {
		dataEnd := dataStart + int(meta.PlaintextSize)
		hmacEnd := dataEnd + int(header.BlockCount())*crypto.HMACSize
		if len(envelope) != hmacEnd {
			return nil, fmt.Errorf("canary envelope has incorrect v2 length")
		}
		plaintext, err = decryptor.Decrypt(envelope[dataStart:dataEnd], envelope[dataEnd:hmacEnd])
		if err != nil {
			return nil, fmt.Errorf("decrypt v2 canary: %w", err)
		}
	}
	if int64(len(plaintext)) != meta.PlaintextSize {
		return nil, fmt.Errorf("canary plaintext size mismatch")
	}
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		return nil, fmt.Errorf("canary plaintext SHA verification failed: %w", err)
	}
	return plaintext, nil
}
