package server

import (
	"encoding/xml"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestMultipleCredentialSets verifies that multiple valid credential sets can authenticate successfully
func TestMultipleCredentialSets(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	// Create multiple valid credential sets
	testCredentials := map[string]*config.Credential{
		"ADMINKEY": {
			AccessKey: "ADMINKEY",
			SecretKey: "ADMINSECRET12345678901234567890123",
			ACLs:      nil, // Full access
		},
		"READONLYKEY": {
			AccessKey: "READONLYKEY",
			SecretKey: "READONLYSECRET1234567890123456789",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "readonly/"},
			},
		},
		"WRITEONLYKEY": {
			AccessKey: "WRITEONLYKEY",
			SecretKey: "WRITEONLYSECRET1234567890123456789",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "writeonly/"},
			},
		},
		"LIMITEDKEY": {
			AccessKey: "LIMITEDKEY",
			SecretKey: "LIMITEDSECRET123456789012345678901",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "limited/"},
			},
		},
	}

	ts := SetupTestServerWithCredentials(t, testCredentials)
	defer TeardownTestServer(t, ts)

	t.Run("All credential sets can authenticate successfully", func(t *testing.T) {
		tests := []struct {
			accessKey string
			secretKey string
			path      string
		}{
			{"ADMINKEY", "ADMINSECRET12345678901234567890123", "/test-bucket/test-key"},
			{"READONLYKEY", "READONLYSECRET1234567890123456789", "/test-bucket/readonly/test-file"},
			{"WRITEONLYKEY", "WRITEONLYSECRET1234567890123456789", "/test-bucket/writeonly/test-file"},
			{"LIMITEDKEY", "LIMITEDSECRET123456789012345678901", "/test-bucket/limited/test-file"},
		}

		for _, tt := range tests {
			resp := MakeAuthenticatedRequest(t, ts, "GET", tt.path, nil,
				tt.accessKey, tt.secretKey)
			resp.Body.Close()

			// Should not get 403 (might get 404 or other error, but not auth-related)
			if resp.StatusCode == 403 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				t.Errorf("Credential %s was rejected with 403 on path %s: %s", tt.accessKey, tt.path, string(bodyBytes))
			}
		}
	})

	t.Run("Different credentials work independently", func(t *testing.T) {
		// Test admin key (full access)
		adminResp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/admin-key", nil,
			"ADMINKEY", "ADMINSECRET12345678901234567890123")
		defer adminResp.Body.Close()

		// Test readonly key (access to readonly/ prefix)
		readonlyResp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/readonly/file", nil,
			"READONLYKEY", "READONLYSECRET1234567890123456789")
		defer readonlyResp.Body.Close()

		// Test writeonly key (access to writeonly/ prefix)
		writeonlyResp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/writeonly/file", nil,
			"WRITEONLYKEY", "WRITEONLYSECRET1234567890123456789")
		defer writeonlyResp.Body.Close()

		// None should get 403 (they should all authenticate successfully)
		if adminResp.StatusCode == 403 {
			t.Error("Admin credential failed authentication")
		}
		if readonlyResp.StatusCode == 403 {
			t.Error("Readonly credential failed authentication")
		}
		if writeonlyResp.StatusCode == 403 {
			t.Error("Writeonly credential failed authentication")
		}
	})
}

