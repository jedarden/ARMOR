package server

// The manifest startup subsystem: bind-first cold loading (plan §9.6).
//
// The cold load used to run synchronously inside server.New, so a slow
// backend consumed the startupProbe budget before either listener could
// answer (iad-ci hit the 480 s bound on every start for a day, then served
// with an empty index). New now returns with the load still running in the
// background: the S3 and admin listeners bind immediately, index misses fall
// back to B2, and writes recorded while the load runs are buffered and
// replayed once the load establishes a safe delta sequence. A partially
// populated index is never exposed — each load attempt fills a private index
// and only a completed one is published — and shutdown cancels the load
// instead of waiting it out.

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/server/handlers"
)

// manifestStatus is the lifecycle phase of the manifest subsystem, published
// through /readyz as manifest_status so operators can tell a still-loading
// pod from a degraded one (plan §9.6).
type manifestStatus string

const (
	// manifestLoading: the background cold load is running. Reads miss the
	// index and fall back to B2; writes are buffered for replay.
	manifestLoading manifestStatus = "loading"
	// manifestLoaded: the cold load (or a first-startup empty store)
	// succeeded; the delta writer and compactor are running.
	manifestLoaded manifestStatus = "loaded"
	// manifestDegraded: the cold load failed after retries. The note names
	// the mode — deltas published above a recovered sequence floor with
	// compaction paused, or delta publication paused outright.
	manifestDegraded manifestStatus = "degraded"
	// manifestDisabled: ARMOR_MANIFEST_ENABLED=false; no recorder is wired.
	manifestDisabled manifestStatus = "disabled"
	// manifestCanceled: shutdown stopped an in-flight load before it
	// resolved. Appears only while the process is exiting.
	manifestCanceled manifestStatus = "canceled"
)

// manifestState is one immutable published snapshot of the manifest
// subsystem. Readers load the pointer atomically and never mutate what they
// receive; the loader goroutine is the only writer.
type manifestState struct {
	status manifestStatus
	// idx is the live index. Empty while loading; the loaded snapshot once
	// resolved. Nil only when the manifest is disabled.
	idx *manifest.Index
	// writer asynchronously persists deltas; nil while loading and in every
	// mode where delta publication is unsafe (no sequence floor).
	writer *manifest.Writer
	// compactor periodically folds deltas into a snapshot; started only on a
	// fully loaded index (see startManifestSystem).
	compactor *manifest.Compactor
	// degradedReason records the cold-load error when the pod fell back;
	// "" when the load succeeded.
	degradedReason string
	// degradedNote names the degraded mode for /readyz; "" when the load
	// succeeded.
	degradedNote string
}

// bufferedRecord is a write recorded while the cold load was still running.
// The recorder cannot apply it yet — there is no writer and no safe delta
// sequence — so it waits inside manifestSystem for the load to resolve.
type bufferedRecord struct {
	bucket, key string
	// entry is the recorded manifest entry; nil marks a delete.
	entry *manifest.Entry
	// chain is the provenance chain entry to embed in the delta line.
	chain *manifest.ChainEntry
}

// apply records the write on the resolved state: the index update makes it
// visible to lookups immediately, the enqueue persists it as a delta above
// the sequence the resolution just established.
func (b bufferedRecord) apply(st *manifestState) {
	if b.entry != nil {
		st.idx.Put(b.bucket, b.key, b.entry)
		st.writer.EnqueuePut(b.bucket, b.key, b.entry, b.chain)
		return
	}
	st.idx.Delete(b.bucket, b.key)
	st.writer.EnqueueDelete(b.bucket, b.key)
}

// manifestSettleWait bounds how long settle waits for an in-flight load to
// observe cancellation. Correctness does not depend on the wait: resolution
// re-checks the context under the handoff lock, so a load that settles late
// either published before settle (its writer and compactor are then stopped
// by the Stop callers) or observes the cancellation and publishes nothing.
// The wait only keeps shutdown from outrunning a fetch that is already
// unwinding past its context.
const manifestSettleWait = 10 * time.Second

// manifestSystem owns the manifest subsystem's published state and its
// background cold load. server.New builds one and returns before the load
// resolves; the recorder reads the published state per call, so it stays
// correct across the loading → loaded/degraded transition without any
// request-path locking beyond the atomic load.
type manifestSystem struct {
	// mu guards buffer and serializes the resolve/settle handoff: records
	// taken while loading and the resolution that replays them must not
	// interleave, or a write could be buffered and never replayed (or
	// replayed twice).
	mu     sync.Mutex
	state  atomic.Pointer[manifestState]
	buffer []bufferedRecord

	// loadCancel stops an in-flight load; nil when nothing runs (disabled).
	loadCancel context.CancelFunc
	// loadDone is closed when the loader goroutine (if any) has exited.
	loadDone   chan struct{}
	settleOnce sync.Once
}

