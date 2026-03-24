package keymanager

import (
	"encoding/hex"
	"testing"
)

func TestNew(t *testing.T) {
	// Create test MEKs
	defaultMEK := make([]byte, 32)
	for i := range defaultMEK {
		defaultMEK[i] = byte(i)
	}

	sensitiveMEK := make([]byte, 32)
	for i := range sensitiveMEK {
		sensitiveMEK[i] = byte(i + 100)
	}

	namedKeys := map[string][]byte{
		"sensitive": sensitiveMEK,
	}

	routes := []Route{
		{Prefix: "data/pii/", KeyName: "sensitive"},
		{Prefix: "data/", KeyName: "default"},
	}

	km, err := New(defaultMEK, namedKeys, routes)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	if km == nil {
		t.Fatal("New() returned nil")
	}

	// Verify keys are stored
	keys := km.ListKeys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}
}

func TestNew_InvalidDefaultMEK(t *testing.T) {
	_, err := New([]byte{1, 2, 3}, nil, nil)
	if err == nil {
		t.Error("Expected error for invalid MEK length")
	}
}

func TestNew_InvalidNamedMEK(t *testing.T) {
	defaultMEK := make([]byte, 32)
	namedKeys := map[string][]byte{
		"test": []byte{1, 2, 3}, // Invalid length
	}

	_, err := New(defaultMEK, namedKeys, nil)
	if err == nil {
		t.Error("Expected error for invalid named MEK length")
	}
}

func TestNew_RouteReferencesUnknownKey(t *testing.T) {
	defaultMEK := make([]byte, 32)
	routes := []Route{
		{Prefix: "data/", KeyName: "nonexistent"},
	}

	_, err := New(defaultMEK, nil, routes)
	if err == nil {
		t.Error("Expected error for route referencing unknown key")
	}
}

func TestGetKey(t *testing.T) {
	defaultMEK := make([]byte, 32)
	sensitiveMEK := make([]byte, 32)
	for i := range sensitiveMEK {
		sensitiveMEK[i] = byte(i + 100)
	}

	namedKeys := map[string][]byte{
		"sensitive": sensitiveMEK,
	}

	routes := []Route{
		{Prefix: "data/pii/", KeyName: "sensitive"},
		{Prefix: "data/", KeyName: "default"},
	}

	km, _ := New(defaultMEK, namedKeys, routes)

	tests := []struct {
		objectKey    string
		expectedName string
	}{
		{"data/pii/users.csv", "sensitive"},
		{"data/pii/subdir/file.txt", "sensitive"},
		{"data/public/logs.txt", "default"},
		{"other/file.txt", "default"},
		{"", "default"},
	}

	for _, tt := range tests {
		key, err := km.GetKey(tt.objectKey)
		if err != nil {
			t.Errorf("GetKey(%q) failed: %v", tt.objectKey, err)
			continue
		}
		if key.Name != tt.expectedName {
			t.Errorf("GetKey(%q) = %q, want %q", tt.objectKey, key.Name, tt.expectedName)
		}
	}
}

func TestGetKey_LongestPrefixMatch(t *testing.T) {
	defaultMEK := make([]byte, 32)
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)

	namedKeys := map[string][]byte{
		"key1": key1,
		"key2": key2,
	}

	// Test that longer prefixes take precedence
	routes := []Route{
		{Prefix: "a/b/c/", KeyName: "key2"},
		{Prefix: "a/b/", KeyName: "key1"},
		{Prefix: "a/", KeyName: "default"},
	}

	km, _ := New(defaultMEK, namedKeys, routes)

	tests := []struct {
		objectKey    string
		expectedName string
	}{
		{"a/b/c/file.txt", "key2"},
		{"a/b/other.txt", "key1"},
		{"a/file.txt", "default"},
		{"b/file.txt", "default"},
	}

	for _, tt := range tests {
		key, err := km.GetKey(tt.objectKey)
		if err != nil {
			t.Errorf("GetKey(%q) failed: %v", tt.objectKey, err)
			continue
		}
		if key.Name != tt.expectedName {
			t.Errorf("GetKey(%q) = %q, want %q", tt.objectKey, key.Name, tt.expectedName)
		}
	}
}

