package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// BenchmarkB2BackendGetRangeFetchWindow measures wall-clock throughput of a
// full-object ordered GetRange against a latency-injected loopback server
// across network fetch sizes and read concurrencies (armor-73564f97). The
// injected delay stands in for backend RTT so the request-count amortisation
// of coalesced fetches is visible; loopback transfer time is far below a real
// B2/CF hop, so absolute MB/s here is a floor on the production win, not a
// measurement of it.
func BenchmarkB2BackendGetRangeFetchWindow(b *testing.B) {
	const objectSize = 16 << 20
	object := bytes.Repeat([]byte("0123456789abcdef"), objectSize/16)
	for _, delay := range []time.Duration{0, 40 * time.Millisecond} {
		for _, fetch := range []int64{64 << 10, 1 << 20, 4 << 20, 8 << 20} {
			for _, conc := range []int{1, 4, 16} {
				b.Run(fmt.Sprintf("delay=%s/fetch=%s/conc=%d", delay, kib(fetch), conc), func(b *testing.B) {
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if delay > 0 {
							time.Sleep(delay)
						}
						start, end, err := benchRange(r)
						if err != nil {
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}
						w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(object)))
						w.WriteHeader(http.StatusPartialContent)
						_, _ = w.Write(object[start : end+1])
					}))
					defer server.Close()

					backend := &B2Backend{
						cfDomain:        strings.TrimPrefix(server.URL, "https://"),
						httpClient:      server.Client(),
						readBlockSize:   fetch,
						readConcurrency: conc,
					}
					b.SetBytes(objectSize)
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						body, _, err := backend.GetRangeWithHeaders(context.Background(), "bucket", "key", 0, objectSize)
						if err != nil {
							b.Fatalf("GetRangeWithHeaders: %v", err)
						}
						if _, err := io.Copy(io.Discard, body); err != nil {
							b.Fatalf("read: %v", err)
						}
						body.Close()
					}
				})
			}
		}
	}
}

// benchRange parses the Range header this benchmark's client sends.
func benchRange(r *http.Request) (start, end int64, err error) {
	value := strings.TrimPrefix(r.Header.Get("Range"), "bytes=")
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid Range header %q", r.Header.Get("Range"))
	}
	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	end, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func kib(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + "B"
	}
	return strconv.FormatInt(n>>10, 10) + "KiB"
}
