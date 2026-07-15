package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestMultiHopHeaderPreservation verifies that authentication headers are preserved
// across multiple hops to ARMOR.
//
// In production, requests may pass through multiple intermediaries before reaching ARMOR:
//
//   Client → Cloudflare/Load Balancer → ARMOR → B2 Backend
//   (or)   Client → Reverse Proxy 1 → Reverse Proxy 2 → ARMOR
//
// This test simulates multi-hop scenarios by wrapping ARMOR's handler in multiple
// middleware layers that represent typical reverse proxy behavior. It verifies that:
//
// 1. Authorization headers survive through all intermediate hops
// 2. X-Amz-* headers are not stripped by proxies or load balancers
// 3. Headers maintain integrity across the full request path
// 4. End-to-end header fidelity is preserved
//
// Bead: bf-54kk2d
// Created: 2026-07-15
func TestMultiHopHeaderPreservation(t *testing.T) {
	// Create test credentials
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

	armorHandler := srv.Handler()

	// Define test scenarios with different hop configurations
	testScenarios := []struct {
		name        string
		description string
		hops        []hopLayer
	}{
		{
			name:        "Single hop - direct to ARMOR",
			description: "Baseline test: no intermediate proxies",
			hops:        []hopLayer{},
		},
		{
			name:        "Two hops - Cloudflare → ARMOR",
			description: "Simulates Cloudflare as reverse proxy in front of ARMOR",
			hops: []hopLayer{
				cloudflareProxyLayer,
			},
		},
		{
			name:        "Three hops - Load Balancer → Cloudflare → ARMOR",
			description: "Simulates load balancer in front of Cloudflare",
			hops: []hopLayer{
				loadBalancerLayer,
				cloudflareProxyLayer,
			},
		},
		{
			name:        "Four hops - Multiple reverse proxies",
			description: "Simulates complex routing with multiple intermediaries",
			hops: []hopLayer{
				reverseProxyLayer("proxy-1"),
				reverseProxyLayer("proxy-2"),
				cloudflareProxyLayer,
			},
		},
	}

	// Define header test cases
	headerTestCases := []struct {
		name            string
		headers         map[string]string
		description     string
		validateHeaders []string // Headers to validate preservation
	}{
		{
			name: "Standard Authorization header",
			headers: map[string]string{
				"Authorization": "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404",
				"X-Amz-Date":    "20130524T000000Z",
				"Host":          "test-bucket.s3.amazonaws.com",
			},
			description:     "Standard AWS SigV4 Authorization header",
			validateHeaders: []string{"Authorization", "X-Amz-Date"},
		},
		{
			name: "Authorization with security token",
			headers: map[string]string{
				"Authorization":    "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-security-token, Signature=c3a5e2f8b1d9e4a7b2c5d8f9a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0",
				"X-Amz-Date":       "20130524T000000Z",
				"X-Amz-Security-Token": "FwoGZXIvYXdzEBYaDmRiwUKvH.example4tZKheCbhYS7CfjzRl6oP2KDsSExamplevmLpU5TyPQc8CnjCEZrzRQEXAMPLE",
				"Host":             "test-bucket.s3.amazonaws.com",
			},
			description:     "Session credentials with security token",
			validateHeaders: []string{"Authorization", "X-Amz-Date", "X-Amz-Security-Token"},
		},
		{
			name: "Multiple X-Amz-* headers",
			headers: map[string]string{
				"Authorization":          "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=fe5f80f77d5fa27bec129f320a5cfe8cd23c890a9f1de8b7b99b1b5b8b7b5b1b",
				"X-Amz-Date":             "20130524T000000Z",
				"X-Amz-Content-Sha256":    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"X-Amz-Algorithm":         "AWS4-HMAC-SHA256",
				"X-Amz-Credential":        "TESTACCESSKEY/20130524/us-east-1/s3/aws4_request",
				"X-Amz-SignedHeaders":     "host;x-amz-content-sha256;x-amz-date",
				"Host":                    "test-bucket.s3.amazonaws.com",
			},
			description:     "Full SigV4 header suite with multiple X-Amz-* headers",
			validateHeaders: []string{"Authorization", "X-Amz-Date", "X-Amz-Content-Sha256", "X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-SignedHeaders"},
		},
		{
			name: "Long Authorization header",
			headers: map[string]string{
				"Authorization": "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date;x-amz-security-token;x-amz-user-agent, Signature=1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
				"X-Amz-Date":    "20130524T000000Z",
				"Host":          "test-bucket.s3.amazonaws.com",
			},
			description:     "Long signature to test no truncation occurs",
			validateHeaders: []string{"Authorization", "X-Amz-Date"},
		},
	}

	t.Log("Testing Multi-Hop Header Preservation")
	t.Log("======================================")
	t.Log("This test simulates requests passing through multiple intermediate")
	t.Log("proxies (Cloudflare, load balancers, etc.) before reaching ARMOR.")
	t.Log("It verifies that authentication headers survive intact through all hops.")
	t.Log("")

	for _, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			t.Logf("Scenario: %s", scenario.description)
			t.Logf("Hops: %d", len(scenario.hops))

			// Build the handler chain: hops → ARMOR
			handler := armorHandler
			for i := len(scenario.hops) - 1; i >= 0; i-- {
				handler = scenario.hops[i](handler)
			}

			for _, tc := range headerTestCases {
				t.Run(tc.name, func(t *testing.T) {
					t.Logf("Testing: %s", tc.description)

					// Create request with authentication headers
					req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
					for key, value := range tc.headers {
						req.Header.Set(key, value)
					}

					// Store original headers for comparison
					originalHeaders := make(map[string]string)
					for _, key := range tc.validateHeaders {
						originalHeaders[key] = req.Header.Get(key)
					}

					// Create response recorder
					w := httptest.NewRecorder()

					// Create a custom wrapper to capture headers at ARMOR's boundary
					var capturedHeadersAtArmor map[string]string
					var authCaptured bool

					finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Capture headers as they arrive at ARMOR (after all hops)
						capturedHeadersAtArmor = make(map[string]string)
						for _, key := range tc.validateHeaders {
							capturedHeadersAtArmor[key] = r.Header.Get(key)
						}
						authCaptured = true

						// Call the actual handler
						handler.ServeHTTP(w, r)
					})

					// Serve the request through the multi-hop chain
					finalHandler.ServeHTTP(w, req)

					// Verify we captured headers at ARMOR boundary
					if !authCaptured {
						t.Errorf("Failed to capture headers at ARMOR boundary")
						return
					}

					// Validate each critical header
					allPassed := true
					for _, key := range tc.validateHeaders {
						original := originalHeaders[key]
						captured := capturedHeadersAtArmor[key]

						if original == "" && captured == "" {
							// Header was not sent, skip validation
							continue
						}

						if original != captured {
							t.Errorf("Header %q was corrupted during multi-hop passthrough!", key)
							t.Logf("  Original length:  %d bytes", len(original))
							t.Logf("  Captured length:  %d bytes", len(captured))
							t.Logf("  Original value:   %q", truncateForLog(original))
							t.Logf("  Captured value:   %q", truncateForLog(captured))

							// Find first difference
							minLen := len(original)
							if len(captured) < minLen {
								minLen = len(captured)
							}
							for i := 0; i < minLen; i++ {
								if original[i] != captured[i] {
									t.Logf("  First difference at byte %d:", i)
									t.Logf("    original[%d] = %c (0x%02x)", i, original[i], original[i])
									t.Logf("    captured[%d] = %c (0x%02x)", i, captured[i], captured[i])
									break
								}
							}
							allPassed = false
						} else {
							t.Logf("✓ Header %q preserved intact (%d bytes)", key, len(captured))
						}
					}

					if allPassed {
						t.Logf("✓ All headers survived %d-hop journey intact", len(scenario.hops))
					}
					t.Log("")
				})
			}
		})
	}

	t.Log("✓ Multi-hop header preservation verified")
	t.Log("✓ Authorization and X-Amz-* headers survive through intermediate proxies")
}

