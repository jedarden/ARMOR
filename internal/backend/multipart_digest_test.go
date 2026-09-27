package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestCombinePartPlaintextSHAsSkipsEmptyParts pins the whole-object plaintext
// digest contract for multipart objects with a zero-byte part (the empty final
// part aws-cli emits). A zero-byte part contributes no stored blocks, so no
// reader — the streaming MultipartDigestAccumulator on GET, or a verifier's
// ComputeMultipartDigest split of the assembled plaintext — can reproduce a
// digest that folds the empty part's SHA in. Complete must therefore skip it,
// or every read of the object trips the plaintext SHA-256 enforcement
// (armor-a2205754).
func TestCombinePartPlaintextSHAsSkipsEmptyParts(t *testing.T) {
	partOne := []byte("part-one payload")
	partOneSHA := sha256.Sum256(partOne)

	combined, err := CombinePartPlaintextSHAs(
		map[int]string{
			1: hex.EncodeToString(partOneSHA[:]),
			2: EmptyPlaintextSHA256Hex,
		},
		[]int{1, 2},
	)
	if err != nil {
		t.Fatalf("CombinePartPlaintextSHAs: %v", err)
	}

	// The empty final part is skipped: the combined digest is the fold over
	// part one alone.
	h := sha256.New()
	h.Write(partOneSHA[:])
	if want := hex.EncodeToString(h.Sum(nil)); combined != want {
		t.Errorf("combined digest = %s, want %s (fold over part 1 only)", combined, want)
	}

	// Reader agreement: the assembled plaintext is exactly part one (an empty
	// final part adds nothing), so the verifier-side reproduction over the
	// full plaintext split at the part boundary must yield the same digest.
	if reproduced := ComputeMultipartDigest(partOne, int64(len(partOne))); reproduced != combined {
		t.Errorf("ComputeMultipartDigest = %s, CombinePartPlaintextSHAs = %s (reader would mismatch)", reproduced, combined)
	}

	// An all-empty upload degenerates to the empty digest — the same value a
	// reader of a zero-byte object reproduces.
	allEmpty, err := CombinePartPlaintextSHAs(
		map[int]string{1: EmptyPlaintextSHA256Hex, 2: EmptyPlaintextSHA256Hex},
		[]int{1, 2},
	)
	if err != nil {
		t.Fatalf("CombinePartPlaintextSHAs all-empty: %v", err)
	}
	if allEmpty != EmptyPlaintextSHA256Hex {
		t.Errorf("all-empty combined digest = %s, want %s", allEmpty, EmptyPlaintextSHA256Hex)
	}

	// The skip must never mask a gap: a part number with no recorded digest
	// still errors.
	if _, err := CombinePartPlaintextSHAs(
		map[int]string{1: hex.EncodeToString(partOneSHA[:])},
		[]int{1, 2},
	); err == nil {
		t.Error("expected missing digest for part 2 to error, got nil")
	}
}
