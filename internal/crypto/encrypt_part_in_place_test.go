package crypto

import (
	"bytes"
	"fmt"
	"testing"
)

func TestEncryptPartV3InPlaceMatchesSeparateOutput(t *testing.T) {
	dek := bytes.Repeat([]byte{0x39}, 32)
	iv := bytes.Repeat([]byte{0xa7}, 16)
	for _, size := range []int{0, 1, 65535, 65536, 65537, 2*65536 + 17} {
		for _, partNumber := range []uint16{1, 10000} {
			t.Run(fmt.Sprintf("size-%d-part-%d", size, partNumber), func(t *testing.T) {
				plain := make([]byte, size)
				for i := range plain {
					plain[i] = byte(i*17 + 3)
				}
				original := bytes.Clone(plain)
				wantCipher, wantHMACs, err := EncryptPartV3(dek, iv, partNumber, 65536, plain)
				if err != nil {
					t.Fatal(err)
				}
				// Check the part layout against independently encrypted blocks,
				// including the partial final block and the part namespace.
				var blockCipher, blockHMACs []byte
				for start, block := 0, uint32(0); start < len(original); start, block = start+65536, block+1 {
					end := start + 65536
					if end > len(original) {
						end = len(original)
					}
					ciphertext, hmac, err := EncryptBlockV3(dek, iv, partNumber, block, original[start:end], 65536)
					if err != nil {
						t.Fatal(err)
					}
					blockCipher = append(blockCipher, ciphertext...)
					blockHMACs = append(blockHMACs, hmac...)
				}
				if !bytes.Equal(wantCipher, blockCipher) || !bytes.Equal(wantHMACs, blockHMACs) {
					t.Fatal("part ciphertext or HMAC table differs from per-block wire format")
				}
				if !bytes.Equal(plain, original) {
					t.Fatal("separate-output encryption changed its input")
				}
				gotHMACs, err := EncryptPartV3InPlace(dek, iv, partNumber, 65536, plain)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(plain, wantCipher) || !bytes.Equal(gotHMACs, wantHMACs) {
					t.Fatal("in-place ciphertext or HMAC table differs")
				}
			})
		}
	}
}
