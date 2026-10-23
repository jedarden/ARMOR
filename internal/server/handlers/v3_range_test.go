package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// v3SinglePUTObject is an encrypted v3 single-PUT object laid out the way the
// server stores them: [header][blocks...][trailer block table].
type v3SinglePUTObject struct {
	plaintext  []byte
	stored     []byte
	blockSize  int
	table      *crypto.BlockTable
	rawLens    []uint32
	dek, iv    []byte
	cipherSpan [2]int64 // ciphertext-section bounds of the stored object
}

// newV3SinglePUTObject encrypts blocks of blockSize bytes (final block short)
// from deterministic plaintext; blocks whose index satisfies compressMod > 0
// and index%compressMod == 0 are stored compressed.
func newV3SinglePUTObject(t *testing.T, blockCount, blockSize, compressMod int) *v3SinglePUTObject {
	t.Helper()

	obj := &v3SinglePUTObject{blockSize: blockSize}
	dek := sha256.Sum256([]byte("armor-v3-singleput-range-dek"))
	iv := sha256.Sum256([]byte("armor-v3-singleput-range-iv"))
	obj.dek, obj.iv = dek[:], iv[:16]

	obj.table = crypto.NewBlockTable(blockSize, blockCount)
	var blocks []byte
	for i := 0; i < blockCount; i++ {
		blockPlain := make([]byte, blockSize)
		if i == blockCount-1 {
			blockPlain = blockPlain[:blockSize-blockSize/4] // short tail
		}
		for j := range blockPlain {
			blockPlain[j] = byte(len(obj.plaintext) + j)
		}
		obj.plaintext = append(obj.plaintext, blockPlain...)

		payload := blockPlain
		compressed := false
		if compressMod > 0 && i%compressMod == 0 {
			payloadBytes, wasCompressed, _, err := crypto.CompressBlock(blockPlain)
			if err != nil {
				t.Fatalf("CompressBlock(%d): %v", i, err)
			}
			payload = payloadBytes
			compressed = wasCompressed
		}

		ciphertext, hmacValue, err := crypto.EncryptBlockV3(obj.dek, obj.iv, 0, uint32(i), payload, blockSize)
		if err != nil {
			t.Fatalf("EncryptBlockV3(%d): %v", i, err)
		}
		var hmacArr [32]byte
		copy(hmacArr[:], hmacValue)
		if err := obj.table.AddEntry(crypto.NewBlockTableEntry(hmacArr, uint32(len(ciphertext)), compressed)); err != nil {
			t.Fatalf("AddEntry(%d): %v", i, err)
		}
		obj.rawLens = append(obj.rawLens, uint32(len(ciphertext)))
		blocks = append(blocks, ciphertext...)
	}

	encodedTable, err := obj.table.Encode()
	if err != nil {
		t.Fatalf("table encode: %v", err)
	}
	header := make([]byte, crypto.HeaderSize)
	obj.stored = append(obj.stored, header...)
	obj.cipherSpan[0] = int64(len(obj.stored))
	obj.stored = append(obj.stored, blocks...)
	obj.cipherSpan[1] = int64(len(obj.stored))
	obj.stored = append(obj.stored, encodedTable...)
	return obj
}

// blockOffset returns the ciphertext-section offset of block i.
func (o *v3SinglePUTObject) blockOffset(t *testing.T, i int) int64 {
	t.Helper()
	off, ok := o.table.BlockOffset(uint32(i))
	if !ok {
		t.Fatalf("BlockOffset(%d) not representable", i)
	}
	return int64(off)
}

// truncatedRangeBackend serves a truncated view of the data span so the
// handler must observe a short span read; trailer fetches (at or past
// spanEnd) are served untruncated.
type truncatedRangeBackend struct {
	*rangeBackend
	spanEnd int64
	avail   int64
}

func (b *truncatedRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := b.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (b *truncatedRangeBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	if offset < b.spanEnd && offset+length > b.avail {
		length = b.avail - offset
		if length < 0 {
			length = 0
		}
	}
	return b.rangeBackend.GetRangeWithHeaders(ctx, bucket, key, offset, length)
}

// runV3SinglePUTRange invokes handleV3RangeRequest against the object.
func runV3SinglePUTRange(t *testing.T, obj *v3SinglePUTObject, beb backend.Backend, start, end int64) *httptest.ResponseRecorder {
	t.Helper()

	h := &Handlers{backend: beb}
	decryptor, err := crypto.NewDecryptor(obj.dek, obj.iv, obj.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   obj.blockSize,
		IV:          obj.iv,
		ETag:        "etag-v3-singleput-range",
		ContentType: "application/octet-stream",
	}

	r := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	h.handleV3RangeRequest(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(obj.plaintext)), int64(len(obj.stored)), time.Now(), start, end,
		v3TableKeyParts{ETag: armorMeta.ETag, StoredSize: int64(len(obj.stored))})
	return w
}

