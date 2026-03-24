package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/b2keys"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
)

func TestB2KeysHandlerMethodRouting(t *testing.T) {
	tests := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		// GET /admin/b2/keys - list keys
		{"GET", "/admin/b2/keys", http.StatusServiceUnavailable}, // Service unavailable without client
		// POST /admin/b2/keys - create key
		{"POST", "/admin/b2/keys", http.StatusServiceUnavailable},
		// DELETE /admin/b2/keys/{id} - delete key
		{"DELETE", "/admin/b2/keys/key123", http.StatusServiceUnavailable},
		// PUT not allowed (but still returns 503 since b2keys is nil)
		{"PUT", "/admin/b2/keys", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			// Create a minimal server with nil b2keys client
			s := &Server{
				b2keys:  nil, // No B2 keys client configured
				logger:  logging.New("test"),
				metrics: metrics.DefaultMetrics,
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			if tt.path == "/admin/b2/keys" {
				s.handleB2Keys(rec, req)
			} else {
				s.handleB2KeyDelete(rec, req)
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestB2KeyDeletePathExtraction(t *testing.T) {
	tests := []struct {
		path          string
		expectedKeyID string
		expectError   bool
	}{
		{
			path:          "/admin/b2/keys/key123",
			expectedKeyID: "key123",
		},
		{
			path:          "/admin/b2/keys/abc-xyz-123",
			expectedKeyID: "abc-xyz-123",
		},
		{
			path:        "/admin/b2/keys/",
			expectError: true, // Empty key ID
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			keyID := strings.TrimPrefix(tt.path, "/admin/b2/keys/")
			if tt.expectError {
				if keyID == "" {
					// This is expected
					return
				}
			}
			if keyID != tt.expectedKeyID {
				t.Errorf("Expected key ID %q, got %q", tt.expectedKeyID, keyID)
			}
		})
	}
}

func TestCreateKeyRequestParsing(t *testing.T) {
	jsonData := `{"name":"test-key","capabilities":["readFiles","writeFiles"],"prefix":"data/","duration":"24h"}`

	var req b2keys.CreateKeyRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("Failed to parse request: %v", err)
	}

	if req.Name != "test-key" {
		t.Errorf("Expected name 'test-key', got %q", req.Name)
	}
	if len(req.Capabilities) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(req.Capabilities))
	}
	if req.Prefix != "data/" {
		t.Errorf("Expected prefix 'data/', got %q", req.Prefix)
	}
	if req.Duration != "24h" {
		t.Errorf("Expected duration '24h', got %q", req.Duration)
	}
}

func TestListKeysResponseFormatting(t *testing.T) {
	resp := b2keys.ListKeysResponse{
		Keys: []b2keys.KeyInfo{
			{ID: "key1", Name: "read-only", Capabilities: []string{"readFiles"}},
			{ID: "key2", Name: "read-write", Capabilities: []string{"readFiles", "writeFiles"}},
		},
		NextCursor: "cursor-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	keys, ok := parsed["keys"].([]interface{})
	if !ok {
		t.Fatal("Expected 'keys' to be an array")
	}
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	cursor, ok := parsed["nextCursor"].(string)
	if !ok {
		t.Fatal("Expected 'nextCursor' to be a string")
	}
	if cursor != "cursor-123" {
		t.Errorf("Expected cursor 'cursor-123', got %q", cursor)
	}
}
