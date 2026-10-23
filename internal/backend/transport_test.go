package backend

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
)

// countingTLSStats records how many TCP connections the server accepted and
// how many TLS handshakes it performed. On a keep-alive client without
// session resumption (the transports here set no ClientSessionCache), every
// new connection costs exactly one handshake, so "handshakes stop growing"
// is the steady-state reuse signature the connection-pool tuning is after.
type countingTLSStats struct {
	conns      atomic.Int64
	handshakes atomic.Int64
}

// newCountingTLSServer starts a TLS test server that counts handshakes and
// accepted connections. Pass enableHTTP2 to negotiate h2 via ALPN.
func newCountingTLSServer(tb testing.TB, handler http.HandlerFunc, enableHTTP2 bool) (*httptest.Server, *countingTLSStats) {
	tb.Helper()
	stats := &countingTLSStats{}
	srv := httptest.NewUnstartedServer(handler)
	if enableHTTP2 {
		srv.EnableHTTP2 = true
	}
	srv.TLS = &tls.Config{
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			stats.handshakes.Add(1)
			return nil, nil
		},
	}
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			stats.conns.Add(1)
		}
	}
	srv.StartTLS()
	tb.Cleanup(srv.Close)
	return srv, stats
}

// insecureReadTransport points a read transport at self-signed test servers.
// The TLS config is cloned from the one the transport produced — including
// its NextProtos, so HTTP/2 negotiation stays live exactly as it is in
// production — with only the trust check relaxed for the test certificate.
func insecureReadTransport(tr *http.Transport) *http.Transport {
	cfg := tr.TLSClientConfig.Clone()
	if cfg == nil {
		cfg = &tls.Config{NextProtos: []string{"h2", "http/1.1"}}
	}
	cfg.InsecureSkipVerify = true //nolint:gosec // httptest self-signed cert
	tr.TLSClientConfig = cfg
	return tr
}

// poolTestTransport returns a client transport built by the production
// builder, relaxed for self-signed test servers.
func poolTestTransport(idlePerHost, idleTotal int) *http.Transport {
	return insecureReadTransport(newReadHTTPTransport(idlePerHost, idleTotal))
}

// insecureSDKClient relaxes an SDK buildable client the same way, preserving
// every other default the SDK configures.
func insecureSDKClient(bc *awshttp.BuildableClient) *awshttp.BuildableClient {
	return bc.WithTransportOptions(func(t *http.Transport) {
		if t.TLSClientConfig == nil {
			t.TLSClientConfig = &tls.Config{}
		}
		t.TLSClientConfig.InsecureSkipVerify = true //nolint:gosec // httptest self-signed cert
	})
}

// driveBursts performs rounds bursts of width concurrent GETs, draining every
// body before the round completes so connections return to the pool. It
// returns the cumulative handshake count observed after each round.
func driveBursts(t *testing.T, client *http.Client, target string, stats *countingTLSStats, rounds, width int) []int64 {
	t.Helper()
	cumulative := make([]int64, 0, rounds)
	for round := 0; round < rounds; round++ {
		var wg sync.WaitGroup
		for i := 0; i < width; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
				if err != nil {
					t.Errorf("request: %v", err)
					return
				}
				resp, err := client.Do(req)
				if err != nil {
					t.Errorf("do: %v", err)
					return
				}
				if _, err := io.Copy(io.Discard, resp.Body); err != nil {
					t.Errorf("body: %v", err)
				}
				resp.Body.Close()
			}()
		}
		wg.Wait()
		// Let the read loops return every connection to the idle pool before
		// the next round samples the counters; without this the first rounds
		// after warmup race the stragglers and dial replacements.
		time.Sleep(25 * time.Millisecond)
		cumulative = append(cumulative, stats.handshakes.Load())
	}
	return cumulative
}

