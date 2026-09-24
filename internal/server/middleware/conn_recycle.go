package middleware

import (
	"net/http"
	"sync"
	"time"
)

const (
	// connStatsStaleAfter bounds how long a connection's tracking entry
	// survives after its last request. The S3 listener reaps idle
	// connections at 2 minutes, so an entry idle this long belongs to a
	// connection that is certainly closed; only its counter would remain.
	connStatsStaleAfter = 10 * time.Minute

	// connSweepInterval is how often the tracking map is swept for stale
	// entries under steady load.
	connSweepInterval = connStatsStaleAfter

	// connStatsMaxEntries is the map size above which a sweep runs even if
	// the sweep interval has not elapsed, so a burst of short-lived
	// connections cannot grow the map without bound between sweeps.
	connStatsMaxEntries = 4096
)

// connStats tracks one keep-alive connection. Requests on an HTTP/1.1
// connection are sequential, so these fields are only ever touched by the
// one goroutine serving that connection; the mutex protects the map itself.
type connStats struct {
	requests int
	started  time.Time
	lastSeen time.Time
}

// ConnRecycler tells clients to close and re-dial keep-alive connections once
// a connection has served more than maxRequests requests or lived longer than
// maxAge.
//
// Why: kube-proxy load-balances per TCP connection, so every connection a
// client keeps alive pins it to one ARMOR pod for as long as it stays open.
// Long-lived client connection pools (boto3, rclone) therefore skew traffic
// across replicas, and adding replicas does not redistribute connections that
// already exist. Recycling a connection makes the client re-dial, and the new
// connection is balanced afresh.
//
// The mechanism is the `Connection: close` response header, which net/http
// both forwards to the client and honors by closing the connection after the
// response is written. The close therefore always lands on a response
// boundary: a request that has begun -- a streamed download, a multipart
// completion, a single part upload -- always completes normally, and clients
// close their end cleanly and re-dial, rather than seeing a mid-connection
// reset.
type ConnRecycler struct {
	maxRequests int
	maxAge      time.Duration

	// now is swappable so tests can drive the age threshold deterministically.
	now func() time.Time

	mu        sync.Mutex
	conns     map[string]*connStats
	lastSweep time.Time
}

// NewConnRecycler builds a ConnRecycler. maxRequests is the number of requests
// a connection serves before the next one is told to close (0 disables the
// request-count threshold); maxAge is how old a connection may get before its
// next request is told to close (0 disables the age threshold). With both
// disabled the recycler is inert and passes handlers through unchanged.
func NewConnRecycler(maxRequests int, maxAge time.Duration) *ConnRecycler {
	now := time.Now()
	return &ConnRecycler{
		maxRequests: maxRequests,
		maxAge:      maxAge,
		now:         time.Now,
		conns:       make(map[string]*connStats),
		lastSweep:   now,
	}
}

// Wrap returns next with connection recycling applied. With both thresholds
// disabled it returns next unchanged.
func (cr *ConnRecycler) Wrap(next http.Handler) http.Handler {
	if !cr.enabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cr.shouldRecycle(r.RemoteAddr) {
			// net/http sends this header to the client and closes the
			// connection once this response is fully written.
			w.Header().Set("Connection", "close")
		}
		next.ServeHTTP(w, r)
	})
}

// enabled reports whether either recycling threshold is set.
func (cr *ConnRecycler) enabled() bool {
	return cr.maxRequests > 0 || cr.maxAge > 0
}

// shouldRecycle reports whether the connection the request arrived on has
// passed a recycling threshold, counting the request as it goes. The first
// request seen on a connection starts its age, which is within milliseconds
// of the connection being accepted.
func (cr *ConnRecycler) shouldRecycle(remoteAddr string) bool {
	now := cr.now()

	cr.mu.Lock()
	defer cr.mu.Unlock()

	if len(cr.conns) >= connStatsMaxEntries || now.Sub(cr.lastSweep) >= connSweepInterval {
		cr.sweepLocked(now)
	}

	cs := cr.conns[remoteAddr]
	if cs == nil {
		cs = &connStats{started: now}
		cr.conns[remoteAddr] = cs
	}
	cs.requests++
	cs.lastSeen = now

	// Strictly greater: the threshold is "this many requests served
	// cleanly", and the request after them is the one told to close.
	if cr.maxRequests > 0 && cs.requests > cr.maxRequests {
		return true
	}
	if cr.maxAge > 0 && now.Sub(cs.started) >= cr.maxAge {
		return true
	}
	return false
}

// sweepLocked drops tracking entries idle long enough that their connections
// are certainly closed. Callers must hold cr.mu.
func (cr *ConnRecycler) sweepLocked(now time.Time) {
	for addr, cs := range cr.conns {
		if now.Sub(cs.lastSeen) >= connStatsStaleAfter {
			delete(cr.conns, addr)
		}
	}
	cr.lastSweep = now
}