// newManifestSystem builds the holder in its pre-start state.
func newManifestSystem() *manifestSystem {
	return &manifestSystem{loadDone: make(chan struct{})}
}

// resolvedManifestSystem builds a holder already settled into st — the state
// New would publish after a completed load. Tests use it to construct
// servers with a known manifest phase without racing a real load.
func resolvedManifestSystem(st manifestState) *manifestSystem {
	sys := newManifestSystem()
	sys.state.Store(&st)
	close(sys.loadDone)
	return sys
}

// start launches the background cold load (or records that the manifest is
// disabled). It returns immediately — that is the point (plan §9.6) — and is
// called from server.New before the listeners bind.
func (sys *manifestSystem) start(cfg *config.Config, logger *logging.Logger, primaryBackend backend.Backend) {
	if !cfg.ManifestEnabled {
		sys.state.Store(&manifestState{status: manifestDisabled})
		close(sys.loadDone)
		return
	}
	sys.state.Store(&manifestState{status: manifestLoading, idx: manifest.New()})
	loadCtx, cancel := context.WithCancel(context.Background())
	sys.loadCancel = cancel
	go func() {
		defer close(sys.loadDone)
		st := startManifestSystem(loadCtx, cfg, logger, primaryBackend)
		sys.resolve(loadCtx, st)
	}()
}

// resolve publishes the load outcome. It runs under mu so buffered records
// cannot be lost to a concurrent recorder call: every record either sees the
// pre-resolution loading state (and buffers, joining this replay) or the
// published post-resolution state (and applies directly).
func (sys *manifestSystem) resolve(loadCtx context.Context, st *manifestStartup) {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	if loadCtx.Err() != nil || st.canceled {
		// Shutdown stopped the load. A load that had already succeeded may
		// have started its writer and compactor inside startManifestSystem;
		// nothing else will stop them once settle has returned, so they are
		// stopped here before publishing the terminal state. Buffered
		// records are dropped — the process is exiting and no delta
		// publication ever became safe for them.
		if st.writer != nil {
			st.writer.Stop()
		}
		if st.compactor != nil {
			st.compactor.Stop()
		}
		sys.buffer = nil
		sys.state.Store(&manifestState{status: manifestCanceled, idx: st.idx})
		return
	}
	// Replay writes recorded while the load ran. They were never publishable
	// during the load (no writer, no safe sequence), so nothing has been
	// lost or collided with; replaying them now keeps this pod's
	// read-your-writes intact against the freshly loaded snapshot — without
	// it, a HEAD after the swap could serve the loaded pre-write entry for a
	// key this pod overwrote mid-load — and publishes their deltas above
	// the sequence the resolution just established. Replay requires a writer:
	// in the publication-paused degraded mode (no recovered floor) apply
	// would dereference a nil writer, and the drop rule record() already
	// applies in that mode is the right outcome anyway — the objects are in
	// B2 with authoritative headers, so the buffered records are discarded.
	published := st.state()
	if published.writer != nil {
		for _, rec := range sys.buffer {
			rec.apply(published)
		}
	}
	sys.buffer = nil
	sys.state.Store(published)
}

// record applies one write to the manifest subsystem. Once the load has
// resolved with a writer running, the index update and delta enqueue happen
// synchronously, exactly as before the load moved to the background. While
// the load is still running the record is buffered for replay at resolution
// — publishing earlier could collide with persisted deltas (armor-b1745d47).
// In disabled, canceled, and publication-paused modes records are dropped:
// no recorder was wired in those modes before, and B2 object headers stay
// authoritative for anything the index misses.
func (sys *manifestSystem) record(rec bufferedRecord) {
	sys.mu.Lock()
	defer sys.mu.Unlock()
	st := sys.state.Load()
	switch {
	case st == nil:
		return
	case st.writer != nil:
		rec.apply(st)
	case st.status == manifestLoading:
		sys.buffer = append(sys.buffer, rec)
	}
}

// settle stops an in-flight cold load. The context cancel unblocks the
// backend fetches; the loader then publishes a terminal canceled state
// without starting (or after stopping) its writer and compactor. Safe to
// call repeatedly and when nothing runs.
func (sys *manifestSystem) settle() {
	if sys.loadCancel == nil {
		return
	}
	sys.settleOnce.Do(func() {
		sys.loadCancel()
		select {
		case <-sys.loadDone:
		case <-time.After(manifestSettleWait):
			// The load ignored its context past the bound. It can no longer
			// start a writer — resolution re-checks the canceled context
			// under mu — so shutdown proceeds.
		}
	})
}

