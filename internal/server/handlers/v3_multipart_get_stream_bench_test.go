package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// Benchmarks and the bounded-memory exercise for the streaming v3 multipart
// full GET (armor-272299ca), run against an in-file characterization of the
// whole-part implementation it replaced (per-part GetRange + io.ReadAll +
// decrypt-into-buffer + one write per part). Local fixture numbers only —
// production TTFB/throughput must be re-measured on the CF/B2 path.

// genSpec describes a logical object whose ciphertext is regenerated block
// by block on the fly: nothing larger than one block is materialized on the
// serve side, so benchmarks and the >5GiB leg run in bounded memory no
// matter the logical object size.
type genSpec struct {
	parts       []int // full-block counts per part
	blockSize   int
	cipherLen   int // ciphertext bytes per full block (uniform)
	dek, iv     []byte
	macs        [][][]byte // [part][block]
	blockPart   []int      // global block index -> part index
	partOffset  []int64    // ciphertext prefix sums per part, plus total
	totalPlain  int64
	totalCipher int64
}

func newGenSpec(tb testing.TB, parts []int, blockSize int) *genSpec {
	tb.Helper()
	dek := make([]byte, 32)
	iv := make([]byte, 16)
	if _, err := rand.Read(dek); err != nil {
		tb.Fatalf("rand dek: %v", err)
	}
	if _, err := rand.Read(iv); err != nil {
		tb.Fatalf("rand iv: %v", err)
	}
	s := &genSpec{parts: parts, blockSize: blockSize, dek: dek, iv: iv}
	off := int64(0)
	for p, blocks := range s.parts {
		s.partOffset = append(s.partOffset, off)
		s.macs = append(s.macs, nil)
		for b := 0; b < blocks; b++ {
			cipher, mac, err := crypto.EncryptBlockV3(dek, iv, uint16(p+1), uint32(b), s.plain(p, b), blockSize)
			if err != nil {
				tb.Fatalf("EncryptBlockV3(%d,%d): %v", p, b, err)
			}
			if s.cipherLen == 0 {
				s.cipherLen = len(cipher)
			} else if len(cipher) != s.cipherLen {
				tb.Fatalf("ciphertext length changed at (%d,%d): %d != %d", p, b, len(cipher), s.cipherLen)
			}
			s.macs[p] = append(s.macs[p], mac)
			s.blockPart = append(s.blockPart, p)
			off += int64(len(cipher))
			s.totalPlain += int64(blockSize)
		}
	}
	s.partOffset = append(s.partOffset, off)
	s.totalCipher = off
	return s
}

// plain derives deterministic plaintext for a (part, block) without storing
// the object.
func (s *genSpec) plain(part, block int) []byte {
	out := make([]byte, s.blockSize)
	binary.LittleEndian.PutUint32(out, uint32(part))
	binary.LittleEndian.PutUint32(out[4:], uint32(block))
	h := sha256.Sum256([]byte{byte(part), byte(block)})
	for i := 8; i < s.blockSize; i += sha256.Size {
		copy(out[i:], h[:])
		h = sha256.Sum256(h[:])
	}
	return out
}

// readAt fills buf with the object's ciphertext starting at off, regenerating
// only the blocks the range touches.
func (s *genSpec) readAt(buf []byte, off int64) {
	overhead := int64(s.cipherLen)
	gb := off / overhead
	fill := int64(0)
	for fill < int64(len(buf)) && gb < int64(len(s.blockPart)) {
		p := s.blockPart[gb]
		// part-local block index: subtract this part's leading block count
		local := int(gb)
		for q := 0; q < p; q++ {
			local -= s.parts[q]
		}
		cipher, _, err := crypto.EncryptBlockV3(s.dek, s.iv, uint16(p+1), uint32(local), s.plain(p, local), s.blockSize)
		if err != nil {
			panic(fmt.Sprintf("genSpec.readAt: regenerate (%d,%d): %v", p, local, err))
		}
		src := int64(0)
		if gb == off/overhead {
			src = off % overhead
		}
		n := copy(buf[fill:], cipher[src:])
		fill += int64(n)
		gb++
	}
}