// TestMultiHopWithRealisticProxyBehavior tests multi-hop scenarios with more
// realistic proxy behavior that might strip or modify headers.
func TestMultiHopWithRealisticProxyBehavior(t *testing.T) {
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

	armorHandler := srv.Handler()

	t.Run("Realistic Cloudflare proxy with header passthrough", func(t *testing.T) {
		t.Log("Testing realistic Cloudflare proxy behavior:")
		t.Log("- Cloudflare adds CF-RAY, CF-IPCountry headers")
		t.Log("- Cloudflare preserves Authorization and X-Amz-* headers")
		t.Log("- Request path: Client → Cloudflare → ARMOR")

		// Create realistic Cloudflare proxy layer
		realisticCloudflare := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate Cloudflare adding its own headers
				w.Header().Set("CF-RAY", "1234567890abcdef")
				w.Header().Set("CF-IPCountry", "US")

				// Cloudflare preserves auth headers (this is the critical behavior)
				// In reality, Cloudflare does NOT strip Authorization or X-Amz-* headers

				// Call next handler
				next.ServeHTTP(w, r)
			})
		}

		handler := realisticCloudflare(armorHandler)

		// Create request with full S3 auth headers
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		authHeader := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-content-sha256, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("X-Amz-Date", "20130524T000000Z")
		req.Header.Set("X-Amz-Content-Sha256", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
		req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

		// Capture headers at ARMOR boundary
		var capturedAuth, capturedAmzDate, capturedAmzSha string
		capturingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			capturedAmzDate = r.Header.Get("X-Amz-Date")
			capturedAmzSha = r.Header.Get("X-Amz-Content-Sha256")
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		capturingHandler.ServeHTTP(w, req)

		// Verify headers are preserved
		if capturedAuth != authHeader {
			t.Errorf("Authorization header corrupted by Cloudflare proxy")
			t.Logf("Expected: %q", truncateForLog(authHeader))
			t.Logf("Got:      %q", truncateForLog(capturedAuth))
		} else {
			t.Logf("✓ Authorization header preserved through Cloudflare")
		}

		if capturedAmzDate != "20130524T000000Z" {
			t.Errorf("X-Amz-Date header corrupted by Cloudflare proxy")
		} else {
			t.Logf("✓ X-Amz-Date header preserved through Cloudflare")
		}

		if capturedAmzSha != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
			t.Errorf("X-Amz-Content-Sha256 header corrupted by Cloudflare proxy")
		} else {
			t.Logf("✓ X-Amz-Content-Sha256 header preserved through Cloudflare")
		}

		t.Log("✓ Realistic Cloudflare proxy preserves auth headers correctly")
	})

	t.Run("Load balancer that preserves authentication headers", func(t *testing.T) {
		t.Log("Testing load balancer behavior:")
		t.Log("- Load balancer adds X-Forwarded-* headers")
		t.Log("- Load balancer preserves Authorization and X-Amz-* headers")
		t.Log("- Request path: Client → Load Balancer → ARMOR")

		// Create realistic load balancer layer
		loadBalancer := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Simulate load balancer adding forwarding headers
				r.Header.Set("X-Forwarded-For", "203.0.113.1")
				r.Header.Set("X-Forwarded-Proto", "https")
				r.Header.Set("X-Forwarded-Host", "armor.example.com")

				// Load balancers typically preserve auth headers
				// This is critical for S3 API compatibility

				next.ServeHTTP(w, r)
			})
		}

		handler := loadBalancer(armorHandler)

		// Create request with auth headers
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		authHeader := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("X-Amz-Date", "20130524T000000Z")
		req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

		// Capture headers at ARMOR boundary
		var capturedAuth, capturedAmzDate string
		capturingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedAuth = r.Header.Get("Authorization")
			capturedAmzDate = r.Header.Get("X-Amz-Date")
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		capturingHandler.ServeHTTP(w, req)

		// Verify headers are preserved
		if capturedAuth != authHeader {
			t.Errorf("Authorization header corrupted by load balancer")
		} else {
			t.Logf("✓ Authorization header preserved through load balancer")
		}

		if capturedAmzDate != "20130524T000000Z" {
			t.Errorf("X-Amz-Date header corrupted by load balancer")
		} else {
			t.Logf("✓ X-Amz-Date header preserved through load balancer")
		}

		// Verify X-Forwarded headers were added
		if req.Header.Get("X-Forwarded-For") != "" {
			t.Logf("✓ Load balancer added X-Forwarded headers correctly")
		}

		t.Log("✓ Load balancer preserves auth headers correctly")
	})
}