// TestTransportPoolHTTP1Bursts shows that a pool sized to the burst width
// reuses every connection in steady state. The stdlib-default arm is
// descriptive: a 2-deep pool forces waves of requests through surviving
// connections, so its handshake count depends on delivery timing and is
// logged for the record rather than asserted; BenchmarkTransportPool
// quantifies the churn difference under sustained load instead.
func TestTransportPoolHTTP1Bursts(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(make([]byte, 128))
	})

	cases := []struct {
		name                   string
		idlePerHost, idleTotal int
	}{
		{"stdlib-default", 2, 100},
		{"sized", defaultReadConcurrency, defaultReadBackendLimit},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No EnableHTTP2: pool sizing is the HTTP/1.1 knob.
			srv, stats := newCountingTLSServer(t, handler, false)
			client := &http.Client{Transport: poolTestTransport(tc.idlePerHost, tc.idleTotal)}

			const rounds, width = 6, 16
			cumulative := driveBursts(t, client, srv.URL+"/file/bucket/key", stats, rounds, width)

			for i, v := range cumulative {
				t.Logf("round %d: cumulative handshakes=%d conns=%d", i+1, v, stats.conns.Load())
			}
			if cumulative[0] == 0 {
				t.Fatal("no handshakes recorded; counter wiring broken")
			}

			if tc.name != "sized" {
				return
			}
			// The warm pool covers the whole burst width, so once the pool
			// has settled the rounds must run handshake-free: three
			// consecutive rounds with zero new handshakes is the steady-state
			// reuse signature. (The default arm is not held to this — a 2-deep
			// pool serializes bursts into waves, so its handshake count
			// depends on delivery timing and is recorded, not asserted.)
			settled := false
			for i := 1; i+2 < len(cumulative); i++ {
				if cumulative[i] == cumulative[i+1] && cumulative[i+1] == cumulative[i+2] {
					settled = true
					break
				}
			}
			if !settled {
				t.Fatalf("no steady state: handshakes kept growing across bursts: %v", cumulative)
			}
		})
	}
}

// TestTransportPoolHTTP2 documents when transport tuning has no material
// effect: against an HTTP/2 server everything multiplexes over the surviving
// connection(s) regardless of the idle-pool sizing, and steady state is
// handshake-free there too.
//
// The pool is warmed with one sequential request first. That pins the
// negotiated protocol — tuning must not have cost ARMOR its h2 — and
// establishes the connection a cold pool lacks: Go's HTTP/2 client has no
// dial coalescing, so every concurrent first request dials its own
// connection. Cold-start connection counts are therefore recorded, not
// asserted; the invariant under test is reuse after warmup.
func TestTransportPoolHTTP2(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(make([]byte, 128))
	})
	srv, stats := newCountingTLSServer(t, handler, true)
	client := &http.Client{Transport: poolTestTransport(defaultReadConcurrency, defaultReadBackendLimit)}

	warm, err := client.Get(srv.URL + "/file/bucket/key")
	if err != nil {
		t.Fatalf("warmup: %v", err)
	}
	if warm.ProtoMajor != 2 {
		t.Fatalf("warmup negotiated HTTP/%d.%d, want HTTP/2", warm.ProtoMajor, warm.ProtoMinor)
	}
	_, _ = io.Copy(io.Discard, warm.Body)
	warm.Body.Close()
	t.Logf("warmup: negotiated HTTP/%d.%d, handshakes=%d", warm.ProtoMajor, warm.ProtoMinor, stats.handshakes.Load())

	const rounds, width = 4, 32
	cumulative := driveBursts(t, client, srv.URL+"/file/bucket/key", stats, rounds, width)

	for i, v := range cumulative {
		t.Logf("round %d: cumulative handshakes=%d conns=%d", i+1, v, stats.conns.Load())
	}
	// The first concurrent burst may still dial (cold-start dials beyond the
	// warmup connection); every round after it must be handshake-free — once
	// the multiplexed pool exists, its sizing never comes into play.
	settled := false
	for i := 1; i+2 < len(cumulative); i++ {
		if cumulative[i] == cumulative[i+1] && cumulative[i+1] == cumulative[i+2] {
			settled = true
			break
		}
	}
	if !settled {
		t.Fatalf("no h2 steady state: handshakes kept growing across bursts: %v", cumulative)
	}
}

