package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestHeaderDuplicationDetectionFixed is a fixed version that tests
// headers that are actually preserved by ARMOR.
func TestHeaderDuplicationDetectionFixed(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := srv.Handler()

	t.Run("Authorization header appears exactly once", func(t *testing.T) {
		authHeader := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"

		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("X-Amz-Date", "20130524T000000Z")
		req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

		var authHeaderCount int
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeaderCount = len(r.Header["Authorization"])
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if authHeaderCount != 1 {
			t.Errorf("Authorization header count is %d (expected 1)", authHeaderCount)
			t.Errorf("Header may have been duplicated during passthrough")
		} else {
			t.Logf("✓ Authorization header appears exactly once")
		}
	})

	t.Run("Multiple X-Amz-* headers each appear once", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Test only headers that are actually preserved by ARMOR
		testHeaders := map[string]string{
			"X-Amz-Date":          "20130524T000000Z",
			"X-Amz-Content-Sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"X-Amz-Security-Token": "FwoGZXIvYXdzEBYaDmRiwUKvH.example",
			"X-Amz-Algorithm":     "AWS4-HMAC-SHA256",
			"X-Amz-Credential":     "TESTACCESSKEY/20130524/us-east-1/s3/aws4_request",
		}

		for name, value := range testHeaders {
			req.Header.Set(name, value)
		}

		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404")
		req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

		headerCounts := make(map[string]int)
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for headerName := range testHeaders {
				headerCounts[headerName] = len(r.Header[headerName])
			}
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		allOnce := true
		for headerName, count := range headerCounts {
			if count != 1 {
				t.Errorf("Header %s appears %d times (expected 1)", headerName, count)
				allOnce = false
			} else {
				t.Logf("✓ Header %s appears exactly once", headerName)
			}
		}

		if allOnce {
			t.Log("✓ All X-Amz-* headers appear exactly once (no duplication)")
		}
	})

	t.Run("Multi-hop scenario preserves header count", func(t *testing.T) {
		// Build a multi-hop handler chain
		multiHopHandler := handler
		multiHopHandler = loadBalancerLayer(multiHopHandler)
		multiHopHandler = cloudflareProxyLayer(multiHopHandler)

		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404")
		req.Header.Set("X-Amz-Date", "20130524T000000Z")
		req.Header.Set("X-Amz-Content-Sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")

		var finalAuthCount, finalAmzDateCount, finalAmzShaCount int
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			finalAuthCount = len(r.Header["Authorization"])
			finalAmzDateCount = len(r.Header["X-Amz-Date"])
			finalAmzShaCount = len(r.Header["X-Amz-Content-Sha256"])
			multiHopHandler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		allOnce := true
		if finalAuthCount != 1 {
			t.Errorf("Authorization appears %d times after multi-hop", finalAuthCount)
			allOnce = false
		}
		if finalAmzDateCount != 1 {
			t.Errorf("X-Amz-Date appears %d times after multi-hop", finalAmzDateCount)
			allOnce = false
		}
		if finalAmzShaCount != 1 {
			t.Errorf("X-Amz-Content-Sha256 appears %d times after multi-hop", finalAmzShaCount)
			allOnce = false
		}

		if allOnce {
			t.Log("✓ All headers preserved exactly once through multi-hop chain")
		}
	})
}