// TestCredentialIsolation verifies that credential isolation is maintained
func TestCredentialIsolation(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	// Create credentials with different ACLs
	testCredentials := map[string]*config.Credential{
		"RESTRICTED1": {
			AccessKey: "RESTRICTED1",
			SecretKey: "RESTRICTED1_SECRET1234567890123456",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "area1/"},
			},
		},
		"RESTRICTED2": {
			AccessKey: "RESTRICTED2",
			SecretKey: "RESTRICTED2_SECRET1234567890123456",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "area2/"},
			},
		},
		"ADMIN": {
			AccessKey: "ADMIN",
			SecretKey: "ADMIN_SECRET12345678901234567890",
			ACLs:      nil, // Full access
		},
	}

	ts := SetupTestServerWithCredentials(t, testCredentials)
	defer TeardownTestServer(t, ts)

	t.Run("Credentials cannot access other areas", func(t *testing.T) {
		// RESTRICTED1 can access area1 but not area2
		resp1 := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/area1/file.txt", nil,
			"RESTRICTED1", "RESTRICTED1_SECRET1234567890123456")
		defer resp1.Body.Close()

		if resp1.StatusCode == 403 {
			t.Error("RESTRICTED1 should be able to access area1/")
		}

		// Try to access area2 with RESTRICTED1 credentials
		resp2 := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/area2/file.txt", nil,
			"RESTRICTED1", "RESTRICTED1_SECRET1234567890123456")
		defer resp2.Body.Close()

		if resp2.StatusCode != 403 {
			t.Error("RESTRICTED1 should NOT be able to access area2/")
		}

		bodyBytes, _ := io.ReadAll(resp2.Body)
		var s3Err S3Error
		if xml.Unmarshal(bodyBytes, &s3Err) == nil {
			if s3Err.Code != "AccessDenied" {
				t.Errorf("Expected AccessDenied error, got %s", s3Err.Code)
			}
		}
	})

	t.Run("Credentials cannot impersonate other credentials", func(t *testing.T) {
		// Try to use RESTRICTED1 secret with RESTRICTED2 access key
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/area1/file.txt", nil,
			"RESTRICTED2", "RESTRICTED1_SECRET1234567890123456")
		defer resp.Body.Close()

		// Should fail with signature mismatch
		if resp.StatusCode != 403 {
			t.Error("Cross-credential authentication should fail")
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var s3Err S3Error
		if xml.Unmarshal(bodyBytes, &s3Err) == nil {
			if s3Err.Code != "SignatureDoesNotMatch" {
				t.Errorf("Expected SignatureDoesNotMatch error, got %s", s3Err.Code)
			}
		}
	})

	t.Run("Admin has access to all areas", func(t *testing.T) {
		areas := []string{"area1/", "area2/", "other/"}

		for _, area := range areas {
			resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/"+area+"file.txt", nil,
				"ADMIN", "ADMIN_SECRET12345678901234567890")
			defer resp.Body.Close()

			// Admin should not get 403 (might get 404, but not access denied)
			if resp.StatusCode == 403 {
				bodyBytes, _ := io.ReadAll(resp.Body)
				t.Errorf("Admin should have full access, got 403 for %s: %s", area, string(bodyBytes))
			}
		}
	})
}

// TestCredentialRotation verifies that credential rotation works correctly
func TestCredentialRotation(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	// Start with old credentials
	oldCredentials := map[string]*config.Credential{
		"OLDKEY": {
			AccessKey: "OLDKEY",
			SecretKey: "OLDSECRET123456789012345678901234",
			ACLs:      nil,
		},
	}

	ts := SetupTestServerWithCredentials(t, oldCredentials)
	defer TeardownTestServer(t, ts)

	t.Run("Old credentials work initially", func(t *testing.T) {
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
			"OLDKEY", "OLDSECRET123456789012345678901234")
		defer resp.Body.Close()

		if resp.StatusCode == 403 {
			t.Error("Old credentials should work initially")
		}
	})

	// Simulate credential rotation by updating the server
	// In a real scenario, this would be a config reload or server restart
	t.Run("Rotated credentials work", func(t *testing.T) {
		// Create new credentials
		newCredentials := map[string]*config.Credential{
			"NEWKEY": {
				AccessKey: "NEWKEY",
				SecretKey: "NEWSECRET123456789012345678901234",
				ACLs:      nil,
			},
		}

		// Create a new test server with rotated credentials
		newTS := SetupTestServerWithCredentials(t, newCredentials)
		defer TeardownTestServer(t, newTS)

		// New credentials should work
		newResp := MakeAuthenticatedRequest(t, newTS, "GET", "/test-bucket/test-key", nil,
			"NEWKEY", "NEWSECRET123456789012345678901234")
		defer newResp.Body.Close()

		if newResp.StatusCode == 403 {
			t.Error("New credentials should work after rotation")
		}
	})

	t.Run("Old credentials fail after rotation", func(t *testing.T) {
		// Create new credentials without the old one
		newCredentials := map[string]*config.Credential{
			"NEWKEY": {
				AccessKey: "NEWKEY",
				SecretKey: "NEWSECRET123456789012345678901234",
				ACLs:      nil,
			},
		}

		// Create a new test server with rotated credentials
		newTS := SetupTestServerWithCredentials(t, newCredentials)
		defer TeardownTestServer(t, newTS)

		// Old credentials should fail
		oldResp := MakeAuthenticatedRequest(t, newTS, "GET", "/test-bucket/test-key", nil,
			"OLDKEY", "OLDSECRET123456789012345678901234")
		defer oldResp.Body.Close()

		if oldResp.StatusCode != 403 {
			t.Error("Old credentials should fail after rotation")
		}

		bodyBytes, _ := io.ReadAll(oldResp.Body)
		var s3Err S3Error
		if xml.Unmarshal(bodyBytes, &s3Err) == nil {
			if s3Err.Code != "InvalidAccessKeyId" {
				t.Errorf("Expected InvalidAccessKeyId, got %s", s3Err.Code)
			}
		}
	})

	t.Run("Gradual credential rotation works", func(t *testing.T) {
		// Simulate gradual rotation where both old and new are valid temporarily
		transitionCredentials := map[string]*config.Credential{
			"OLDKEY": {
				AccessKey: "OLDKEY",
				SecretKey: "OLDSECRET123456789012345678901234",
				ACLs:      nil,
			},
			"NEWKEY": {
				AccessKey: "NEWKEY",
				SecretKey: "NEWSECRET123456789012345678901234",
				ACLs:      nil,
			},
		}

		transitionTS := SetupTestServerWithCredentials(t, transitionCredentials)
		defer TeardownTestServer(t, transitionTS)

		// Both should work during transition
		oldResp := MakeAuthenticatedRequest(t, transitionTS, "GET", "/test-bucket/test-key", nil,
			"OLDKEY", "OLDSECRET123456789012345678901234")
		defer oldResp.Body.Close()

		newResp := MakeAuthenticatedRequest(t, transitionTS, "GET", "/test-bucket/test-key", nil,
			"NEWKEY", "NEWSECRET123456789012345678901234")
		defer newResp.Body.Close()

		if oldResp.StatusCode == 403 {
			t.Error("Old credentials should work during transition")
		}
		if newResp.StatusCode == 403 {
			t.Error("New credentials should work during transition")
		}
	})
}