// TestNewB2BackendTransportSizing verifies both outbound clients are built
// with the pool sized from the backend's own read knobs, and that every
// default that must be preserved (proxy, HTTP/2, SDK hard caps, TLS floor)
// is.
func TestNewB2BackendTransportSizing(t *testing.T) {
	backend, err := NewB2Backend(context.Background(), B2Config{
		Region:           "us-east-005",
		Endpoint:         "https://s3.us-east-005.backblazeb2.com",
		AccessKeyID:      "test-key",
		SecretKey:        "test-secret",
		CFDomain:         "f00baa.example.com",
		ReadConcurrency:  24,
		ReadBackendLimit: 40,
	})
	if err != nil {
		t.Fatalf("NewB2Backend: %v", err)
	}

	cf, ok := backend.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("CF transport is %T, want *http.Transport", backend.httpClient.Transport)
	}
	if cf.MaxIdleConnsPerHost != 24 {
		t.Errorf("CF MaxIdleConnsPerHost = %d, want 24 (ReadConcurrency)", cf.MaxIdleConnsPerHost)
	}
	if cf.MaxIdleConns != 40 {
		t.Errorf("CF MaxIdleConns = %d, want 40 (ReadBackendLimit)", cf.MaxIdleConns)
	}
	if !cf.ForceAttemptHTTP2 {
		t.Error("CF transport lost ForceAttemptHTTP2")
	}
	if cf.Proxy == nil {
		t.Error("CF transport lost proxy-from-environment")
	}
	if cf.IdleConnTimeout != 90*time.Second {
		t.Errorf("CF IdleConnTimeout = %v, want the DefaultTransport 90s", cf.IdleConnTimeout)
	}

	sdk, ok := backend.s3Client.Options().HTTPClient.(*awshttp.BuildableClient)
	if !ok {
		t.Fatalf("SDK HTTPClient is %T, want *awshttp.BuildableClient", backend.s3Client.Options().HTTPClient)
	}
	st := sdk.GetTransport()
	if st.MaxIdleConnsPerHost != 24 {
		t.Errorf("SDK MaxIdleConnsPerHost = %d, want 24", st.MaxIdleConnsPerHost)
	}
	if st.MaxIdleConns != 40 {
		t.Errorf("SDK MaxIdleConns = %d, want 40", st.MaxIdleConns)
	}
	if !st.ForceAttemptHTTP2 {
		t.Error("SDK transport lost ForceAttemptHTTP2")
	}
	if st.Proxy == nil {
		t.Error("SDK transport lost proxy-from-environment")
	}
	if st.MaxConnsPerHost != awshttp.DefaultHTTPTransportMaxConnsPerHost {
		t.Errorf("SDK MaxConnsPerHost = %d, want the SDK default %d", st.MaxConnsPerHost, awshttp.DefaultHTTPTransportMaxConnsPerHost)
	}
	if st.TLSClientConfig == nil || st.TLSClientConfig.MinVersion != awshttp.DefaultHTTPTransportTLSMinVersion {
		t.Error("SDK transport lost its TLS minimum version")
	}

	// Zero config falls back to the same defaults the read path uses.
	defaultSized, err := NewB2Backend(context.Background(), B2Config{
		Region: "us-east-005", AccessKeyID: "k", SecretKey: "s",
	})
	if err != nil {
		t.Fatalf("NewB2Backend(defaults): %v", err)
	}
	dcf := defaultSized.httpClient.Transport.(*http.Transport)
	if dcf.MaxIdleConnsPerHost != defaultReadConcurrency || dcf.MaxIdleConns != defaultReadBackendLimit {
		t.Errorf("default sizing = %d/%d, want %d/%d", dcf.MaxIdleConnsPerHost, dcf.MaxIdleConns, defaultReadConcurrency, defaultReadBackendLimit)
	}
}

// Bench payloads are package-level so the benchmark measures request cost,
// not payload construction. 64 KiB is the order of a small ranged fetch and
// a small uploaded part — large enough that per-op cost is dominated by the
// connection, not the framework.
var (
	benchReadPayload  = bytes.Repeat([]byte{0xa5}, 64<<10)
	benchWritePayload = bytes.Repeat([]byte{0x5a}, 64<<10)
)

