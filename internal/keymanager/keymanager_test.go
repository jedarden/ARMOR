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

	km, err := New(defaultMEK, namedKeys, routes, nil)
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
	_, err := New([]byte{1, 2, 3}, nil, nil, nil)
	if err == nil {
		t.Error("Expected error for invalid MEK length")
	}
}

func TestNew_InvalidNamedMEK(t *testing.T) {
	defaultMEK := make([]byte, 32)
	namedKeys := map[string][]byte{
		"test": []byte{1, 2, 3}, // Invalid length
	}

	_, err := New(defaultMEK, namedKeys, nil, nil)
	if err == nil {
		t.Error("Expected error for invalid named MEK length")
	}
}

func TestNew_RouteReferencesUnknownKey(t *testing.T) {
	defaultMEK := make([]byte, 32)
	routes := []Route{
		{Prefix: "data/", KeyName: "nonexistent"},
	}

	_, err := New(defaultMEK, nil, routes, nil)
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

	km, _ := New(defaultMEK, namedKeys, routes, nil)

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

	km, _ := New(defaultMEK, namedKeys, routes, nil)

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

	km, _ := New(defaultMEK, namedKeys, nil, nil)

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

	km, _ := New(defaultMEK, namedKeys, routes, nil)

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

	km, _ := New(defaultMEK, namedKeys, nil, nil)

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
				{Prefix: "data/pii/", KeyName: "sensitive"},
				{Prefix: "data/", KeyName: "default"},
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
		{
			input:    "data/*/file=sensitive",
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

func TestParseRoutesSelectExpectedKeyForDocumentedPatterns(t *testing.T) {
	defaultMEK := bytesOf(1)
	sensitiveMEK := bytesOf(2)
	archiveMEK := bytesOf(3)
	routes, err := ParseKeyRoutes("data/pii/*=SENSITIVE,archive/*=archive,*=default")
	if err != nil {
		t.Fatalf("ParseKeyRoutes failed: %v", err)
	}
	km, err := New(defaultMEK, map[string][]byte{
		"sensitive": sensitiveMEK,
		"archive":   archiveMEK,
	}, routes, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	tests := map[string]string{
		"data/pii/customer.json":   "sensitive",
		"data/public/report.json":  "default",
		"archive/2026/report.json": "archive",
		"other/report.json":        "default",
	}
	for objectKey, want := range tests {
		key, err := km.GetKey(objectKey)
		if err != nil {
			t.Fatalf("GetKey(%q): %v", objectKey, err)
		}
		if key.Name != want {
			t.Errorf("GetKey(%q) = %q, want %q", objectKey, key.Name, want)
		}
	}
}

func TestUpdateKeyRotatesOnlySelectedKey(t *testing.T) {
	defaultMEK := bytesOf(1)
	sensitiveMEK := bytesOf(2)
	newSensitiveMEK := bytesOf(9)
	archiveMEK := bytesOf(3)
	km, err := New(defaultMEK, map[string][]byte{
		"sensitive": sensitiveMEK,
		"archive":   archiveMEK,
	}, []Route{
		{Prefix: "sensitive/", KeyName: "sensitive"},
		{Prefix: "archive/", KeyName: "archive"},
	}, nil)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	if err := km.UpdateKey("SENSITIVE", newSensitiveMEK); err != nil {
		t.Fatalf("UpdateKey: %v", err)
	}
	got, _, err := km.GetMEK("sensitive/file")
	if err != nil || !equalBytes(got, newSensitiveMEK) {
		t.Fatalf("sensitive key after update = %x, err %v; want new key", got, err)
	}
	got, _, err = km.GetMEK("archive/file")
	if err != nil || !equalBytes(got, archiveMEK) {
		t.Fatalf("archive key changed during sensitive rotation: %x, err %v", got, err)
	}
	got, _, err = km.GetMEK("public/file")
	if err != nil || !equalBytes(got, defaultMEK) {
		t.Fatalf("default key changed during sensitive rotation: %x, err %v", got, err)
	}

	if err := km.UpdateKey("missing", bytesOf(8)); err == nil {
		t.Error("UpdateKey should reject an unknown key")
	}
}

func bytesOf(value byte) []byte {
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = value
	}
	return mek
}

func TestParseNamedKeys(t *testing.T) {
	defaultMEKHex := hex.EncodeToString(make([]byte, 32))
	sensitiveMEK := make([]byte, 32)
	for i := range sensitiveMEK {
		sensitiveMEK[i] = byte(i + 100)
	}
	sensitiveMEKHex := hex.EncodeToString(sensitiveMEK)

	envVars := map[string]string{
		"ARMOR_MEK":           defaultMEKHex,
		"ARMOR_MEK_SENSITIVE": sensitiveMEKHex,
		"ARMOR_MEK_ARCHIVE":   sensitiveMEKHex,
		"OTHER_VAR":           "ignored",
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

	km, _ := New(defaultMEK, namedKeys, nil, nil)

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

	km, _ := New(defaultMEK, nil, routes, nil)

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

func TestKeyRingFunctionality(t *testing.T) {
	// Create test MEKs
	defaultMEK := make([]byte, 32)
	for i := range defaultMEK {
		defaultMEK[i] = byte(i)
	}

	retired1 := make([]byte, 32)
	for i := range retired1 {
		retired1[i] = byte(i + 100)
	}

	retired2 := make([]byte, 32)
	for i := range retired2 {
		retired2[i] = byte(i + 200)
	}

	// Create ring data (concatenated MEKs)
	ringData := append([]byte{}, retired1...)
	ringData = append(ringData, retired2...)

	ringKeys := map[string][]byte{
		"default": ringData,
	}

	km, err := New(defaultMEK, nil, nil, ringKeys)
	if err != nil {
		t.Fatalf("New() with ring failed: %v", err)
	}

	// Test Ring() accessor
	ring := km.Ring("default")
	if ring == nil {
		t.Fatal("Ring() returned nil for default key")
	}

	if len(ring) != 2 {
		t.Errorf("Ring() returned %d entries, want 2", len(ring))
	}

	// Verify fingerprints are set
	for i, entry := range ring {
		if entry.Fingerprint == "" {
			t.Errorf("Ring entry %d has empty fingerprint", i)
		}
		if len(entry.Fingerprint) != 16 {
			t.Errorf("Ring entry %d fingerprint length = %d, want 16", i, len(entry.Fingerprint))
		}
	}

	// Test Ring() for non-existent key
	ring = km.Ring("nonexistent")
	if ring != nil {
		t.Error("Ring() should return nil for non-existent key")
	}

	// Test Ring() with empty key name (should use default)
	ring = km.Ring("")
	if ring == nil {
		t.Error("Ring() with empty name should return default ring")
	}

	if len(ring) != 2 {
		t.Errorf("Ring() with empty name returned %d entries, want 2", len(ring))
	}
}

func TestGetMEKByFingerprint(t *testing.T) {
	// Create test MEKs
	defaultMEK, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	retired1, _ := hex.DecodeString("1111111111111111111111111111111111111111111111111111111111111111")
	retired2, _ := hex.DecodeString("2222222222222222222222222222222222222222222222222222222222222222")

	// Create ring data
	ringData := append([]byte{}, retired1...)
	ringData = append(ringData, retired2...)

	ringKeys := map[string][]byte{
		"default": ringData,
	}

	km, err := New(defaultMEK, nil, nil, ringKeys)
	if err != nil {
		t.Fatalf("New() with ring failed: %v", err)
	}

	// Test getting active key by fingerprint
	defaultFP := "588161913cc0c9f5" // Known fingerprint for the default MEK
	mek, found := km.GetMEKByFingerprint("default", defaultFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find active key")
	}
	if !equalBytes(mek, defaultMEK) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for active key")
	}

	// Test getting retired key by fingerprint
	retired1FP := "f65f837a1b287304" // Known fingerprint for retired1
	mek, found = km.GetMEKByFingerprint("default", retired1FP)
	if !found {
		t.Error("GetMEKByFingerprint() should find retired key")
	}
	if !equalBytes(mek, retired1) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for retired key")
	}

	// Test non-existent fingerprint
	_, found = km.GetMEKByFingerprint("default", "ffffffffffffffff")
	if found {
		t.Error("GetMEKByFingerprint() should not find non-existent fingerprint")
	}

	// Test with empty key name (should use default)
	_, found = km.GetMEKByFingerprint("", defaultFP)
	if !found {
		t.Error("GetMEKByFingerprint() with empty name should use default key")
	}

	// Test with named key
	namedMEK, _ := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	namedRetired, _ := hex.DecodeString("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	namedRingData := append([]byte{}, namedRetired...)
	namedRingKeys := map[string][]byte{
		"default": ringData,
		"test":    namedRingData,
	}

	namedKeys := map[string][]byte{
		"test": namedMEK,
	}

	km2, err := New(defaultMEK, namedKeys, nil, namedRingKeys)
	if err != nil {
		t.Fatalf("New() with named ring failed: %v", err)
	}

	// Test getting named active key
	namedFP := "76e65f64b00963fb" // Known fingerprint for namedMEK
	mek, found = km2.GetMEKByFingerprint("test", namedFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find named active key")
	}
	if !equalBytes(mek, namedMEK) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for named active key")
	}

	// Test getting named retired key
	namedRetiredFP := "e3c08bbc03627091" // Known fingerprint for namedRetired
	mek, found = km2.GetMEKByFingerprint("test", namedRetiredFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find named retired key")
	}
	if !equalBytes(mek, namedRetired) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for named retired key")
	}

	// Test non-existent key
	_, found = km2.GetMEKByFingerprint("nonexistent", defaultFP)
	if found {
		t.Error("GetMEKByFingerprint() should not find non-existent key")
	}
}

func TestEmptyRing(t *testing.T) {
	defaultMEK := make([]byte, 32)
	for i := range defaultMEK {
		defaultMEK[i] = byte(i)
	}

	km, err := New(defaultMEK, nil, nil, nil)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Ring() should return nil for empty ring
	ring := km.Ring("default")
	if ring != nil {
		t.Error("Ring() should return nil when no ring is configured")
	}

	// GetMEKByFingerprint should only find active key
	activeFP := "588161913cc0c9f5"
	mek, found := km.GetMEKByFingerprint("default", activeFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find active key even without ring")
	}
	if !equalBytes(mek, defaultMEK) {
		t.Error("GetMEKByFingerprint() returned wrong MEK")
	}
}

func TestRingWithNamedKeys(t *testing.T) {
	defaultMEK, _ := hex.DecodeString("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	namedMEK, _ := hex.DecodeString("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	namedRetired, _ := hex.DecodeString("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")

	namedKeys := map[string][]byte{
		"test": namedMEK,
	}

	namedRingData := append([]byte{}, namedRetired...)
	ringKeys := map[string][]byte{
		"test": namedRingData,
	}

	km, err := New(defaultMEK, namedKeys, nil, ringKeys)
	if err != nil {
		t.Fatalf("New() with named ring failed: %v", err)
	}

	// Verify named key ring
	ring := km.Ring("test")
	if ring == nil {
		t.Fatal("Ring() should find named key ring")
	}

	if len(ring) != 1 {
		t.Errorf("Named ring has %d entries, want 1", len(ring))
	}

	// Verify default key has no ring
	defaultRing := km.Ring("default")
	if defaultRing != nil {
		t.Error("Default ring should be nil when not configured")
	}

	// Test GetMEKByFingerprint with named key
	namedFP := "76e65f64b00963fb"
	mek, found := km.GetMEKByFingerprint("test", namedFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find named active key")
	}
	if !equalBytes(mek, namedMEK) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for named active key")
	}

	// Test finding in named ring
	namedRetiredFP := "e3c08bbc03627091"
	mek, found = km.GetMEKByFingerprint("test", namedRetiredFP)
	if !found {
		t.Error("GetMEKByFingerprint() should find named retired key")
	}
	if !equalBytes(mek, namedRetired) {
		t.Error("GetMEKByFingerprint() returned wrong MEK for named retired key")
	}
}

func TestRingValidationErrors(t *testing.T) {
	defaultMEK := make([]byte, 32)
	for i := range defaultMEK {
		defaultMEK[i] = byte(i)
	}

	tests := []struct {
		name      string
		ringKeys  map[string][]byte
		expectErr bool
	}{
		{
			name: "invalid ring data - not multiple of 32 bytes",
			ringKeys: map[string][]byte{
				"default": []byte{1, 2, 3}, // Only 3 bytes
			},
			expectErr: true,
		},
		{
			name: "ring for non-existent key",
			ringKeys: map[string][]byte{
				"nonexistent": make([]byte, 32),
			},
			expectErr: true,
		},
		{
			name: "valid empty ring",
			ringKeys: map[string][]byte{
				"default": []byte{},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(defaultMEK, nil, nil, tt.ringKeys)
			if tt.expectErr && err == nil {
				t.Error("Expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