func TestGetKeyByID(t *testing.T) {
	defaultMEK := make([]byte, 32)
	sensitiveMEK := make([]byte, 32)

	namedKeys := map[string][]byte{
		"sensitive": sensitiveMEK,
	}

	km, _ := New(defaultMEK, namedKeys, nil)

	// Test getting default key
	key, err := km.GetKeyByID("")
	if err != nil {
		t.Errorf("GetKeyByID(\"\") failed: %v", err)
	}
	if key.Name != "default" {
		t.Errorf("GetKeyByID(\"\") = %q, want \"default\"", key.Name)
	}

	// Test getting named key
	key, err = km.GetKeyByID("sensitive")
	if err != nil {
		t.Errorf("GetKeyByID(\"sensitive\") failed: %v", err)
	}
	if key.Name != "sensitive" {
		t.Errorf("GetKeyByID(\"sensitive\") = %q, want \"sensitive\"", key.Name)
	}

	// Test getting unknown key
	_, err = km.GetKeyByID("unknown")
	if err == nil {
		t.Error("Expected error for unknown key")
	}
}

func TestGetMEK(t *testing.T) {
	defaultMEK := make([]byte, 32)
	for i := range defaultMEK {
		defaultMEK[i] = byte(i)
	}

	sensitiveMEK := make([]byte, 32)
	for i := range sensitiveMEK {
		sensitiveMEK[i] = byte(i + 100)
	}

	namedKeys := map[string][]byte{
		"sensitive": sensitiveMEK,
	}

	routes := []Route{
		{Prefix: "pii/", KeyName: "sensitive"},
	}

	km, _ := New(defaultMEK, namedKeys, routes)

	// Test default key
	mek, keyName, err := km.GetMEK("public/file.txt")
	if err != nil {
		t.Fatalf("GetMEK failed: %v", err)
	}
	if keyName != "default" {
		t.Errorf("GetMEK keyName = %q, want \"default\"", keyName)
	}
	if !equalBytes(mek, defaultMEK) {
		t.Error("GetMEK returned wrong MEK for default")
	}

	// Test sensitive key
	mek, keyName, err = km.GetMEK("pii/secrets.txt")
	if err != nil {
		t.Fatalf("GetMEK failed: %v", err)
	}
	if keyName != "sensitive" {
		t.Errorf("GetMEK keyName = %q, want \"sensitive\"", keyName)
	}
	if !equalBytes(mek, sensitiveMEK) {
		t.Error("GetMEK returned wrong MEK for sensitive")
	}
}

func TestGetMEKByID(t *testing.T) {
	defaultMEK := make([]byte, 32)
	sensitiveMEK := make([]byte, 32)

	namedKeys := map[string][]byte{
		"sensitive": sensitiveMEK,
	}

	km, _ := New(defaultMEK, namedKeys, nil)

	// Test default key
	mek, err := km.GetMEKByID("")
	if err != nil {
		t.Fatalf("GetMEKByID failed: %v", err)
	}
	if !equalBytes(mek, defaultMEK) {
		t.Error("GetMEKByID returned wrong MEK")
	}

	// Test named key
	mek, err = km.GetMEKByID("sensitive")
	if err != nil {
		t.Fatalf("GetMEKByID failed: %v", err)
	}
	if !equalBytes(mek, sensitiveMEK) {
		t.Error("GetMEKByID returned wrong MEK")
	}
}