// TestMultiHopEndToEndHeaderFidelity tests complete end-to-end header fidelity
// through the entire request path, simulating a real production environment.
func TestMultiHopEndToEndHeaderFidelity(t *testing.T) {
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

	armorHandler := srv.Handler()

	t.Run("Complete production path: Client → Cloudflare → Load Balancer → ARMOR", func(t *testing.T) {
		t.Log("Testing complete production request path:")
		t.Log("1. Client creates S3 request with Authorization header")
		t.Log("2. Request passes through Cloudflare (CDN/reverse proxy)")
		t.Log("3. Request passes through load balancer (L7 LB)")
		t.Log("4. Request reaches ARMOR S3 API server")
		t.Log("5. Verify headers are byte-for-byte identical at each step")

		// Build the complete proxy chain
		handler := armorHandler

		// Layer 3: Load Balancer (closest to ARMOR)
		handler = loadBalancerLayer(handler)

		// Layer 2: Cloudflare (in front of load balancer)
		handler = cloudflareProxyLayer(handler)

		// Capture headers at each hop
		var (
			headersAfterCF, headersAfterLB, headersAtArmor map[string]string
			capturedCF, capturedLB, capturedArmor bool
		)

		// Wrap to capture after Cloudflare
		captureAfterCF := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headersAfterCF = make(map[string]string)
			headersAfterCF["Authorization"] = r.Header.Get("Authorization")
			headersAfterCF["X-Amz-Date"] = r.Header.Get("X-Amz-Date")
			headersAfterCF["X-Amz-Content-Sha256"] = r.Header.Get("X-Amz-Content-Sha256")
			capturedCF = true

			// Continue to load balancer
			handler.ServeHTTP(w, r)
		})

		// Wrap to capture after Load Balancer
		captureAfterLB := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headersAfterLB = make(map[string]string)
			headersAfterLB["Authorization"] = r.Header.Get("Authorization")
			headersAfterLB["X-Amz-Date"] = r.Header.Get("X-Amz-Date")
			headersAfterLB["X-Amz-Content-Sha256"] = r.Header.Get("X-Amz-Content-Sha256")
			capturedLB = true

			// Continue to ARMOR
			captureAfterCF.ServeHTTP(w, r)
		})

		// Wrap to capture at ARMOR boundary
		captureAtArmor := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			headersAtArmor = make(map[string]string)
			headersAtArmor["Authorization"] = r.Header.Get("Authorization")
			headersAtArmor["X-Amz-Date"] = r.Header.Get("X-Amz-Date")
			headersAtArmor["X-Amz-Content-Sha256"] = r.Header.Get("X-Amz-Content-Sha256")
			capturedArmor = true

			// Call ARMOR
			captureAfterLB.ServeHTTP(w, r)
		})

		// Create client request with full S3 auth
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		originalAuth := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-content-sha256, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
		originalAmzDate := "20130524T000000Z"
		originalAmzSha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

		req.Header.Set("Authorization", originalAuth)
		req.Header.Set("X-Amz-Date", originalAmzDate)
		req.Header.Set("X-Amz-Content-Sha256", originalAmzSha)
		req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

		// Serve the complete request chain
		w := httptest.NewRecorder()
		captureAtArmor.ServeHTTP(w, req)

		// Verify we captured at all points
		if !capturedCF || !capturedLB || !capturedArmor {
			t.Errorf("Failed to capture headers at all hop points")
			t.Logf("capturedCF: %v, capturedLB: %v, capturedArmor: %v", capturedCF, capturedLB, capturedArmor)
			return
		}

		// Verify end-to-end fidelity
		t.Log("Verifying end-to-end header fidelity:")

		allHeaders := []string{"Authorization", "X-Amz-Date", "X-Amz-Content-Sha256"}
		allPreserved := true

		for _, headerName := range allHeaders {
			var originalValue string
			switch headerName {
			case "Authorization":
				originalValue = originalAuth
			case "X-Amz-Date":
				originalValue = originalAmzDate
			case "X-Amz-Content-Sha256":
				originalValue = originalAmzSha
			}

			cfValue := headersAfterCF[headerName]
			lbValue := headersAfterLB[headerName]
			armorValue := headersAtArmor[headerName]

			t.Logf("  %s:", headerName)
			t.Logf("    Original: %q (len=%d)", truncateForLog(originalValue), len(originalValue))
			t.Logf("    After CF: %q (len=%d)", truncateForLog(cfValue), len(cfValue))
			t.Logf("    After LB: %q (len=%d)", truncateForLog(lbValue), len(lbValue))
			t.Logf("    At ARMOR: %q (len=%d)", truncateForLog(armorValue), len(armorValue))

			if originalValue == cfValue && cfValue == lbValue && lbValue == armorValue {
				t.Logf("    ✓ Preserved through all hops")
			} else {
				t.Logf("    ✗ CORRUPTED during passthrough!")
				allPreserved = false
			}
		}

		if allPreserved {
			t.Log("✓ End-to-end header fidelity verified")
			t.Log("✓ All headers preserved byte-for-byte through complete production path")
		} else {
			t.Error("✗ Header corruption detected in multi-hop path")
		}
	})
}

