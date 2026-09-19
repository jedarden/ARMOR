package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// v3RangeFixture is a fully encrypted v3 multipart object backed by an
// in-memory ciphertext, with a sidecar cache entry built the same way the
// production cache builds them.
type v3RangeFixture struct {
	dek, iv    []byte
	blockSize  int
	sidecar    *backend.HMACTableSidecarV3
	cache      *backend.MultipartSidecarCache
	entry      *backend.MultipartSidecarEntry
	ciphertext []byte
	plaintext  []byte
	partPlain  [][]byte
	partCipher [][]byte
}

const v3RangeETag = "etag-v3-range-test"

// newV3RangeFixture encrypts blocksPerPart[i] blocks (blockSize bytes each;
// the final block of the final part is 3/4 size) for each part and registers
// the sidecar in a fresh cache.
func newV3RangeFixture(t *testing.T, blocksPerPart []int, blockSize int) *v3RangeFixture {
	t.Helper()

	dek := sha256.Sum256([]byte("armor-v3-range-fixture-dek"))
	iv := sha256.Sum256([]byte("armor-v3-range-fixture-iv"))
	fx := &v3RangeFixture{dek: dek[:], iv: iv[:16], blockSize: blockSize}
	fx.cache = backend.NewMultipartSidecarCache(10, 60)

	sidecar := &backend.HMACTableSidecarV3{Version: 3, BlockSize: blockSize}

	for partIdx, blockCount := range blocksPerPart {
		partNum := partIdx + 1

		var partPlain, partCipher []byte
		sidecarPart := backend.HMACPartV3{N: partNum}

		for blockIdx := 0; blockIdx < blockCount; blockIdx++ {
			isLast := partIdx == len(blocksPerPart)-1 && blockIdx == blockCount-1
			blockPlain := make([]byte, blockSize)
			if isLast {
				blockPlain = blockPlain[:blockSize-blockSize/4]
			}
			// Deterministic provenance-carrying plaintext
			for i := range blockPlain {
				blockPlain[i] = byte(len(fx.plaintext) + i)
			}

			ciphertext, hmacValue, err := crypto.EncryptBlockV3(fx.dek, fx.iv, uint16(partNum), uint32(blockIdx), blockPlain, blockSize)
			if err != nil {
				t.Fatalf("EncryptBlockV3(part %d, block %d): %v", partNum, blockIdx, err)
			}

			clen := make([]byte, 4)
			binary.BigEndian.PutUint32(clen, uint32(len(ciphertext)))
			sidecarPart.Blocks = append(sidecarPart.Blocks, []string{
				base64.StdEncoding.EncodeToString(hmacValue),
				base64.StdEncoding.EncodeToString(clen),
			})

			partPlain = append(partPlain, blockPlain...)
			partCipher = append(partCipher, ciphertext...)
			fx.plaintext = append(fx.plaintext, blockPlain...)
			fx.ciphertext = append(fx.ciphertext, ciphertext...)
		}

		sidecarPart.PlaintextLen = int64(len(partPlain))
		sidecarPart.CiphertextLen = int64(len(partCipher))
		sidecar.Parts = append(sidecar.Parts, sidecarPart)
		fx.partPlain = append(fx.partPlain, partPlain)
		fx.partCipher = append(fx.partCipher, partCipher)
	}

	fx.sidecar = sidecar
	if err := fx.cache.Set("bucket", "prefixed/key", v3RangeETag, sidecar); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}
	fx.entry = fx.cache.Get("bucket", "prefixed/key", v3RangeETag)
	if fx.entry == nil {
		t.Fatal("sidecar cache entry missing after Set")
	}
	return fx
}

// rangeBackend serves the fixture ciphertext from memory and records every
// ranged call the handler makes.
type rangeBackend struct {
	*backend.NilBackend
	mu    sync.Mutex
	data  []byte
	calls []backendRangeCall
}

type backendRangeCall struct {
	offset int64
	length int64
}

func newRangeBackend(data []byte) *rangeBackend {
	return &rangeBackend{NilBackend: backend.NewNilBackend(), data: data}
}

func (b *rangeBackend) recordedCalls() []backendRangeCall {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]backendRangeCall(nil), b.calls...)
}

func (b *rangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := b.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (b *rangeBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	b.mu.Lock()
	b.calls = append(b.calls, backendRangeCall{offset: offset, length: length})
	b.mu.Unlock()

	if offset < 0 || offset+length > int64(len(b.data)) {
		return nil, nil, fmt.Errorf("range %d+%d out of bounds (have %d bytes)", offset, length, len(b.data))
	}
	return io.NopCloser(bytes.NewReader(b.data[offset : offset+length])), map[string]string{}, nil
}

// offsetCaptureBackend records requested offsets without serving data, for
// asserting the addresses computed for very large objects.
type offsetCaptureBackend struct {
	*backend.NilBackend
	mu      sync.Mutex
	offsets []int64
	lengths []int64
}

func (b *offsetCaptureBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	b.mu.Lock()
	b.offsets = append(b.offsets, offset)
	b.lengths = append(b.lengths, length)
	b.mu.Unlock()
	return io.NopCloser(bytes.NewReader(nil)), nil
}

func (b *offsetCaptureBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	body, err := b.GetRange(ctx, bucket, key, offset, length)
	return body, nil, err
}

func (b *offsetCaptureBackend) recorded() ([]int64, []int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]int64(nil), b.offsets...), append([]int64(nil), b.lengths...)
}

