package crypto

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func replayableKeyMaterial() (dek []byte, iv []byte) {
	dek = make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i*7 + 1)
	}
	iv = make([]byte, 16)
	for i := range iv {
		iv[i] = byte(0xF0 + i)
	}
	return dek, iv
}

func replayableSpool(t *testing.T, data []byte) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "plaintext.bin")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write spool: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open spool: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// expectedEnvelope reproduces the pipe path's bytes: envelope header, then
// EncryptStream/EncryptV3Stream output, then the trailer table.
func expectedEnvelope(t *testing.T, dek, iv []byte, blockSize int, plaintext []byte, version uint8) ([]byte, []byte) {
	t.Helper()
	var sha [32]byte
	copy(sha[:], func() []byte { s := sha256.Sum256(plaintext); return s[:] }())
	header, err := NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, sha, version)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	headerBytes, err := header.Encode()
	if err != nil {
		t.Fatalf("header encode: %v", err)
	}
	var out bytes.Buffer
	out.Write(headerBytes)
	enc, err := NewEncryptorWithVersion(dek, iv, blockSize, version)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	if version == Version3 {
		bt, err := enc.EncryptV3Stream(bytes.NewReader(plaintext), &out, int64(len(plaintext)), false)
		if err != nil {
			t.Fatalf("EncryptV3Stream: %v", err)
		}
		tbl, err := bt.Encode()
		if err != nil {
			t.Fatalf("block table encode: %v", err)
		}
		out.Write(tbl)
	} else {
		hmacTable, err := enc.EncryptStream(bytes.NewReader(plaintext), &out, int64(len(plaintext)))
		if err != nil {
			t.Fatalf("EncryptStream: %v", err)
		}
		out.Write(hmacTable)
	}
	return headerBytes, out.Bytes()
}

func replayableEnvelopeFor(t *testing.T, plaintext []byte, blockSize int, version uint8) (*ReplayableEnvelopeReader, []byte) {
	t.Helper()
	dek, iv := replayableKeyMaterial()
	headerBytes, want := expectedEnvelope(t, dek, iv, blockSize, plaintext, version)
	r, err := NewReplayableEnvelopeReader(replayableSpool(t, plaintext), headerBytes, dek, iv, blockSize, int64(len(plaintext)), version)
	if err != nil {
		t.Fatalf("NewReplayableEnvelopeReader: %v", err)
	}
	return r, want
}

func TestReplayableEnvelopeByteIdentity(t *testing.T) {
	const blockSize = 4096
	sizes := []int{1, 4095, 4096, 4097, 12000}
	plaintexts := make([][]byte, len(sizes))
	for i, n := range sizes {
		p := make([]byte, n)
		for j := range p {
			p[j] = byte(j*31 + i)
		}
		plaintexts[i] = p
	}
	for _, version := range []uint8{Version2, Version3} {
		for i, plaintext := range plaintexts {
			r, want := replayableEnvelopeFor(t, plaintext, blockSize, version)
			if got := r.Len(); got != int64(len(want)) {
				t.Fatalf("v%d size %d: Len = %d, want %d", version, sizes[i], got, len(want))
			}
			got, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("v%d size %d: read: %v", version, sizes[i], err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("v%d size %d: stream mismatch (%d vs %d bytes)", version, sizes[i], len(got), len(want))
			}
		}
	}
}

func TestReplayableEnvelopeRewindByteIdentical(t *testing.T) {
	const blockSize = 4096
	plaintext := make([]byte, 3*blockSize+17)
	for i := range plaintext {
		plaintext[i] = byte(i * 13)
	}
	for _, version := range []uint8{Version2, Version3} {
		r, want := replayableEnvelopeFor(t, plaintext, blockSize, version)
		first, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("v%d first pass: %v", version, err)
		}
		second, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("v%d read after EOF: %v", version, err)
		}
		if len(second) != 0 {
			t.Fatalf("v%d read at EOF returned %d bytes", version, len(second))
		}
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("v%d rewind: %v", version, err)
		}
		replay, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("v%d replay: %v", version, err)
		}
		if !bytes.Equal(first, want) || !bytes.Equal(replay, want) {
			t.Fatalf("v%d replay is not byte-identical", version)
		}
	}
}