func (s *genSpec) sidecar() *backend.HMACTableSidecarV3 {
	sc := &backend.HMACTableSidecarV3{Version: 3, BlockSize: s.blockSize}
	b64 := base64.StdEncoding
	for p, blocks := range s.parts {
		part := backend.HMACPartV3{N: p + 1, PlaintextLen: int64(blocks) * int64(s.blockSize)}
		clenRaw := make([]byte, 4)
		binary.BigEndian.PutUint32(clenRaw, uint32(s.cipherLen))
		for b := 0; b < blocks; b++ {
			part.Blocks = append(part.Blocks, []string{
				b64.EncodeToString(s.macs[p][b]),
				b64.EncodeToString(clenRaw),
			})
			part.CiphertextLen += int64(s.cipherLen)
		}
		sc.Parts = append(sc.Parts, part)
	}
	return sc
}

// genBackend serves ranges of a generated object, pausing delay per chunk to
// stand in for the CF/B2 network leg.
type genBackend struct {
	*backend.NilBackend
	spec  *genSpec
	delay time.Duration
}

func (b *genBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	return &genReader{b: b, ctx: ctx, pos: offset, end: offset + length}, nil
}

type genReader struct {
	b     *genBackend
	ctx   context.Context
	pos   int64
	end   int64
	srvd  bool
	stage []byte
}

func (r *genReader) Read(p []byte) (int, error) {
	if r.pos >= r.end {
		return 0, io.EOF
	}
	if len(r.stage) == 0 {
		if r.srvd {
			select {
			case <-r.ctx.Done():
				return 0, r.ctx.Err()
			case <-time.After(r.b.delay):
			}
		}
		r.srvd = true
		n := 256 << 10
		if rem := r.end - r.pos; rem < int64(n) {
			n = int(rem)
		}
		r.stage = make([]byte, n)
		r.b.spec.readAt(r.stage, r.pos)
	}
	n := copy(p, r.stage)
	r.stage = r.stage[n:]
	r.pos += int64(n)
	return n, nil
}

func (r *genReader) Close() error { return nil }

// ttfbWriter is an http.ResponseWriter that discards output and stamps the
// first write.
type ttfbWriter struct {
	header http.Header
	first  atomic.Int64 // nanoseconds since start; 0 = not yet written
	n      atomic.Int64
	start  time.Time
}

func newTTFBWriter() *ttfbWriter { return &ttfbWriter{header: http.Header{}, start: time.Now()} }

func (w *ttfbWriter) Header() http.Header { return w.header }
func (w *ttfbWriter) WriteHeader(int)     {}
func (w *ttfbWriter) Write(p []byte) (int, error) {
	w.first.CompareAndSwap(0, time.Since(w.start).Nanoseconds())
	w.n.Add(int64(len(p)))
	return len(p), nil
}

func (w *ttfbWriter) ttfb() time.Duration { return time.Duration(w.first.Load()) }

func newStreamBenchSetup(b *testing.B, spec *genSpec, be backend.Backend) (*Handlers, *crypto.Decryptor, *backend.ARMORMetadata, int64) {
	b.Helper()
	cache := backend.NewMultipartSidecarCache(10, 60)
	if err := cache.Set("bucket", "prefixed/key", v3RangeETag, spec.sidecar()); err != nil {
		b.Fatalf("cache.Set: %v", err)
	}
	entry := cache.Get("bucket", "prefixed/key", v3RangeETag)
	decryptor, err := crypto.NewDecryptor(spec.dek, spec.iv, spec.blockSize)
	if err != nil {
		b.Fatalf("NewDecryptor: %v", err)
	}
	meta := &backend.ARMORMetadata{
		BlockSize:   spec.blockSize,
		IV:          spec.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}
	return &Handlers{backend: be, multipartSidecarCache: cache}, decryptor, meta, entry.TotalPlaintextSize()
}

