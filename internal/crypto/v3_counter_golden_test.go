package crypto

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMakeV3CounterGolden pins the wire representation of the V3 counter.
// These are independent known answers for the ADR-005 layout:
// IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock), with all
// integer fields encoded big-endian.
func TestMakeV3CounterGolden(t *testing.T) {
	iv, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	require.NoError(t, err)

	tests := []struct {
		name     string
		part     uint16
		block    uint32
		aesBlock uint16
		want     string
	}{
		{
			name:     "single-put origin",
			part:     0,
			block:    0,
			aesBlock: 0,
			want:     "00112233445566770000000000000000",
		},
		{
			name:     "mixed fields",
			part:     0x1234,
			block:    0x89abcdef,
			aesBlock: 0x4567,
			want:     "0011223344556677123489abcdef4567",
		},
		{
			name:     "field carry boundaries",
			part:     0x0102,
			block:    0x03040506,
			aesBlock: 0x0708,
			want:     "00112233445566770102030405060708",
		},
		{
			name:     "maximum fields",
			part:     0xffff,
			block:    0xffffffff,
			aesBlock: 0xffff,
			want:     "0011223344556677ffffffffffffffff",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MakeV3Counter(iv, tt.part, tt.block, tt.aesBlock)
			require.Equal(t, tt.want, hex.EncodeToString(got))
		})
	}
}

// TestMakeV3CounterBoundaryFields verifies both the field widths and the
// namespace boundaries used to keep parts and blocks from colliding. The
// values include the first, carry-adjacent, and maximum values for each
// encoded field.
func TestMakeV3CounterBoundaryFields(t *testing.T) {
	iv, err := hex.DecodeString("fedcba98765432100123456789abcdef")
	require.NoError(t, err)

	parts := []uint16{0, 1, 0xff, 0x100, 0xffff}
	blocks := []uint32{0, 1, 0xff, 0x100, 0xffffffff}
	aesBlocks := []uint16{0, 1, 0xff, 0x100, 0xffff}

	seen := make(map[string]struct{}, len(parts)*len(blocks)*len(aesBlocks))
	for _, part := range parts {
		for _, block := range blocks {
			for _, aesBlock := range aesBlocks {
				counter := MakeV3Counter(iv, part, block, aesBlock)
				require.Len(t, counter, 16)
				require.Equal(t, iv[:8], counter[:8])
				require.Equal(t, part, binary.BigEndian.Uint16(counter[8:10]))
				require.Equal(t, block, binary.BigEndian.Uint32(counter[10:14]))
				require.Equal(t, aesBlock, binary.BigEndian.Uint16(counter[14:16]))

				key := string(counter)
				if _, duplicate := seen[key]; duplicate {
					t.Fatalf("counter collision for part=%d block=%d aesBlock=%d", part, block, aesBlock)
				}
				seen[key] = struct{}{}
			}
		}
	}

	require.Len(t, seen, len(parts)*len(blocks)*len(aesBlocks))
}

// TestMakeV3CounterNoCollisionsAcrossPartsAndBlocks keeps the AES-block
// position fixed and checks the independent part/block namespaces directly.
func TestMakeV3CounterNoCollisionsAcrossPartsAndBlocks(t *testing.T) {
	iv := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	parts := []uint16{0, 1, 0xffff}
	blocks := []uint32{0, 1, 0xffffffff}
	const aesBlock = uint16(0x55aa)

	seen := make(map[string]string, len(parts)*len(blocks))
	for _, part := range parts {
		for _, block := range blocks {
			counter := string(MakeV3Counter(iv, part, block, aesBlock))
			coordinates := fmt.Sprintf("part=%d block=%d", part, block)
			if previous, duplicate := seen[counter]; duplicate {
				t.Fatalf("counter collision: %s and %s", previous, coordinates)
			}
			seen[counter] = coordinates
		}
	}

	require.Len(t, seen, len(parts)*len(blocks))
}
