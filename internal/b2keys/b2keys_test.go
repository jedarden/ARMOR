package b2keys

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKeyInfoJSON(t *testing.T) {
	key := KeyInfo{
		ID:           "key123",
		Name:         "test-key",
		Capabilities: []string{"readFiles", "writeFiles"},
		Expires:      "2026-04-24T00:00:00Z",
		Secret:       "secret123",
	}

	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("Failed to marshal KeyInfo: %v", err)
	}

	var parsed KeyInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal KeyInfo: %v", err)
	}

	if parsed.ID != key.ID {
		t.Errorf("ID mismatch: got %q, want %q", parsed.ID, key.ID)
	}
	if parsed.Name != key.Name {
		t.Errorf("Name mismatch: got %q, want %q", parsed.Name, key.Name)
	}
	if len(parsed.Capabilities) != len(key.Capabilities) {
		t.Errorf("Capabilities length mismatch: got %d, want %d", len(parsed.Capabilities), len(key.Capabilities))
	}
	if parsed.Secret != key.Secret {
		t.Errorf("Secret mismatch: got %q, want %q", parsed.Secret, key.Secret)
	}
}

func TestKeyInfoWithoutOptionalFields(t *testing.T) {
	key := KeyInfo{
		ID:           "key456",
		Name:         "minimal-key",
		Capabilities: []string{"listFiles"},
	}

	data, err := json.Marshal(key)
	if err != nil {
		t.Fatalf("Failed to marshal KeyInfo: %v", err)
	}

	var parsed KeyInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal KeyInfo: %v", err)
	}

	if parsed.Expires != "" {
		t.Errorf("Expected empty Expires, got %q", parsed.Expires)
	}
	if parsed.Secret != "" {
		t.Errorf("Expected empty Secret, got %q", parsed.Secret)
	}
}

func TestCreateKeyRequestJSON(t *testing.T) {
	req := CreateKeyRequest{
		Name:         "new-key",
		Capabilities: []string{"readFiles", "writeFiles", "deleteFiles"},
		Prefix:       "data/",
		Duration:     "168h",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal CreateKeyRequest: %v", err)
	}

	var parsed CreateKeyRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal CreateKeyRequest: %v", err)
	}

	if parsed.Name != req.Name {
		t.Errorf("Name mismatch: got %q, want %q", parsed.Name, req.Name)
	}
	if parsed.Prefix != req.Prefix {
		t.Errorf("Prefix mismatch: got %q, want %q", parsed.Prefix, req.Prefix)
	}
	if parsed.Duration != req.Duration {
		t.Errorf("Duration mismatch: got %q, want %q", parsed.Duration, req.Duration)
	}
}

func TestCreateKeyRequestMinimal(t *testing.T) {
	// Minimal request with only required fields
	jsonData := `{"name":"minimal","capabilities":["readFiles"]}`

	var req CreateKeyRequest
	if err := json.Unmarshal([]byte(jsonData), &req); err != nil {
		t.Fatalf("Failed to unmarshal minimal CreateKeyRequest: %v", err)
	}

	if req.Name != "minimal" {
		t.Errorf("Name mismatch: got %q, want %q", req.Name, "minimal")
	}
	if len(req.Capabilities) != 1 || req.Capabilities[0] != "readFiles" {
		t.Errorf("Capabilities mismatch: got %v", req.Capabilities)
	}
	if req.Prefix != "" {
		t.Errorf("Expected empty Prefix, got %q", req.Prefix)
	}
	if req.Duration != "" {
		t.Errorf("Expected empty Duration, got %q", req.Duration)
	}
}

func TestListKeysResponseJSON(t *testing.T) {
	resp := ListKeysResponse{
		Keys: []KeyInfo{
			{ID: "key1", Name: "key-one", Capabilities: []string{"readFiles"}},
			{ID: "key2", Name: "key-two", Capabilities: []string{"writeFiles"}},
		},
		NextCursor: "next-cursor-123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal ListKeysResponse: %v", err)
	}

	var parsed ListKeysResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal ListKeysResponse: %v", err)
	}

	if len(parsed.Keys) != 2 {
		t.Errorf("Keys length mismatch: got %d, want 2", len(parsed.Keys))
	}
	if parsed.NextCursor != resp.NextCursor {
		t.Errorf("NextCursor mismatch: got %q, want %q", parsed.NextCursor, resp.NextCursor)
	}
}

func TestListKeysResponseEmpty(t *testing.T) {
	resp := ListKeysResponse{
		Keys: []KeyInfo{},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal empty ListKeysResponse: %v", err)
	}

	var parsed ListKeysResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal empty ListKeysResponse: %v", err)
	}

	if len(parsed.Keys) != 0 {
		t.Errorf("Expected empty Keys slice, got %d", len(parsed.Keys))
	}
}

func TestKeyNotFoundError(t *testing.T) {
	err := ErrKeyNotFound
	if err == nil {
		t.Fatal("ErrKeyNotFound should not be nil")
	}
	if err.Error() != "key not found" {
		t.Errorf("Error message mismatch: got %q, want %q", err.Error(), "key not found")
	}
}

func TestDurationParsing(t *testing.T) {
	tests := []struct {
		duration string
		valid    bool
	}{
		{"1h", true},
		{"24h", true},
		{"168h", true}, // 7 days
		{"720h", true}, // 30 days
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.duration, func(t *testing.T) {
			if tt.duration == "" {
				// Empty duration is valid (no expiration)
				return
			}
			_, err := time.ParseDuration(tt.duration)
			if tt.valid && err != nil {
				t.Errorf("Expected valid duration %q, got error: %v", tt.duration, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("Expected invalid duration %q to fail, but it succeeded", tt.duration)
			}
		})
	}
}