// wholePartV3Get characterizes the pre-armor-272299ca implementation: fetch
// each part whole, decrypt it into a growing buffer, write once per part.
func wholePartV3Get(out io.Writer, be backend.Backend, ctx context.Context, bucket, prefixedKey string, sidecar *backend.MultipartSidecarEntry, decryptor *crypto.Decryptor, iv []byte, blockSize int) error {
	for partIdx := 0; partIdx < sidecar.PartCount(); partIdx++ {
		part := sidecar.Sidecar.Parts[partIdx]
		partOffset, err := sidecar.GetPartCiphertextOffset(partIdx)
		if err != nil {
			return err
		}
		rc, err := be.GetRange(ctx, bucket, prefixedKey, partOffset, part.CiphertextLen)
		if err != nil {
			return err
		}
		partCiphertext, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		var buf []byte
		for blockIdx := range part.Blocks {
			blockLen, err := sidecar.GetBlockLength(partIdx, blockIdx)
			if err != nil {
				return err
			}
			expectedHMAC, err := sidecar.GetBlockHMAC(partIdx, blockIdx)
			if err != nil {
				return err
			}
			blockCipher := partCiphertext[:blockLen]
			partCiphertext = partCiphertext[blockLen:]
			plain, err := crypto.DecryptBlockV3(decryptor.DEK(), iv, uint16(part.N), uint32(blockIdx), blockCipher, expectedHMAC, blockSize)
			if err != nil {
				return err
			}
			buf = append(buf, plain...)
		}
		if _, err := out.Write(buf); err != nil {
			return err
		}
	}
	return nil
}

// runStreamGet drives the streaming handler once.
func runStreamGet(b *testing.B, h *Handlers, decryptor *crypto.Decryptor, meta *backend.ARMORMetadata, plaintextSize int64) *ttfbWriter {
	b.Helper()
	w := newTTFBWriter()
	r := httptest.NewRequest("GET", "/bucket/key", nil)
	h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, meta, plaintextSize, time.Now())
	return w
}

// BenchmarkV3MultipartGetStreamTTFB measures the streaming handler over a
// latency-injected backend: each 256KiB chunk costs 5ms, so the first
// verified plaintext should land after roughly one chunk plus block crypto.
func BenchmarkV3MultipartGetStreamTTFB(b *testing.B) {
	const blockSize = 64 << 10
	spec := newGenSpec(b, []int{16, 16, 16, 16}, blockSize) // 4 MiB logical object
	be := &genBackend{NilBackend: backend.NewNilBackend(), spec: spec, delay: 5 * time.Millisecond}
	h, decryptor, meta, plainSize := newStreamBenchSetup(b, spec, be)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := runStreamGet(b, h, decryptor, meta, plainSize)
		if got := w.n.Load(); got != plainSize {
			b.Fatalf("streamed %d bytes, want %d", got, plainSize)
		}
		b.ReportMetric(float64(w.ttfb().Microseconds())/1000, "ttfb-ms")
	}
}

// BenchmarkV3MultipartGetWholePartTTFB is the same object and backend driven
// through the whole-part characterization: the first write waits for the
// entire first part (four chunks here), which is exactly the first-byte
// delay armor-272299ca removed.
func BenchmarkV3MultipartGetWholePartTTFB(b *testing.B) {
	const blockSize = 64 << 10
	spec := newGenSpec(b, []int{16, 16, 16, 16}, blockSize)
	be := &genBackend{NilBackend: backend.NewNilBackend(), spec: spec, delay: 5 * time.Millisecond}
	_, decryptor, _, plainSize := newStreamBenchSetup(b, spec, be)
	entry, err := backend.NewMultipartSidecarEntry(spec.sidecar())
	if err != nil {
		b.Fatalf("sidecar entry: %v", err)
	}

	for i := 0; i < b.N; i++ {
		w := newTTFBWriter()
		if err := wholePartV3Get(w, be, context.Background(), "bucket", "prefixed/key", entry, decryptor, spec.iv, blockSize); err != nil {
			b.Fatalf("whole-part: %v", err)
		}
		if got := w.n.Load(); got != plainSize {
			b.Fatalf("streamed %d bytes, want %d", got, plainSize)
		}
		b.ReportMetric(float64(w.ttfb().Microseconds())/1000, "ttfb-ms")
	}
}