func TestV3SinglePUTRangeOneFetchForManyBlocks(t *testing.T) {
	const blockSize = 1024
	obj := newV3SinglePUTObject(t, 12, blockSize, 0)
	beb := newRangeBackend(obj.stored)

	start := int64(2*blockSize + 17) // mid-block 2
	end := int64(9*blockSize + 5)   // mid-block 9
	w := runV3SinglePUTRange(t, obj, beb, start, end)

	if w.Code != 206 {
		t.Fatalf("status = %d, body %q", w.Code, w.Body.String())
	}

	// One fetch for the trailer, one for the whole contiguous span — not one
	// per block.
	calls := beb.recordedCalls()
	if len(calls) != 2 {
		t.Fatalf("backend calls = %d (%+v), want 2 (trailer + span)", len(calls), calls)
	}

	wantSpanStart := obj.cipherSpan[0] + obj.blockOffset(t, 2)
	var wantSpanLen int64
	for i := 2; i <= 9; i++ {
		wantSpanLen += int64(obj.rawLens[i])
	}
	if calls[1].offset != wantSpanStart || calls[1].length != wantSpanLen {
		t.Fatalf("span fetch = (%d, %d), want (%d, %d)", calls[1].offset, calls[1].length, wantSpanStart, wantSpanLen)
	}

	if got := w.Body.String(); got != string(obj.plaintext[start:end+1]) {
		t.Fatalf("body length %d, want %d (plaintext mismatch)", len(got), int(end-start+1))
	}
	if got := w.Header().Get("Content-Range"); got != fmt.Sprintf("bytes %d-%d/%d", start, end, len(obj.plaintext)) {
		t.Errorf("Content-Range = %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != fmt.Sprintf("%d", end-start+1) {
		t.Errorf("Content-Length = %q", got)
	}
}

func TestV3SinglePUTRangeSliceSemantics(t *testing.T) {
	const blockSize = 512
	obj := newV3SinglePUTObject(t, 7, blockSize, 0)
	total := int64(len(obj.plaintext))

	cases := []struct {
		name       string
		start, end int64
	}{
		{"aligned full blocks", 1 * blockSize, 4*blockSize - 1},
		{"unaligned both ends", blockSize + 100, 3*blockSize + 55},
		{"single byte", 2*blockSize + 7, 2*blockSize + 7},
		{"suffix range", total - 100, total - 1},
		{"short tail within last block", total - blockSize/3, total - 1},
		{"whole object", 0, total - 1},
		{"starts at block boundary, ends mid-block", 2 * blockSize, 5*blockSize + 33},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			beb := newRangeBackend(obj.stored)
			w := runV3SinglePUTRange(t, obj, beb, tc.start, tc.end)
			if w.Code != 206 {
				t.Fatalf("status = %d, body %q", w.Code, w.Body.String())
			}
			want := string(obj.plaintext[tc.start : tc.end+1])
			if got := w.Body.String(); got != want {
				t.Fatalf("body mismatch: got %d bytes, want %d", len(got), len(want))
			}
			if got := w.Header().Get("Content-Range"); got != fmt.Sprintf("bytes %d-%d/%d", tc.start, tc.end, total) {
				t.Errorf("Content-Range = %q", got)
			}
		})
	}
}

func TestV3SinglePUTRangeCompressedBlocks(t *testing.T) {
	const blockSize = 1024
	// Blocks 0, 2, 4, 6 compressed; the range crosses compressed and raw
	// blocks and mid-block boundaries.
	obj := newV3SinglePUTObject(t, 8, blockSize, 2)
	beb := newRangeBackend(obj.stored)

	start := int64(blockSize / 2)
	end := int64(6*blockSize + blockSize/2)
	w := runV3SinglePUTRange(t, obj, beb, start, end)

	if w.Code != 206 {
		t.Fatalf("status = %d, body %q", w.Code, w.Body.String())
	}
	if got := w.Body.String(); got != string(obj.plaintext[start:end+1]) {
		t.Fatalf("body mismatch: got %d bytes, want %d", len(got), int(end-start+1))
	}

	// The fixture must actually have stored compressed blocks, or this test
	// exercises nothing.
	compressedSeen := false
	for _, entry := range obj.table.Entries {
		if entry.IsCompressed() {
			compressedSeen = true
		}
	}
	if !compressedSeen {
		t.Skip("compressor did not compress any block; fixture is not exercising the compressed path")
	}
}

func TestV3SinglePUTRangeCorruptBlockRejected(t *testing.T) {
	const blockSize = 1024
	obj := newV3SinglePUTObject(t, 6, blockSize, 0)

	// Flip one ciphertext byte inside block 3
	corruptAt := obj.cipherSpan[0] + obj.blockOffset(t, 3) + 11
	obj.stored[corruptAt] ^= 0xFF

	beb := newRangeBackend(obj.stored)
	start := int64(blockSize / 2)
	end := int64(4 * blockSize)
	w := runV3SinglePUTRange(t, obj, beb, start, end)

	// The handler must fail closed: 500, and the error body is the generic
	// S3 XML (no plaintext, no internal detail).
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500 for corrupt block", w.Code)
	}
	if strings.Contains(w.Body.String(), string(obj.plaintext[start:start+32])) {
		t.Error("error response leaked plaintext")
	}
}

