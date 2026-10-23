package backend

import (
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
)

// Outbound connection pooling for the B2 backend.
//
// Both outbound clients — the Cloudflare read client and the AWS SDK client
// that serves writes, HEADs, direct reads and multipart uploads — used to run
// on transports whose idle-pool retention was far below the backend's own
// request budget: the stdlib default keeps MaxIdleConnsPerHost=2 idle
// connections per host and the SDK default keeps 10. Neither is a concurrency
// cap (in-flight requests are unbounded by these settings); the number bounds
// only how many connections survive between requests. A read pipeline opens
// readConcurrency ranged requests at once and the backend-wide read budget
// admits up to ReadBackendLimit, so with retention below that every burst
// past the pool closed its extra connections after one use and paid a fresh
// TLS handshake on the next burst (~1 RTT for TLS 1.3, more for a full
// handshake).
//
// The pool is therefore sized from the knobs that already govern backend
// traffic: retain one pipeline-width pool (readConcurrency idle per host) and
// never retain more than the backend-wide request budget (readBackendLimit
// idle total). When the peer negotiates HTTP/2 the pool collapses to one or
// two multiplexed connections regardless of these numbers — see
// TestTransportPoolHTTP2 — so the sizing only matters against HTTP/1.1 peers,
// where it eliminates steady-state handshake churn entirely (measured in
// TestTransportPoolHTTP1Bursts and BenchmarkTransportPool).

// newReadHTTPTransport builds the transport used by the Cloudflare read
// client. It is a clone of http.DefaultTransport — proxy-from-environment,
// TLS defaults, 90s idle timeout, 10s handshake timeout and HTTP/2
// negotiation via ForceAttemptHTTP2 are all preserved — with only the
// idle-pool sizes raised from the stdlib defaults.
func newReadHTTPTransport(idlePerHost, idleTotal int) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.MaxIdleConnsPerHost = idlePerHost
	tr.MaxIdleConns = idleTotal
	return tr
}

// newS3HTTPClient wraps the AWS SDK's default buildable client with the same
// idle-pool sizing. Everything the SDK configures by default (proxy from
// environment, TLS floor 1.2, 30s dial / 10s handshake timeouts, 90s idle
// timeout, MaxConnsPerHost=2048 hard cap, HTTP/2 attempt) is preserved; only
// idle retention changes. Retries, checksums and cancellation are SDK-level
// concerns and are untouched.
func newS3HTTPClient(idlePerHost, idleTotal int) *awshttp.BuildableClient {
	return awshttp.NewBuildableClient().WithTransportOptions(func(t *http.Transport) {
		t.MaxIdleConnsPerHost = idlePerHost
		t.MaxIdleConns = idleTotal
	})
}
