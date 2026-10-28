package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

// uploadPartSource is one v3 UploadPart's plaintext held as a seekable
// source, so the replayable part reader can encrypt lazily under the
// backend's read and rewind byte-exactly on a backend retry
// (armor-cb2b0731). Known Content-Lengths within the pooled part size reuse
// the shared buffer pool; unknown or larger lengths spill to a temp file, so
// resident memory per spilled part is the copy buffer, not the part.
type uploadPartSource struct {
	reader  io.ReadSeeker
	size    int64
	digest  [sha256.Size]byte
	release func()
}

// partMemoryBudget bounds the aggregate plaintext bytes concurrent v3
// UploadPart requests may hold in memory. Without it, in-memory part bodies
// scale with client-chosen upload concurrency — the iad-ci workload ran
// eleven 64 MiB parts against a 1 GiB pod — so the budget is what turns
// "sum of client-selected part sizes" into a documented constant.
type partMemoryBudget struct {
	mu      sync.Mutex
	total   int64
	avail   int64
	waiters []chan struct{}
}

func newPartMemoryBudget(total int64) *partMemoryBudget {
	if total < 1 {
		total = 1
	}
	return &partMemoryBudget{total: total, avail: total}
}

// acquire reserves n bytes, blocking until the budget can admit the part or
// ctx is done. A part larger than the whole budget reserves everything and
// is admitted alone, so admission never deadlocks on an oversized part; the
// release notes document that such a part may transiently exceed the budget
// by up to one part. Requests blocked here leave their bodies unread, which
// is the backpressure: the client's own connection window holds the rest.
func (b *partMemoryBudget) acquire(ctx context.Context, n int64) (release func(), err error) {
	if b == nil || n <= 0 {
		return func() {}, nil
	}
	if n > b.total {
		n = b.total
	}
	for {
		b.mu.Lock()
		if b.avail >= n {
			b.avail -= n
			b.mu.Unlock()
			var once sync.Once
			held := n
			return func() {
				once.Do(func() {
					b.mu.Lock()
					b.avail += held
					waiters := b.waiters
					b.waiters = nil
					b.mu.Unlock()
					// Wake everyone; a woken request that still does not fit
					// re-adds itself, so a large part cannot swallow a
					// signal meant for a smaller one.
					for _, ch := range waiters {
						close(ch)
					}
				})
			}, nil
		}
		ch := make(chan struct{})
		b.waiters = append(b.waiters, ch)
		b.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			b.mu.Lock()
			b.removeWaiterLocked(ch)
			b.mu.Unlock()
			return func() {}, ctx.Err()
		}
	}
}

// removeWaiterLocked drops ch from the waiter list. It is a no-op when a
// release already consumed the list (closing ch again would panic).
func (b *partMemoryBudget) removeWaiterLocked(ch chan struct{}) {
	kept := b.waiters[:0]
	for _, w := range b.waiters {
		if w != ch {
			kept = append(kept, w)
		}
	}
	b.waiters = kept
}

// readUploadPartBodySeekable reads one v3 part body into a bounded, seekable
// plaintext source, computing its SHA-256 digest while reading. Parts with a
// known Content-Length at or below the pooled part size are admitted against
// budget before their bodies are read, so the aggregate stays bounded no
// matter how many parts the client uploads at once. Unknown or larger
// lengths never touch the budget: they spill to a temp file, and their
// memory cost is the streaming copy buffer.
func readUploadPartBodySeekable(ctx context.Context, body io.Reader, contentLength int64, budget *partMemoryBudget) (*uploadPartSource, error) {
	if contentLength >= 0 && contentLength <= pooledUploadPartSize {
		releaseBudget, err := budget.acquire(ctx, contentLength)
		if err != nil {
			return nil, err
		}
		data, digest, releaseBody, err := readUploadPartBody(body, contentLength)
		if err != nil {
			releaseBudget()
			return nil, err
		}
		src := &uploadPartSource{
			reader: bytes.NewReader(data),
			size:   int64(len(data)),
			digest: digest,
		}
		src.release = func() {
			releaseBody()
			releaseBudget()
		}
		return src, nil
	}
	return spoolUploadPartBody(body, contentLength)
}

// spoolUploadPartBody streams the part body to a temp file while hashing it,
// for part bodies with an unknown Content-Length or one above the pooled
// part size. The file is the seekable plaintext source; release closes and
// removes it.
func spoolUploadPartBody(body io.Reader, contentLength int64) (*uploadPartSource, error) {
	var source uploadPartDigestSource
	if candidate, ok := body.(uploadPartDigestSource); ok && candidate.CanVerifyPayloadSHA256() {
		source = candidate
	}
	hasher := sha256.New()
	var reader io.Reader = body
	if source == nil {
		reader = io.TeeReader(body, hasher)
	}
	f, err := os.CreateTemp("", "armor-part-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create part spool: %w", err)
	}
	size, err := io.Copy(f, reader)
	if err == nil && contentLength >= 0 && size != contentLength {
		err = fmt.Errorf("request body is shorter than Content-Length")
	}
	if err == nil {
		var digest [sha256.Size]byte
		if source != nil {
			var verified bool
			digest, verified = source.VerifiedPayloadSHA256()
			if !verified {
				err = errors.New("signed payload digest unavailable after body read")
			}
		} else {
			hasher.Sum(digest[:0])
		}
		if err == nil {
			if _, serr := f.Seek(0, io.SeekStart); serr != nil {
				err = fmt.Errorf("seek part spool: %w", serr)
			}
		}
		if err == nil {
			name := f.Name()
			src := &uploadPartSource{reader: f, size: size, digest: digest}
			src.release = func() {
				f.Close()
				os.Remove(name)
			}
			return src, nil
		}
	}
	f.Close()
	os.Remove(f.Name())
	return nil, err
}