func TestV3SinglePUTRangeShortSpanReadRejected(t *testing.T) {
	const blockSize = 1024
	obj := newV3SinglePUTObject(t, 6, blockSize, 0)

	// Serve the data span except the last two blocks' worth of it
	avail := obj.cipherSpan[1] - int64(obj.rawLens[4]) - int64(obj.rawLens[5])
	beb := &truncatedRangeBackend{rangeBackend: newRangeBackend(obj.stored), spanEnd: obj.cipherSpan[1], avail: avail}

	start := int64(blockSize)
	end := int64(5*blockSize - 1)
	w := runV3SinglePUTRange(t, obj, beb, start, end)

	// The short span read must fail the request, never serve a short 206.
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500 for truncated span", w.Code)
	}
}

// latencyRangeBackend injects a fixed delay per backend call, the way the
// production origin's round trips dominate a per-block fetch loop.
type latencyRangeBackend struct {
	*rangeBackend
	delay time.Duration
}

func (b *latencyRangeBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	time.Sleep(b.delay)
	return b.rangeBackend.GetRange(ctx, bucket, key, offset, length)
}

// peakRSSKB reads the process's high-water-mark resident set from
// /proc/self/status (Linux). ok is false where /proc is unavailable.
func peakRSSKB() (int64, bool) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if v, ok := strings.CutPrefix(line, "VmHWM:"); ok {
			if fields := strings.Fields(v); len(fields) >= 1 {
				if kb, err := strconv.ParseInt(fields[0], 10, 64); err == nil {
					return kb, true
				}
			}
		}
	}
	return 0, false
}

// TestV3SinglePUTRangeCoalesceBench is the opt-in latency-injected
// before/after measurement for the coalesced single-PUT range read
// (armor-5694713b). It reports backend request counts (also asserted
// deterministically by TestV3SinglePUTRangeOneFetchForManyBlocks; the
// before-shape is the prior handler's exact call pattern: 1 trailer fetch +
// 1 sequential block fetch per block), measured range latency at the
// injected RTT, and peak process heap.
//
//	ARMOR_RANGE_COALESCE_BENCH=1 go test ./internal/server/handlers/ \
//	  -run TestV3SinglePUTRangeCoalesceBench -v
//
// ARMOR_RANGE_COALESCE_BENCH_RTT_MS overrides the per-call RTT (default 74,
// the observed cross-region origin round trip).
func TestV3SinglePUTRangeCoalesceBench(t *testing.T) {
	if os.Getenv("ARMOR_RANGE_COALESCE_BENCH") == "" {
		t.Skip("set ARMOR_RANGE_COALESCE_BENCH=1 to run the latency-injected v3 range coalescing benchmark")
	}
	rttMS := 74
	if v := os.Getenv("ARMOR_RANGE_COALESCE_BENCH_RTT_MS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			t.Fatalf("ARMOR_RANGE_COALESCE_BENCH_RTT_MS must be a positive integer, got %q", v)
		}
		rttMS = n
	}
	rtt := time.Duration(rttMS) * time.Millisecond

	const blockSize = 64 << 10
	const blockCount = 1024 // 64MiB object
	obj := newV3SinglePUTObject(t, blockCount, blockSize, 0)

	start := int64(16 * blockSize)
	end := int64(1008*blockSize - 1)

	var memAfter runtime.MemStats
	runtime.GC()
	rssBefore, _ := peakRSSKB()

	beb := &latencyRangeBackend{rangeBackend: newRangeBackend(obj.stored), delay: rtt}
	fetchStart := time.Now()
	w := runV3SinglePUTRange(t, obj, beb, start, end)
	elapsed := time.Since(fetchStart)
	if w.Code != 206 {
		t.Fatalf("status = %d, body %q", w.Code, w.Body.String())
	}
	runtime.ReadMemStats(&memAfter)
	rssAfter, _ := peakRSSKB()

	calls := len(beb.recordedCalls())
	blocksInSpan := int(end/int64(blockSize)) - int(start/int64(blockSize)) + 1
	t.Logf("range %d-%d (%d blocks, %d bytes): backend calls=%d (before-shape: %d = 1 trailer + %d sequential block fetches), elapsed=%s at RTT=%dms, heap alloc total=%d MiB, peak RSS %d->%d KiB",
		start, end, blocksInSpan, end-start+1, calls, blocksInSpan+1, blocksInSpan, elapsed, rttMS, memAfter.TotalAlloc>>20, rssBefore, rssAfter)

	if calls != 2 {
		t.Fatalf("backend calls = %d, want 2", calls)
	}
	if got := w.Body.String(); len(got) != int(end-start+1) {
		t.Fatalf("body length = %d, want %d", len(got), int(end-start+1))
	}
}