// BenchmarkTransportPool measures the two outbound request shapes the B2
// backend drives — reads (GET, response drained) through the CF transport
// and writes (PUT with a replayable *bytes.Reader body, so ContentLength
// and GetBody are set the way the SDK's retry path expects) through the SDK
// buildable client — each with the previous default pool and the sized pool,
// over HTTP/1.1 and HTTP/2.
//
// Concurrency is fixed at defaultReadConcurrency workers, matching the
// backend's own read pipeline rather than machine parallelism. The
// handshakes/op metric is the churn measure: on HTTP/1.1 it stays far above
// zero for the undersized default pools (every burst past the pool closes
// its surplus connections and pays a fresh handshake next burst) and
// collapses to ~0 for the sized pool. On HTTP/2 both arms multiplex over
// surviving connections and the metric is ~0 either way — the documented
// case where transport tuning has no material effect.
func BenchmarkTransportPool(b *testing.B) {
	protos := []struct {
		name string
		h2   bool
	}{{"h1", false}, {"h2", true}}

	b.Run("reads", func(b *testing.B) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(benchReadPayload)
		})
		arms := []struct {
			name string
			tr   func() http.RoundTripper
		}{
			{"default", func() http.RoundTripper {
				return insecureReadTransport(http.DefaultTransport.(*http.Transport).Clone())
			}},
			{"sized", func() http.RoundTripper {
				return insecureReadTransport(newReadHTTPTransport(defaultReadConcurrency, defaultReadBackendLimit))
			}},
		}
		for _, proto := range protos {
			for _, arm := range arms {
				b.Run(proto.name+"/"+arm.name, func(b *testing.B) {
					srv, stats := newCountingTLSServer(b, handler, proto.h2)
					client := &http.Client{Transport: arm.tr()}
					target := srv.URL + "/file/bucket/key"
					runPoolOps(b, stats, func() error {
						resp, err := client.Get(target)
						if err != nil {
							return err
						}
						_, err = io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						return err
					})
				})
			}
		}
	})

	b.Run("writes", func(b *testing.B) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		})
		arms := []struct {
			name string
			bc   func() *awshttp.BuildableClient
		}{
			{"default", func() *awshttp.BuildableClient {
				return insecureSDKClient(awshttp.NewBuildableClient())
			}},
			{"sized", func() *awshttp.BuildableClient {
				return insecureSDKClient(newS3HTTPClient(defaultReadConcurrency, defaultReadBackendLimit))
			}},
		}
		for _, proto := range protos {
			for _, arm := range arms {
				b.Run(proto.name+"/"+arm.name, func(b *testing.B) {
					srv, stats := newCountingTLSServer(b, handler, proto.h2)
					client := arm.bc()
					target := srv.URL + "/bucket/key"
					runPoolOps(b, stats, func() error {
						// *bytes.Reader makes the request replayable:
						// net/http sets ContentLength and GetBody from it,
						// the shape the SDK's retry path depends on for PUT
						// and UploadPart.
						req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(benchWritePayload))
						if err != nil {
							return err
						}
						resp, err := client.Do(req)
						if err != nil {
							return err
						}
						_, err = io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
						return err
					})
				})
			}
		}
	})
}

// runPoolOps drives op at defaultReadConcurrency-way concurrency — the
// backend's own pipeline width — and reports the server-observed connection
// and handshake counts per operation.
func runPoolOps(b *testing.B, stats *countingTLSStats, op func() error) {
	b.Helper()
	b.ResetTimer()
	var next atomic.Int64
	var wg sync.WaitGroup
	wg.Add(defaultReadConcurrency)
	for i := 0; i < defaultReadConcurrency; i++ {
		go func() {
			defer wg.Done()
			for next.Add(1) <= int64(b.N) {
				if err := op(); err != nil {
					b.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
	b.StopTimer()
	b.ReportMetric(float64(stats.handshakes.Load())/float64(b.N), "handshakes/op")
	b.ReportMetric(float64(stats.conns.Load()), "conns-total")
}