func TestReplayableEnvelopeRandomSeekWindows(t *testing.T) {
	const blockSize = 4096
	plaintext := make([]byte, 2*blockSize+5)
	for i := range plaintext {
		plaintext[i] = byte(i * 29)
	}
	for _, version := range []uint8{Version2, Version3} {
		r, want := replayableEnvelopeFor(t, plaintext, blockSize, version)
		for _, off := range []int64{0, 1, 33, blockSize - 1, blockSize, int64(len(want)) - 1, int64(len(want)) - 10} {
			for _, span := range []int{1, 7, 64, 1000} {
				if off+int64(span) > int64(len(want)) {
					continue
				}
				if _, err := r.Seek(off, io.SeekStart); err != nil {
					t.Fatalf("v%d seek %d: %v", version, off, err)
				}
				buf := make([]byte, span)
				if _, err := io.ReadFull(r, buf); err != nil {
					t.Fatalf("v%d read at %d span %d: %v", version, off, span, err)
				}
				if !bytes.Equal(buf, want[off:off+int64(span)]) {
					t.Fatalf("v%d window at %d span %d mismatch", version, off, span)
				}
			}
		}
		// Trailer-first: seek straight into the trailer region.
		if _, err := r.Seek(int64(len(want))-10, io.SeekStart); err != nil {
			t.Fatalf("v%d trailer seek: %v", version, err)
		}
		tail := make([]byte, 10)
		if _, err := io.ReadFull(r, tail); err != nil {
			t.Fatalf("v%d trailer read: %v", version, err)
		}
		if !bytes.Equal(tail, want[int64(len(want))-10:]) {
			t.Fatalf("v%d trailer-first read mismatch", version)
		}
	}
}

func TestReplayableEnvelopeSingleByteReads(t *testing.T) {
	const blockSize = 4096
	plaintext := []byte("armor replayable envelope single byte reads")
	for _, version := range []uint8{Version2, Version3} {
		r, want := replayableEnvelopeFor(t, plaintext, blockSize, version)
		var got []byte
		one := make([]byte, 1)
		for {
			n, err := r.Read(one)
			got = append(got, one[:n]...)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("v%d single-byte read: %v", version, err)
			}
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("v%d single-byte stream mismatch", version)
		}
	}
}

func TestReplayableEnvelopeSeekWhence(t *testing.T) {
	const blockSize = 4096
	plaintext := make([]byte, blockSize+3)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}
	r, want := replayableEnvelopeFor(t, plaintext, blockSize, Version3)
	end, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatalf("seek end: %v", err)
	}
	if end != int64(len(want)) {
		t.Fatalf("Seek(0, End) = %d, want %d", end, len(want))
	}
	tail, err := r.Seek(-5, io.SeekEnd)
	if err != nil || tail != int64(len(want)-5) {
		t.Fatalf("Seek(-5, End) = %d, %v", tail, err)
	}
	buf := make([]byte, 5)
	if _, err := io.ReadFull(r, buf); err != nil {
		t.Fatalf("tail read: %v", err)
	}
	if !bytes.Equal(buf, want[int64(len(want)-5):]) {
		t.Fatalf("Seek(-5, End) window mismatch")
	}
	cur, err := r.Seek(-2, io.SeekCurrent)
	if err != nil || cur != int64(len(want)-2) {
		t.Fatalf("Seek(-2, Current) = %d, %v", cur, err)
	}
	if _, err := r.Seek(-1, io.SeekStart); err == nil {
		t.Fatalf("negative seek accepted")
	}
}