// status returns the currently published phase.
func (sys *manifestSystem) status() manifestStatus {
	if st := sys.state.Load(); st != nil {
		return st.status
	}
	return manifestDisabled
}

// manifestStartup is the outcome of the manifest startup sequence in
// startManifestSystem.
type manifestStartup struct {
	// idx is the in-memory manifest index; nil only when the manifest is
	// disabled (ARMOR_MANIFEST_ENABLED=false).
	idx *manifest.Index
	// writer asynchronously persists deltas; nil when the manifest is
	// disabled or when delta publication is unsafe (no sequence floor).
	writer *manifest.Writer
	// compactor periodically folds deltas into a snapshot; nil when the
	// manifest is disabled, when publication is unsafe, or when the startup
	// index is incomplete (a compaction pass on an index that missed the
	// persisted shards would overwrite the good snapshot and delete deltas
	// whose entries were never loaded).
	compactor *manifest.Compactor
	// degradedReason records the cold-load error when the pod fell back
	// (armor-f1791426); "" when the load succeeded.
	degradedReason string
	// degradedNote is the /readyz note describing which degraded mode the
	// pod is in; "" when the load succeeded.
	degradedNote string
	// canceled reports that the caller's context stopped the load before it
	// resolved (shutdown raced the load). No writer or compactor was
	// started, and the caller must not publish deltas.
	canceled bool
}

// state converts the startup outcome into the published manifestState.
// Called only after the canceled check in manifestSystem.resolve.
func (s *manifestStartup) state() *manifestState {
	st := &manifestState{
		idx:       s.idx,
		writer:    s.writer,
		compactor: s.compactor,
	}
	if s.degradedReason == "" {
		st.status = manifestLoaded
	} else {
		st.status = manifestDegraded
		st.degradedReason = s.degradedReason
		st.degradedNote = s.degradedNote
	}
	return st
}