// runV3RangeHandler invokes handleV3MultipartRangeRequest against the fixture.
func runV3RangeHandler(t *testing.T, fx *v3RangeFixture, beb backend.Backend, rangeStart, rangeEnd int64) *httptest.ResponseRecorder {
	t.Helper()

	h := &Handlers{backend: beb, multipartSidecarCache: fx.cache}
	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   fx.blockSize,
		IV:          fx.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}

	r := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	h.handleV3MultipartRangeRequest(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(fx.plaintext)), time.Now(), rangeStart, rangeEnd)
	return w
}

// TestV3MultipartRangeOneRangedRequestPerPart verifies that a range spanning
// whole parts is fetched with one ranged request per part, not one per block
// (armor-817d9d92: one request per 64KiB block made a 64MiB range need 1024
// sequential origin round trips and blow the client's read timeout).
func TestV3MultipartRangeOneRangedRequestPerPart(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4, 4}, 128)
	be := newRangeBackend(fx.ciphertext)

	w := runV3RangeHandler(t, fx, be, 0, int64(len(fx.plaintext))-1)
	if w.Code != 206 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != string(fx.plaintext) {
		t.Fatalf("plaintext mismatch: got %d bytes, want %d", len(got), len(fx.plaintext))
	}

	calls := be.recordedCalls()
	if len(calls) != 3 {
		t.Fatalf("ranged requests = %d (%+v), want 3 — one per part", len(calls), calls)
	}
	wantOffsets := []int64{0, int64(len(fx.partCipher[0])), int64(len(fx.partCipher[0]) + len(fx.partCipher[1]))}
	for i, call := range calls {
		if call.offset != wantOffsets[i] {
			t.Errorf("call %d offset = %d, want %d", i, call.offset, wantOffsets[i])
		}
		wantLen := int64(len(fx.partCipher[i]))
		if call.length != wantLen {
			t.Errorf("call %d length = %d, want %d", i, call.length, wantLen)
		}
	}

	wantCR := fmt.Sprintf("bytes 0-%d/%d", len(fx.plaintext)-1, len(fx.plaintext))
	if got := w.Header().Get("Content-Range"); got != wantCR {
		t.Errorf("Content-Range = %q, want %q", got, wantCR)
	}
}

// TestV3MultipartRangeMidBlockCrossPartSlice verifies the plaintext slicing
// for a range that starts mid-block and ends mid-block across a part
// boundary: FirstBlockOffset must carve the correct bytes out of the first
// fetched block.
func TestV3MultipartRangeMidBlockCrossPartSlice(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4}, 128)
	be := newRangeBackend(fx.ciphertext)

	const start = 100
	const end = 128*4 + 64 // part 0 block 3 mid-block .. part 1 block 0 mid-block
	w := runV3RangeHandler(t, fx, be, start, end)
	if w.Code != 206 {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	want := string(fx.plaintext[start : end+1])
	if got := w.Body.String(); got != want {
		t.Fatalf("body mismatch: got %d bytes, want %d", len(got), len(want))
	}

	calls := be.recordedCalls()
	if len(calls) != 2 {
		t.Fatalf("ranged requests = %d (%+v), want 2 — one per part", len(calls), calls)
	}
}

// TestV3MultipartRangeContiguousRunsMapping verifies mapV3MultipartRange
// emits one run per intersecting part, spanning that part's full block range.
func TestV3MultipartRangeContiguousRunsMapping(t *testing.T) {
	fx := newV3RangeFixture(t, []int{8, 8, 8}, 64)
	h := &Handlers{}

	// Range wholly inside part 0 (0-based)
	info, err := h.mapV3MultipartRange(fx.entry, 70, 130, fx.blockSize)
	if err != nil {
		t.Fatalf("mapV3MultipartRange: %v", err)
	}
	if len(info.BlockRuns) != 1 {
		t.Fatalf("runs = %+v, want 1", info.BlockRuns)
	}
	run := info.BlockRuns[0]
	if run.PartIdx != 0 || run.StartBlock != 1 || run.EndBlock != 2 {
		t.Errorf("run = %+v, want part 0 blocks 1-2", run)
	}
	if info.FirstBlockOffset != 6 {
		t.Errorf("FirstBlockOffset = %d, want 6", info.FirstBlockOffset)
	}

	// Range spanning all three parts
	info, err = h.mapV3MultipartRange(fx.entry, 0, int64(len(fx.plaintext))-1, fx.blockSize)
	if err != nil {
		t.Fatalf("mapV3MultipartRange: %v", err)
	}
	if len(info.BlockRuns) != 3 {
		t.Fatalf("runs = %+v, want 3", info.BlockRuns)
	}
	for i, want := range []V3BlockRun{{0, 0, 7}, {1, 0, 7}, {2, 0, 7}} {
		if info.BlockRuns[i] != want {
			t.Errorf("run %d = %+v, want %+v", i, info.BlockRuns[i], want)
		}
	}
}

