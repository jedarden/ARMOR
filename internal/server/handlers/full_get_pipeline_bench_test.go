package handlers_test

// Latency-injected streaming v3 full-GET benchmark for armor-43e2062b.
//
// This is opt-in because it encrypts and reads a 64 MiB object three times at
// a deliberately slow backend RTT. It records TTFB and total handler latency
// for the acceptance sizes while the request-count tests pin the call shape.
//
// Run with:
//
//	ARMOR_FULL_GET_PIPELINE_BENCH=1 go test ./internal/server/handlers/ \
//	  -run TestFullGETPipelineLatency -v -count=1 -timeout 20m

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

type firstByteWriter struct {
	*httptest.ResponseRecorder
	firstByte time.Time
}

func (w *firstByteWriter) Write(p []byte) (int, error) {
	if len(p) > 0 && w.firstByte.IsZero() {
		w.firstByte = time.Now()
	}
	return w.ResponseRecorder.Write(p)
}

func TestFullGETPipelineLatency(t *testing.T) {
	if os.Getenv("ARMOR_FULL_GET_PIPELINE_BENCH") == "" {
		t.Skip("set ARMOR_FULL_GET_PIPELINE_BENCH=1 to run the latency-injected full-GET benchmark")
	}
	if testing.Short() {
		t.Skip("latency benchmark is an explicit long-running test")
	}

	rttMs := 200
	if raw := os.Getenv("ARMOR_FULL_GET_PIPELINE_RTT_MS"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("ARMOR_FULL_GET_PIPELINE_RTT_MS must be positive, got %q", raw)
		}
		rttMs = parsed
	}
	rtt := time.Duration(rttMs) * time.Millisecond
	const bucket = "pipeline-bench-bucket"
	for _, size := range []int{1 << 20, 8 << 20, 64 << 20} {
		size := size
		t.Run(fmt.Sprintf("%dMiB", size>>20), func(t *testing.T) {
			plaintext := smallGETPlaintext(size)
			cfg, cb, h, _ := countingTestSetup(t, 3, rtt)
			cfg.SmallObjectFetchBytes = 0
			key := fmt.Sprintf("v3-%d.bin", size)
			putObjectThroughHandler(t, h, bucket, key, plaintext)
			cb.resetCalls()

			req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
			recorder := httptest.NewRecorder()
			writer := &firstByteWriter{ResponseRecorder: recorder}
			start := time.Now()
			h.HandleRoot(writer, req)
			total := time.Since(start)
			if writer.Code != http.StatusOK {
				t.Fatalf("GET failed: status %d: %s", writer.Code, writer.Body.String())
			}
			if !bytes.Equal(writer.Body.Bytes(), plaintext) {
				t.Fatalf("GET body mismatch: got %d bytes, want %d", writer.Body.Len(), len(plaintext))
			}
			if writer.firstByte.IsZero() {
				t.Fatal("GET emitted no body byte")
			}
			calls := cb.recordedCalls()
			if len(calls) != 4 {
				t.Fatalf("GET made %d calls, want probe pair plus concurrent table/data: %s", len(calls), formatCalls(calls))
			}
			if !cb.rangedCallsOverlapped() {
				t.Fatal("trailer and data ranges did not overlap")
			}
			ttfb := writer.firstByte.Sub(start)
			serialized := 4 * rtt
			t.Logf("RESULT size=%dMiB rtt=%dms ttfb=%s total=%s serialized-baseline≈%s reduction≈%.1fx calls=%s",
				size>>20, rttMs, ttfb.Round(time.Millisecond), total.Round(time.Millisecond), serialized, float64(serialized)/float64(ttfb), formatCalls(calls))
		})
	}
}