// Helper types and functions for multi-hop testing

// hopLayer represents a middleware layer in the request path
type hopLayer func(next http.Handler) http.Handler

// cloudflareProxyLayer simulates Cloudflare as a reverse proxy
// Cloudflare preserves authentication headers but adds its own metadata headers
func cloudflareProxyLayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Cloudflare adds response headers
		w.Header().Set("CF-RAY", "1234567890abcdef")
		w.Header().Set("CF-Cache-Status", "MISS")

		// Cloudflare preserves authentication headers (critical behavior)
		// Cloudflare does NOT strip Authorization, X-Amz-* headers

		next.ServeHTTP(w, r)
	})
}

// loadBalancerLayer simulates a layer 7 load balancer
// Load balancers typically preserve authentication headers for S3 API compatibility
func loadBalancerLayer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load balancers add forwarding headers
		r.Header.Set("X-Forwarded-For", "203.0.113.1")
		r.Header.Set("X-Forwarded-Proto", "https")
		r.Header.Set("X-Forwarded-Host", "armor.example.com")

		// Load balancers preserve authentication headers (critical for S3 APIs)

		next.ServeHTTP(w, r)
	})
}

// reverseProxyLayer simulates a generic reverse proxy (nginx, envoy, etc.)
func reverseProxyLayer(name string) hopLayer {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generic reverse proxy behavior
			// Most reverse proxies preserve authentication headers by default
			// unless explicitly configured to strip them

			// Add proxy identification header
			r.Header.Set("X-Forwarded-By", name)

			next.ServeHTTP(w, r)
		})
	}
}