// TestV3MultipartRunCiphertextOffsetBeyondUint32 pins the part-offset
// arithmetic at cumulative ciphertext sizes past 2^32: the previous uint32
// accumulator silently wrapped, so any block in part 128+ of a large multipart
// object was fetched from the wrong address (armor-817d9d92).
func TestV3MultipartRunCiphertextOffsetBeyondUint32(t *testing.T) {
	const partCiphertextLen = int64(1) << 25 // 32MiB per part
	const partCount = 130                    // cumulative 4160MiB > 2^32

	sidecar := &backend.HMACTableSidecarV3{Version: 3, BlockSize: 65536}
	clen := make([]byte, 4)
	binary.BigEndian.PutUint32(clen, uint32(partCiphertextLen))
	block := []string{
		base64.StdEncoding.EncodeToString(make([]byte, 32)),
		base64.StdEncoding.EncodeToString(clen),
	}
	for p := 1; p <= partCount; p++ {
		sidecar.Parts = append(sidecar.Parts, backend.HMACPartV3{
			N:             p,
			PlaintextLen:  partCiphertextLen,
			CiphertextLen: partCiphertextLen,
			Blocks:        [][]string{block},
		})
	}

	cache := backend.NewMultipartSidecarCache(10, 60)
	if err := cache.Set("bucket", "prefixed/key", v3RangeETag, sidecar); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}
	entry := cache.Get("bucket", "prefixed/key", v3RangeETag)
	if entry == nil {
		t.Fatal("sidecar cache entry missing after Set")
	}

	be := &offsetCaptureBackend{NilBackend: backend.NewNilBackend()}
	h := &Handlers{backend: be}

	for _, partIdx := range []int{0, 63, 127, 128, 129} {
		_, err := h.fetchV3MultipartBlockRun(context.Background(), "bucket", "prefixed/key", entry, V3BlockRun{PartIdx: partIdx, StartBlock: 0, EndBlock: 0})
		if err != nil {
			t.Fatalf("fetchV3MultipartBlockRun(part %d): %v", partIdx, err)
		}
	}

	offsets, lengths := be.recorded()
	wantOffsets := []int64{0, 63 << 25, 127 << 25, 128 << 25, 129 << 25}
	for i, want := range wantOffsets {
		if offsets[i] != want {
			t.Errorf("part %d fetch offset = %d, want %d", []int{0, 63, 127, 128, 129}[i], offsets[i], want)
		}
		if lengths[i] != partCiphertextLen {
			t.Errorf("part fetch length = %d, want %d", lengths[i], partCiphertextLen)
		}
	}

	// The uint32 wrap this test pins: part 128 starts exactly at 2^32, which
	// truncated to 0 under the previous accumulator.
	if got := offsets[3]; got != 1<<32 {
		t.Errorf("part 128 offset = %d, want 2^32 = %d", got, int64(1)<<32)
	}
}

// TestV3MultipartRangeShortRunReadIsRejected verifies a truncated run body
// surfaces as a 500 rather than silently decrypting short data.
func TestV3MultipartRangeShortRunReadIsRejected(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4, 4}, 128)

	be := newRangeBackend(fx.ciphertext[:len(fx.ciphertext)-16]) // truncate last block
	w := runV3RangeHandler(t, fx, be, 0, int64(len(fx.plaintext))-1)
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "InternalError") {
		t.Errorf("error body missing InternalError code: %s", w.Body.String())
	}
}

// TestV3MultipartRangeInvalidBlockRunRejected verifies run validation.
func TestV3MultipartRangeInvalidBlockRunRejected(t *testing.T) {
	fx := newV3RangeFixture(t, []int{4}, 128)
	be := &offsetCaptureBackend{NilBackend: backend.NewNilBackend()}
	h := &Handlers{backend: be}

	if _, err := h.fetchV3MultipartBlockRun(context.Background(), "bucket", "prefixed/key", fx.entry, V3BlockRun{PartIdx: 0, StartBlock: 2, EndBlock: 9}); err == nil {
		t.Error("expected error for run past last block, got nil")
	}
	if _, err := h.fetchV3MultipartBlockRun(context.Background(), "bucket", "prefixed/key", fx.entry, V3BlockRun{PartIdx: 5, StartBlock: 0, EndBlock: 0}); err == nil {
		t.Error("expected error for invalid part index, got nil")
	}
	if _, err := h.fetchV3MultipartBlockRun(context.Background(), "bucket", "prefixed/key", fx.entry, V3BlockRun{PartIdx: 0, StartBlock: 3, EndBlock: 1}); err == nil {
		t.Error("expected error for inverted block range, got nil")
	}
}
