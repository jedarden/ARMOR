package backend

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// readBufferBudget reserves the backing bytes of ranged-read buffers across
// every reader sharing one B2 backend. It uses a waiter list instead of a
// channel so a final, short fetch can reserve a different number of bytes.
type readBufferBudget struct {
	mu      sync.Mutex
	total   int64
	avail   int64
	waiters []chan struct{}
	inUse   atomic.Int64
	peak    atomic.Int64
}

func newReadBufferBudget(total int64) *readBufferBudget {
	if total < 1 {
		total = 1
	}
	return &readBufferBudget{total: total, avail: total}
}

// acquire reserves n backing bytes, blocking until they are available or ctx
// is canceled. A caller must invoke the returned release exactly once.
func (b *readBufferBudget) acquire(ctx context.Context, n int64) (release func(), wait time.Duration, saturated bool, err error) {
	if b == nil || n <= 0 {
		return func() {}, 0, false, nil
	}
	if n > b.total {
		return nil, 0, false, context.Canceled
	}
	started := time.Now()
	waited := false
	for {
		b.mu.Lock()
		if b.avail >= n {
			b.avail -= n
			b.mu.Unlock()
			held := n
			used := b.inUse.Add(held)
			for {
				peak := b.peak.Load()
				if used <= peak || b.peak.CompareAndSwap(peak, used) {
					break
				}
			}
			var once sync.Once
			return func() {
				once.Do(func() {
					b.inUse.Add(-held)
					b.mu.Lock()
					b.avail += held
					waiters := b.waiters
					b.waiters = nil
					b.mu.Unlock()
					for _, ch := range waiters {
						close(ch)
					}
				})
			}, time.Since(started), waited, nil
		}
		ch := make(chan struct{})
		b.waiters = append(b.waiters, ch)
		b.mu.Unlock()
		waited = true
		select {
		case <-ch:
		case <-ctx.Done():
			b.mu.Lock()
			b.removeWaiterLocked(ch)
			b.mu.Unlock()
			return nil, time.Since(started), true, ctx.Err()
		}
	}
}

func (b *readBufferBudget) removeWaiterLocked(ch chan struct{}) {
	kept := b.waiters[:0]
	for _, waiter := range b.waiters {
		if waiter != ch {
			kept = append(kept, waiter)
		}
	}
	b.waiters = kept
}

func (b *readBufferBudget) inUseBytes() int64 {
	if b == nil {
		return 0
	}
	return b.inUse.Load()
}

func (b *readBufferBudget) peakBytes() int64 {
	if b == nil {
		return 0
	}
	return b.peak.Load()
}

// readBuffer owns one pooled fetch backing array and its budget reservation.
// Keeping the pool value as *[]byte avoids the SA6002 interface allocation.
type readBuffer struct {
	buf           []byte
	data          []byte
	pool          *sync.Pool
	releaseBudget func()
	once          sync.Once
}

func (b *readBuffer) release() {
	if b == nil {
		return
	}
	b.once.Do(func() {
		if b.pool != nil && b.buf != nil {
			pooled := b.buf[:cap(b.buf)]
			b.pool.Put(&pooled)
		}
		if b.releaseBudget != nil {
			b.releaseBudget()
		}
		b.buf = nil
		b.data = nil
	})
}