// startManifestSystem performs the manifest startup sequence against
// primaryBackend: load the index from B2 (with retries), then start the
// delta writer and compactor as the load outcome allows. It runs inside the
// manifestSystem loader goroutine, after the listeners have bound; ctx is
// the loader's context and cancels at shutdown.
//
// The manifest is a performance optimisation, so a failed load never prevents
// startup — but the fallback must not reset the delta sequence (plan §9.6).
// An index that loads empty and restarts its writer at sequence 1 would PUT
// delta-0000000001.jsonl over a persisted delta of the same name after a
// transient load failure. The fallback therefore lists the current writer's
// shard — key names alone encode the sequence, so a list recovers a safe
// high-water mark without fetching a single delta body — and resumes
// publishing above that floor with compaction paused (the index content is
// still incomplete until a successful load). When even that list fails, no
// floor exists: delta publication stays off entirely and reads/writes go
// through the existing B2 fallback until a restart loads successfully.
func startManifestSystem(ctx context.Context, cfg *config.Config, logger *logging.Logger, primaryBackend backend.Backend) *manifestStartup {
	s := &manifestStartup{}
	if !cfg.ManifestEnabled {
		return s
	}
	s.idx = manifest.New()
	// ListRaw bypasses the .armor/ filter so manifest keys are discoverable.
	lister := func(ctx context.Context, prefix, token string) ([]string, string, error) {
		result, lerr := primaryBackend.ListRaw(ctx, cfg.Bucket, prefix, "", token, 1000)
		if lerr != nil {
			return nil, "", lerr
		}
		keys := make([]string, len(result.Objects))
		for i, obj := range result.Objects {
			keys[i] = obj.Key
		}
		return keys, result.NextToken, nil
	}
	// Fetch snapshot and delta blobs via Cloudflare for free egress and edge
	// caching. Fall back to direct B2 if no Cloudflare domain is configured.
	fetcher := func(ctx context.Context, key string) ([]byte, error) {
		var body io.ReadCloser
		var ferr error
		if cfg.CFDomain != "" {
			body, _, ferr = primaryBackend.Get(ctx, cfg.Bucket, key)
		} else {
			body, _, ferr = primaryBackend.GetDirect(ctx, cfg.Bucket, key)
		}
		if ferr != nil {
			return nil, ferr
		}
		defer body.Close()
		return io.ReadAll(body)
	}
	loadStart := time.Now()
	// Progress marker before the load: pod logs are the only window into the
	// background load, and a load stuck on slow backend fetches must not
	// produce silence that looks like a hung process.
	logger.WithFields(map[string]interface{}{
		"timeout": time.Duration(cfg.ManifestLoadTimeout) * time.Second,
		"backend": "background",
	}).Info("manifest cold load starting")
	// Bound the load so it cannot wedge the loader forever: the B2 client
	// timeout is 30 minutes per request, and a load that outlasts every
	// bound must land in the same non-fatal fallback as a load error below.
	// Timing out no longer blocks the listeners (they are already bound);
	// the async writer keeps persisting new deltas.
	loadCtx, cancelLoad := manifestLoadContext(ctx, cfg)
	defer cancelLoad()
	// Retry the whole load under the same bounded read-retry policy as
	// backend fetches (armor-f1791426): a transient Cloudflare 52x during
	// startup previously landed the pod straight in the empty-index
	// fallback, where it stays until restart. Each attempt loads into a
	// fresh index so a failed attempt cannot half-fill the merged one.
	retryPolicy := backend.RetryPolicy{
		Attempts:   cfg.ReadRetryAttempts,
		Backoff:    cfg.ReadRetryBackoff,
		MaxBackoff: cfg.ReadRetryMaxBackoff,
	}
	loaded := false
	if _, loadErr := backend.DoWithRetry(loadCtx, retryPolicy, func() error {
		fresh := manifest.New()
		if err := manifest.Load(loadCtx, fresh, cfg.ManifestPrefix, cfg.WriterID, lister, fetcher); err != nil {
			return err
		}
		s.idx = fresh
		return nil
	}); loadErr != nil {
		if ctx.Err() != nil {
			// Shutdown raced the load. No floor recovery, no writer: the
			// process is exiting and any publication would race Stop.
			s.canceled = true
			return s
		}
		logger.WithFields(map[string]interface{}{
			"error":    loadErr.Error(),
			"duration": time.Since(loadStart).String(),
		}).Warn("manifest startup load failed — recovering delta sequence floor")
		s.degradedReason = loadErr.Error()
		// Fresh budget: a timeout that killed the load must not also kill
		// the floor recovery, and the recovery list is one cheap call.
		floorCtx, cancelFloor := manifestLoadContext(ctx, cfg)
		defer cancelFloor()
		var floor uint64
		if _, floorErr := backend.DoWithRetry(floorCtx, retryPolicy, func() error {
			f, err := manifest.RecoverSeqFloor(floorCtx, cfg.ManifestPrefix, cfg.WriterID, lister)
			floor = f
			return err
		}); floorErr != nil {
			if ctx.Err() != nil {
				s.canceled = true
				return s
			}
			// No floor: any delta this pod wrote could overwrite a
			// persisted one. Publication stays off; reads and writes go
			// through the B2 fallback until a restart loads successfully.
			logger.WithFields(map[string]interface{}{
				"error": floorErr.Error(),
			}).Warn("delta sequence floor unrecoverable — manifest delta publication paused")
			s.degradedNote = "manifest startup load failed after retries — serving with an empty index; delta sequence floor unrecoverable, delta publication paused"
			return s
		}
		s.idx.SetSeq(floor)
		s.degradedNote = fmt.Sprintf("manifest startup load failed after retries — serving with an empty index; delta sequence recovered at %d, compaction paused", floor)
		logger.WithFields(map[string]interface{}{
			"seq_floor": floor,
		}).Info("delta sequence floor recovered — publishing deltas above it with compaction paused")
	} else {
		loaded = true
		logger.WithFields(map[string]interface{}{
			"entries":  s.idx.Len(),
			"seq":      s.idx.Seq(),
			"duration": time.Since(loadStart).String(),
		}).Info("manifest index loaded")
	}

	// Create and start the manifest writer (async delta persistence to B2).
	// Uploads go direct to B2 (free ingress); no Cloudflare path needed here.
	uploader := func(ctx context.Context, key string, data []byte) error {
		return primaryBackend.Put(ctx, cfg.Bucket, key, bytes.NewReader(data), int64(len(data)), nil)
	}
	// Chain-head writer uploads .armor/chain-head/<writer> after each batch
	chainHeadWriter := func(ctx context.Context, key string, data []byte) error {
		return primaryBackend.Put(ctx, cfg.Bucket, key, bytes.NewReader(data), int64(len(data)), nil)
	}
	s.writer = manifest.NewWriterWithChain(s.idx, cfg.ManifestPrefix, cfg.WriterID, uploader, chainHeadWriter, 0)

	if loaded {
		// Compactor lists and batch-deletes via the B2 backend. It runs only
		// on a fully loaded index: compacting the empty fallback index would
		// upload a snapshot missing every pre-failure entry and then delete
		// the deltas that carried them.
		listerForCompactor := func(ctx context.Context, prefix, token string) ([]string, string, error) {
			result, lerr := primaryBackend.ListRaw(ctx, cfg.Bucket, prefix, "", token, 1000)
			if lerr != nil {
				return nil, "", lerr
			}
			keys := make([]string, len(result.Objects))
			for i, obj := range result.Objects {
				keys[i] = obj.Key
			}
			return keys, result.NextToken, nil
		}
		deleter := func(ctx context.Context, keys []string) error {
			return primaryBackend.DeleteObjects(ctx, cfg.Bucket, keys)
		}
		compactionInterval := time.Duration(cfg.ManifestCompactionInterval) * time.Second

		// Create a logger writer adapter that bridges the structured logger to
		// the standard library logger interface expected by the compactor.
		loggerWriter := &loggerWriter{logger: logger}
		compactorLogger := log.New(loggerWriter, "[manifest-compactor] ", log.LstdFlags|log.Lmsgprefix)

		s.compactor = manifest.NewCompactor(
			s.idx,
			cfg.ManifestPrefix,
			cfg.WriterID,
			uploader,
			listerForCompactor,
			deleter,
			compactionInterval,
			cfg.ManifestCompactionThreshold,
			compactorLogger,
			metrics.DefaultMetrics,
		)

		// Wire writer → compactor: after each delta flush, notify compactor.
		s.writer.SetOnFlush(s.compactor.NotifyDelta)
	}

	s.writer.Start(context.Background())
	if s.compactor != nil {
		s.compactor.Start(context.Background())
	}
	if loaded {
		logger.Info("manifest writer and compactor started")
	} else {
		logger.Info("manifest writer started (compaction paused — startup index incomplete)")
	}
	return s
}