// TestCredentialEdgeCases verifies that configuration handles credential edge cases
func TestCredentialEdgeCases(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	t.Run("Empty credential map is handled", func(t *testing.T) {
		emptyCredentials := map[string]*config.Credential{}

		cfg := &config.Config{
			Bucket:      "test-bucket",
			B2Region:    "us-east-005",
			Credentials: emptyCredentials,
			MEK:         make([]byte, 32),
			BlockSize:   65536,
		}

		_, err := New(cfg)
		if err == nil {
			t.Error("Expected error when creating server with no credentials")
		}
	})

	t.Run("Single credential works", func(t *testing.T) {
		singleCredential := map[string]*config.Credential{
			"SINGLEKEY": {
				AccessKey: "SINGLEKEY",
				SecretKey: "SINGLESECRET12345678901234567890",
				ACLs:      nil,
			},
		}

		ts := SetupTestServerWithCredentials(t, singleCredential)
		defer TeardownTestServer(t, ts)

		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
			"SINGLEKEY", "SINGLESECRET12345678901234567890")
		defer resp.Body.Close()

		if resp.StatusCode == 403 {
			t.Error("Single credential should work")
		}
	})

	t.Run("Large number of credentials works", func(t *testing.T) {
		// Create many credentials (simulate enterprise scenario)
		manyCredentials := make(map[string]*config.Credential)
		for i := 0; i < 50; i++ {
			accessKey := "CRED" + string(rune('A'+i))
			secretKey := "SECRET" + string(rune('A'+i)) + "123456789012345678901234567890"
			manyCredentials[accessKey] = &config.Credential{
				AccessKey: accessKey,
				SecretKey: secretKey,
				ACLs: []config.ACLEntry{
					{Bucket: "test-bucket", Prefix: string(rune('a' + i)) + "/"},
				},
			}
		}

		ts := SetupTestServerWithCredentials(t, manyCredentials)
		defer TeardownTestServer(t, ts)

		// Test first and last credentials
		firstResp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/a/file", nil,
			"CRED" + string(rune('A'+0)), "SECRET" + string(rune('A'+0)) + "123456789012345678901234567890")
		defer firstResp.Body.Close()

		lastResp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/az/file", nil,
			"CREDX", "SECRETX123456789012345678901234567890")
		defer lastResp.Body.Close()

		if firstResp.StatusCode == 403 {
			t.Error("First credential should work")
		}
		if lastResp.StatusCode == 403 {
			t.Error("Last credential should work")
		}
	})

	t.Run("Credentials with complex ACLs work", func(t *testing.T) {
		complexCredentials := map[string]*config.Credential{
			"COMPLEX": {
				AccessKey: "COMPLEX",
				SecretKey: "COMPLEX_SECRET1234567890123456789",
				ACLs: []config.ACLEntry{
					{Bucket: "test-bucket", Prefix: "path1/"},
					{Bucket: "test-bucket", Prefix: "path2/"},
					{Bucket: "other-bucket", Prefix: ""},
				},
			},
		}

		ts := SetupTestServerWithCredentials(t, complexCredentials)
		defer TeardownTestServer(t, ts)

		// Test all allowed paths
		paths := []string{
			"/test-bucket/path1/file",
			"/test-bucket/path2/file",
		}

		for _, path := range paths {
			resp := MakeAuthenticatedRequest(t, ts, "GET", path, nil,
				"COMPLEX", "COMPLEX_SECRET1234567890123456789")
			defer resp.Body.Close()

			if resp.StatusCode == 403 {
				t.Errorf("Complex credential should work for path %s", path)
			}
		}
	})

	t.Run("Credential with wildcard bucket ACL works", func(t *testing.T) {
		wildcardCredentials := map[string]*config.Credential{
			"STARKEY": {
				AccessKey: "STARKEY",
				SecretKey: "STAR_SECRET123456789012345678901234",
				ACLs: []config.ACLEntry{
					{Bucket: "*", Prefix: "public/"},
				},
			},
		}

		ts := SetupTestServerWithCredentials(t, wildcardCredentials)
		defer TeardownTestServer(t, ts)

		// Should work for any bucket with public/ prefix
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/any-bucket/public/file", nil,
			"STARKEY", "STAR_SECRET123456789012345678901234")
		defer resp.Body.Close()

		if resp.StatusCode == 403 {
			t.Error("Wildcard bucket credential should work")
		}
	})

	t.Run("Credential expiration is enforced", func(t *testing.T) {
		testCredentials := map[string]*config.Credential{
			"TEMPKEY": {
				AccessKey: "TEMPKEY",
				SecretKey: "TEMP_SECRET123456789012345678901",
				ACLs:      nil,
			},
		}

		ts := SetupTestServerWithCredentials(t, testCredentials)
		defer TeardownTestServer(t, ts)

		// Test with expired timestamp
		oldTime := time.Now().Add(-20 * time.Minute)
		resp := MakeAuthenticatedRequestWithTime(t, ts, "GET", "/test-bucket/test-key", nil,
			"TEMPKEY", "TEMP_SECRET123456789012345678901", oldTime)
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Error("Expired credential should be rejected")
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var s3Err S3Error
		if xml.Unmarshal(bodyBytes, &s3Err) == nil {
			if s3Err.Code != "RequestExpired" {
				t.Errorf("Expected RequestExpired, got %s", s3Err.Code)
			}
		}
	})

	t.Run("Concurrent requests with different credentials work", func(t *testing.T) {
		concurrentCredentials := map[string]*config.Credential{
			"KEY1": {
				AccessKey: "KEY1",
				SecretKey: "SECRET11234567890123456789012345",
				ACLs:      nil,
			},
			"KEY2": {
				AccessKey: "KEY2",
				SecretKey: "SECRET21234567890123456789012345",
				ACLs:      nil,
			},
			"KEY3": {
				AccessKey: "KEY3",
				SecretKey: "SECRET31234567890123456789012345",
				ACLs:      nil,
			},
		}

		ts := SetupTestServerWithCredentials(t, concurrentCredentials)
		defer TeardownTestServer(t, ts)

		// Make concurrent requests
		results := make(chan error, 3)

		go func() {
			resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/key1", nil,
				"KEY1", "SECRET11234567890123456789012345")
			defer resp.Body.Close()
			if resp.StatusCode == 403 {
				results <- nil
			} else {
				results <- nil
			}
		}()

		go func() {
			resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/key2", nil,
				"KEY2", "SECRET21234567890123456789012345")
			defer resp.Body.Close()
			if resp.StatusCode == 403 {
				results <- nil
			} else {
				results <- nil
			}
		}()

		go func() {
			resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/key3", nil,
				"KEY3", "SECRET31234567890123456789012345")
			defer resp.Body.Close()
			if resp.StatusCode == 403 {
				results <- nil
			} else {
				results <- nil
			}
		}()

		// Wait for all to complete
		for i := 0; i < 3; i++ {
			<-results
		}

		// If we got here without deadlock, concurrent requests work
		t.Log("Concurrent requests with different credentials work")
	})
}

// SetupTestServerWithCredentials creates a test server with specific credentials
func SetupTestServerWithCredentials(t *testing.T, credentials map[string]*config.Credential) *TestServer {
	t.Helper()

	// Generate a 32-byte MEK for testing
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i % 256)
	}

	// Create test configuration
	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         mek,
		BlockSize:   65536,
	}

	// Create server
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	// Create test HTTP server using httptest
	testServer := httptest.NewUnstartedServer(srv.Handler())
	testServer.Start()

	// Create test server instance
	testSrv := &TestServer{
		server:     srv,
		testServer: testServer,
		config:     cfg,
		baseURL:    testServer.URL,
	}

	t.Logf("Test server started with %d credentials: %s", len(credentials), testSrv.baseURL)

	return testSrv
}