func TestParseKeyRoutes(t *testing.T) {
	tests := []struct {
		input    string
		expected []Route
		hasError bool
	}{
		{
			input:    "",
			expected: nil,
		},
		{
			input: "data/pii/*=sensitive,data/*=default",
			expected: []Route{
				{Prefix: "data/pii/*", KeyName: "sensitive"},
				{Prefix: "data/*", KeyName: "default"},
			},
		},
		{
			input: "*=default",
			expected: []Route{
				{Prefix: "", KeyName: "default"},
			},
		},
		{
			input:    "invalid",
			hasError: true,
		},
		{
			input:    "=key",
			hasError: true,
		},
		{
			input:    "prefix=",
			hasError: true,
		},
	}

	for _, tt := range tests {
		routes, err := ParseKeyRoutes(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("ParseKeyRoutes(%q) expected error", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseKeyRoutes(%q) failed: %v", tt.input, err)
			continue
		}
		if len(routes) != len(tt.expected) {
			t.Errorf("ParseKeyRoutes(%q) returned %d routes, want %d", tt.input, len(routes), len(tt.expected))
			continue
		}
		for i, route := range routes {
			if route.Prefix != tt.expected[i].Prefix || route.KeyName != tt.expected[i].KeyName {
				t.Errorf("ParseKeyRoutes(%q)[%d] = %+v, want %+v", tt.input, i, route, tt.expected[i])
			}
		}
	}
}

func TestParseNamedKeys(t *testing.T) {
	defaultMEKHex := hex.EncodeToString(make([]byte, 32))
	sensitiveMEK := make([]byte, 32)
	for i := range sensitiveMEK {
		sensitiveMEK[i] = byte(i + 100)
	}
	sensitiveMEKHex := hex.EncodeToString(sensitiveMEK)

	envVars := map[string]string{
		"ARMOR_MEK":          defaultMEKHex,
		"ARMOR_MEK_SENSITIVE": sensitiveMEKHex,
		"ARMOR_MEK_ARCHIVE":   sensitiveMEKHex,
		"OTHER_VAR":          "ignored",
	}

	namedKeys, err := ParseNamedKeys(envVars)
	if err != nil {
		t.Fatalf("ParseNamedKeys failed: %v", err)
	}

	// Should have 2 named keys (sensitive and archive), not default
	if len(namedKeys) != 2 {
		t.Errorf("Expected 2 named keys, got %d", len(namedKeys))
	}

	// Check key names are lowercase
	if _, ok := namedKeys["sensitive"]; !ok {
		t.Error("Expected 'sensitive' key")
	}
	if _, ok := namedKeys["archive"]; !ok {
		t.Error("Expected 'archive' key")
	}
}

func TestParseNamedKeys_InvalidHex(t *testing.T) {
	envVars := map[string]string{
		"ARMOR_MEK_TEST": "not-valid-hex",
	}

	_, err := ParseNamedKeys(envVars)
	if err == nil {
		t.Error("Expected error for invalid hex")
	}
}

func TestParseNamedKeys_WrongLength(t *testing.T) {
	envVars := map[string]string{
		"ARMOR_MEK_TEST": hex.EncodeToString([]byte{1, 2, 3}),
	}

	_, err := ParseNamedKeys(envVars)
	if err == nil {
		t.Error("Expected error for wrong MEK length")
	}
}

func TestListKeys(t *testing.T) {
	defaultMEK := make([]byte, 32)
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)

	namedKeys := map[string][]byte{
		"zebra": key1,
		"alpha": key2,
	}

	km, _ := New(defaultMEK, namedKeys, nil)

	keys := km.ListKeys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Should be sorted alphabetically
	expected := []string{"alpha", "default", "zebra"}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("ListKeys()[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

func TestListRoutes(t *testing.T) {
	defaultMEK := make([]byte, 32)
	routes := []Route{
		{Prefix: "a/", KeyName: "default"},
		{Prefix: "b/", KeyName: "default"},
	}

	km, _ := New(defaultMEK, nil, routes)

	listRoutes := km.ListRoutes()
	if len(listRoutes) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(listRoutes))
	}
}

// Helper function
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