// manifestLoadContext returns the context for one startup manifest pass,
// bounded by ARMOR_MANIFEST_LOAD_TIMEOUT (0 = unbounded) and cut short
// whenever parent (the loader context, canceled at shutdown) is.
func manifestLoadContext(parent context.Context, cfg *config.Config) (context.Context, context.CancelFunc) {
	if cfg.ManifestLoadTimeout > 0 {
		return context.WithTimeout(parent, time.Duration(cfg.ManifestLoadTimeout)*time.Second)
	}
	return context.WithCancel(parent)
}

// manifestRecorder implements handlers.ManifestRecorder. It resolves the
// manifest subsystem's published state per call, so one recorder instance
// stays correct across the background cold load: while loading, lookups miss
// (reads fall back to B2) and records buffer for replay; once resolved with
// a writer running, the index update and delta enqueue are synchronous, as
// they were before the load moved off the listener startup path.
type manifestRecorder struct {
	sys *manifestSystem
}

func (m *manifestRecorder) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64, userMetadata map[string]string) {
	entry := &manifest.Entry{
		PlaintextSize:   size,
		PlaintextSHA256: sha256Hex,
		IV:              iv,
		WrappedDEK:      wrappedDEK,
		MEKFingerprint:  mekFingerprint,
		BlockSize:       blockSize,
		ContentType:     contentType,
		ETag:            etag,
		LastModified:    time.Now().UTC(),
		CiphertextSize:  ciphertextSize,
		UserMetadata:    userMetadata,
	}
	m.sys.record(bufferedRecord{bucket: bucket, key: key, entry: entry, chain: chainEntry})
}

func (m *manifestRecorder) RecordDelete(bucket, key string) {
	m.sys.record(bufferedRecord{bucket: bucket, key: key})
}

func (m *manifestRecorder) Lookup(bucket, key string) (*handlers.ManifestEntry, bool) {
	st := m.sys.state.Load()
	// Loading (empty index), disabled, canceled, and publication-paused
	// modes all serve lookups as misses: no recorder was wired in those
	// modes before the load moved to the background, and the B2-backed read
	// path behind every miss is the fallback this design relies on.
	if st == nil || st.idx == nil || st.writer == nil {
		return nil, false
	}
	entry, ok := st.idx.Get(bucket, key)
	if !ok {
		return nil, false
	}
	return &handlers.ManifestEntry{
		PlaintextSize:  entry.PlaintextSize,
		ContentType:    entry.ContentType,
		ETag:           entry.ETag,
		LastModified:   entry.LastModified,
		IV:             entry.IV,
		WrappedDEK:     entry.WrappedDEK,
		BlockSize:      entry.BlockSize,
		CiphertextSize: entry.CiphertextSize,
		UserMetadata:   entry.UserMetadata,
	}, true
}