func TestReplayableEnvelopeEmptyInput(t *testing.T) {
	dek, iv := replayableKeyMaterial()
	headerBytes, v3Want := expectedEnvelope(t, dek, iv, 4096, nil, Version3)
	v3, err := NewReplayableEnvelopeReader(replayableSpool(t, nil), headerBytes, dek, iv, 4096, 0, Version3)
	if err != nil {
		t.Fatalf("v3 empty envelope: %v", err)
	}
	got, err := io.ReadAll(v3)
	if err != nil {
		t.Fatalf("v3 empty read: %v", err)
	}
	if !bytes.Equal(got, v3Want) {
		t.Fatalf("v3 empty envelope mismatch (%d vs %d bytes)", len(got), len(v3Want))
	}
	var want []byte
	headerBytes, want = expectedEnvelope(t, dek, iv, 4096, nil, Version2)
	r, err := NewReplayableEnvelopeReader(replayableSpool(t, nil), headerBytes, dek, iv, 4096, 0, Version2)
	if err != nil {
		t.Fatalf("v2 empty envelope: %v", err)
	}
	got, err = io.ReadAll(r)
	if err != nil {
		t.Fatalf("v2 empty read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("v2 empty envelope mismatch (%d vs %d bytes)", len(got), len(want))
	}
}

func TestReplayableEnvelopePlaintextReadAccounting(t *testing.T) {
	const blockSize = 4096
	plaintext := make([]byte, blockSize+11)
	for i := range plaintext {
		plaintext[i] = byte(i * 5)
	}
	r, _ := replayableEnvelopeFor(t, plaintext, blockSize, Version3)
	if _, err := io.Copy(io.Discard, r); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := r.PlaintextBytesRead(); got != int64(len(plaintext)) {
		t.Fatalf("PlaintextBytesRead = %d, want %d for one forward pass", got, len(plaintext))
	}
}

func TestReplayableEnvelopeRejectsInvalidSourceAndShortSpool(t *testing.T) {
	dek, iv := replayableKeyMaterial()
	for _, tc := range []struct {
		name      string
		source    io.ReadSeeker
		blockSize int
		plainSize int64
	}{
		{"nil source", nil, 4096, 1},
		{"zero block size", bytes.NewReader(nil), 0, 1},
		{"negative size", bytes.NewReader(nil), 4096, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewReplayableEnvelopeReader(tc.source, nil, dek, iv, tc.blockSize, tc.plainSize, Version3); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
	r, err := NewReplayableEnvelopeReader(bytes.NewReader([]byte("short")), []byte("header"), dek, iv, 4096, 10, Version3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(r); err == nil {
		t.Fatal("short plaintext spool produced a complete encrypted envelope")
	}
}

// TestReplayablePartReaderMatchesEncryptPartV3 pins the part stream to the
// buffered multipart reference: the same (part, block) counters, ciphertext
// bytes, per-block HMACs, byte-identical replay after a rewind, and backfill
// of blocks a partial consumer never reached (armor-cb2b0731).
func TestReplayablePartReaderMatchesEncryptPartV3(t *testing.T) {
	dek, iv := replayableKeyMaterial()
	const blockSize = 4096
	cases := []struct {
		size       int64
		partNumber uint16
	}{
		{0, 1}, {1, 1}, {blockSize - 1, 2}, {blockSize, 3},
		{3*blockSize + 7, 10000}, {2*blockSize + 512, 513},
	}
	for _, tc := range cases {
		plaintext := make([]byte, tc.size)
		for i := range plaintext {
			plaintext[i] = byte(i*31 + int(tc.partNumber))
		}
		refCiphertext, refHMACs, err := EncryptPartV3(dek, iv, tc.partNumber, blockSize, plaintext)
		if err != nil {
			t.Fatalf("size %d part %d: reference encrypt: %v", tc.size, tc.partNumber, err)
		}

		spool := replayableSpool(t, plaintext)
		reader, err := NewReplayablePartReader(spool, dek, iv, blockSize, tc.size, tc.partNumber)
		if err != nil {
			t.Fatalf("size %d part %d: %v", tc.size, tc.partNumber, err)
		}
		if got := reader.Len(); got != tc.size {
			t.Fatalf("size %d part %d: Len()=%d want %d", tc.size, tc.partNumber, got, tc.size)
		}
		first, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("size %d part %d: read: %v", tc.size, tc.partNumber, err)
		}
		if !bytes.Equal(first, refCiphertext) {
			t.Fatalf("size %d part %d: ciphertext differs from EncryptPartV3", tc.size, tc.partNumber)
		}

		// A backend retry rewinds and must replay byte-identical ciphertext.
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			t.Fatalf("size %d part %d: seek: %v", tc.size, tc.partNumber, err)
		}
		second, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("size %d part %d: replay read: %v", tc.size, tc.partNumber, err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("size %d part %d: replayed ciphertext differs from first pass", tc.size, tc.partNumber)
		}

		hmacs, err := reader.PartBlockHMACs()
		if err != nil {
			t.Fatalf("size %d part %d: PartBlockHMACs: %v", tc.size, tc.partNumber, err)
		}
		if len(hmacs) != len(refHMACs)/HMACSize {
			t.Fatalf("size %d part %d: %d HMACs, want %d", tc.size, tc.partNumber, len(hmacs), len(refHMACs)/HMACSize)
		}
		for b, mac := range hmacs {
			if !bytes.Equal(mac, refHMACs[b*HMACSize:(b+1)*HMACSize]) {
				t.Fatalf("size %d part %d: block %d HMAC differs from reference", tc.size, tc.partNumber, b)
			}
		}
	}
}

// TestReplayablePartReaderBackfillsUnreadBlocks verifies that a consumer
// which stops mid-stream still yields the complete HMAC set — the missing
// blocks are re-encrypted from the spool, deterministically.
func TestReplayablePartReaderBackfillsUnreadBlocks(t *testing.T) {
	dek, iv := replayableKeyMaterial()
	const blockSize = 4096
	plaintext := make([]byte, 5*blockSize+3)
	for i := range plaintext {
		plaintext[i] = byte(i)
	}
	_, refHMACs, err := EncryptPartV3(dek, iv, 7, blockSize, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	spool := replayableSpool(t, plaintext)
	reader, err := NewReplayablePartReader(spool, dek, iv, blockSize, int64(len(plaintext)), 7)
	if err != nil {
		t.Fatal(err)
	}
	// Consume only the first block and a half, like an interrupted transfer.
	buf := make([]byte, blockSize+blockSize/2)
	if _, err := io.ReadFull(reader, buf); err != nil {
		t.Fatal(err)
	}
	hmacs, err := reader.PartBlockHMACs()
	if err != nil {
		t.Fatal(err)
	}
	if len(hmacs) != len(refHMACs)/HMACSize {
		t.Fatalf("backfill produced %d HMACs, want %d", len(hmacs), len(refHMACs)/HMACSize)
	}
	for b, mac := range hmacs {
		if !bytes.Equal(mac, refHMACs[b*HMACSize:(b+1)*HMACSize]) {
			t.Fatalf("backfilled block %d HMAC differs from reference", b)
		}
	}
}

// TestReplayablePartReaderRejectsInvalidArguments covers the constructor's
// contract: no plaintext source, non-positive block size, negative size and
// part number zero are rejected.
func TestReplayablePartReaderRejectsInvalidArguments(t *testing.T) {
	dek, iv := replayableKeyMaterial()
	spool := replayableSpool(t, []byte{1, 2, 3})
	if _, err := NewReplayablePartReader(nil, dek, iv, 4096, 3, 1); err == nil {
		t.Fatal("nil source accepted")
	}
	if _, err := NewReplayablePartReader(spool, dek, iv, 0, 3, 1); err == nil {
		t.Fatal("zero block size accepted")
	}
	if _, err := NewReplayablePartReader(spool, dek, iv, 4096, -1, 1); err == nil {
		t.Fatal("negative size accepted")
	}
	if _, err := NewReplayablePartReader(spool, dek, iv, 4096, 3, 0); err == nil {
		t.Fatal("part number 0 accepted")
	}
}