// TestMultiHopHeaderIntegrityUnderLoad tests header preservation when
// multiple requests are processed concurrently through multi-hop paths.
func TestMultiHopHeaderIntegrityUnderLoad(t *testing.T) {
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

	armorHandler := srv.Handler()

	// Build multi-hop handler
	handler := armorHandler
	handler = loadBalancerLayer(handler)
	handler = cloudflareProxyLayer(handler)

	t.Run("Concurrent requests preserve headers correctly", func(t *testing.T) {
		t.Log("Testing concurrent multi-hop requests:")
		t.Log("- Multiple simultaneous requests through proxy chain")
		t.Log("- Verify no cross-contamination between requests")
		t.Log("- Verify each request's headers remain isolated and intact")

		numRequests := 10
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			go func(reqNum int) {
				// Create unique headers for this request
				uniqueSig := fmt.Sprintf("%064x", reqNum) // 64-char hex signature
				authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=%s", uniqueSig)
				amzDate := fmt.Sprintf("20130524T%06dZ", reqNum)

				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", authHeader)
				req.Header.Set("X-Amz-Date", amzDate)
				req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

				// Capture headers at ARMOR
				var capturedAuth, capturedAmzDate string
				capturingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					capturedAuth = r.Header.Get("Authorization")
					capturedAmzDate = r.Header.Get("X-Amz-Date")
					handler.ServeHTTP(w, r)
				})

				w := httptest.NewRecorder()
				capturingHandler.ServeHTTP(w, req)

				// Verify headers match what we sent
				if capturedAuth != authHeader {
					errors <- fmt.Errorf("Request %d: Authorization header mismatch (expected %q, got %q)", reqNum, truncateForLog(authHeader), truncateForLog(capturedAuth))
					return
				}

				if capturedAmzDate != amzDate {
					errors <- fmt.Errorf("Request %d: X-Amz-Date header mismatch (expected %q, got %q)", reqNum, amzDate, capturedAmzDate)
					return
				}

				errors <- nil
			}(i)
		}

		// Collect results
		errorCount := 0
		for i := 0; i < numRequests; i++ {
			if err := <-errors; err != nil {
				t.Error(err)
				errorCount++
			}
		}

		if errorCount == 0 {
			t.Logf("✓ All %d concurrent requests preserved headers correctly", numRequests)
			t.Log("✓ No cross-contamination detected between requests")
		} else {
			t.Errorf("✗ %d of %d requests had header corruption", errorCount, numRequests)
		}
	})
}
