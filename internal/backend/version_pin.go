package backend

import (
	"context"
	"sync"
	"sync/atomic"
)

// A VersionPin records, for the lifetime of one client request, the B2 version
// (file ID) that each key's HEAD observed. The HEAD goes straight to B2 and is
// authoritative; the body is then read through Cloudflare, whose edge may still
// hold the bytes of a version that has since been overwritten. Comparing the
// version Cloudflare served with the pinned one is what lets the read path tell
// a stale edge copy from a good one.
//
// The pin is per request, never shared: a process-wide "latest known version"
// would hand request A the version request B saw after an overwrite and mix the
// metadata of one version with the bytes of another.
type VersionPin struct {
	mu       sync.Mutex
	versions map[string]string
}

type versionPinKey struct{}

// WithVersionPin returns a context carrying an empty pin. Only the server's
// request path creates one; every other caller (armor decrypt, the verifiers)
// reads without a pin and sees the behavior it always had.
func WithVersionPin(ctx context.Context) context.Context {
	return context.WithValue(ctx, versionPinKey{}, &VersionPin{versions: make(map[string]string)})
}

func versionPinFrom(ctx context.Context) *VersionPin {
	p, _ := ctx.Value(versionPinKey{}).(*VersionPin)
	return p
}

func pinKey(bucket, key string) string { return bucket + "\x00" + key }

// recordVersionPin keeps the first version seen for a key: that HEAD is the one
// the handler decodes the object's metadata from.
func recordVersionPin(ctx context.Context, bucket, key, versionID string) {
	if versionID == "" {
		return
	}
	p := versionPinFrom(ctx)
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.versions[pinKey(bucket, key)]; !ok {
		p.versions[pinKey(bucket, key)] = versionID
	}
}

func pinnedVersion(ctx context.Context, bucket, key string) string {
	p := versionPinFrom(ctx)
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.versions[pinKey(bucket, key)]
}

var cfStaleFallbacks atomic.Int64

// CFStaleFallbackCount reports how many ranged reads were re-fetched directly
// from B2 because Cloudflare served a version other than the pinned one.
func CFStaleFallbackCount() int64 { return cfStaleFallbacks.Load() }