// TestV3MultipartGetFiveGiBLogicalObjectBoundedMemory exercises a >5GiB
// logical object end to end and asserts the round trip while reporting peak
// heap: the streaming handler's in-process payload stays at one ciphertext
// plus one plaintext block regardless of object size, and the serve side of
// the fixture generates ciphertext a block at a time — so a whole-object or
// whole-part buffer would show up as a multi-GiB heap. Skipped unless
// ARMOR_V3_STREAM_5GIB=1 (the leg decrypts six gibibytes).
func TestV3MultipartGetFiveGiBLogicalObjectBoundedMemory(t *testing.T) {
	if os.Getenv("ARMOR_V3_STREAM_5GIB") != "1" {
		t.Skip("set ARMOR_V3_STREAM_5GIB=1 to run the >5GiB bounded-memory leg")
	}
	const blockSize = 64 << 10
	const mib = 1 << 20
	const gib = 1 << 30
	// Six 1GiB parts: 6GiB logical plaintext, past both 2^32 and B2's 5GiB
	// part ceiling.
	blocksPerGib := int(gib / blockSize)
	spec := newGenSpec(t, []int{blocksPerGib, blocksPerGib, blocksPerGib, blocksPerGib, blocksPerGib, blocksPerGib}, blockSize)
	be := &genBackend{NilBackend: backend.NewNilBackend(), spec: spec, delay: 0}
	cache := backend.NewMultipartSidecarCache(10, 60)
	if err := cache.Set("bucket", "prefixed/key", v3RangeETag, spec.sidecar()); err != nil {
		t.Fatalf("cache.Set: %v", err)
	}
	decryptor, err := crypto.NewDecryptor(spec.dek, spec.iv, spec.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	meta := &backend.ARMORMetadata{
		BlockSize:   spec.blockSize,
		IV:          spec.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}
	h := &Handlers{backend: be, multipartSidecarCache: cache}

	var peakHeap atomic.Int64
	stop := make(chan struct{})
	go func() {
		var ms runtime.MemStats
		tick := time.NewTicker(20 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				runtime.ReadMemStats(&ms)
				peak(&peakHeap, int64(ms.HeapAlloc))
			}
		}
	}()

	w := newTTFBWriter()
	r := httptest.NewRequest("GET", "/bucket/key", nil)
	started := time.Now()
	h.handleV3MultipartGet(w, r, "bucket", "key", "prefixed/key", decryptor, meta, spec.totalPlain, time.Now())
	elapsed := time.Since(started)
	close(stop)

	if got := w.n.Load(); got != spec.totalPlain {
		t.Fatalf("streamed %d bytes, want %d", got, spec.totalPlain)
	}
	peakMiB := peakHeap.Load() / mib
	t.Logf("logical object: %d MiB (%d bytes); TTFB %v; wall %v; peak HeapAlloc %d MiB",
		spec.totalPlain/mib, spec.totalPlain, w.ttfb(), elapsed, peakMiB)
	if peakMiB > 512 {
		t.Fatalf("peak heap %d MiB suggests object-sized buffering; want bounded windows", peakMiB)
	}
}

func peak(counter *atomic.Int64, v int64) {
	for {
		cur := counter.Load()
		if v <= cur || counter.CompareAndSwap(cur, v) {
			return
		}
	}
}
