// Package keymanager handles multi-key routing for ARMOR.
// It allows different MEKs to be used for different object key prefixes,
// enabling data classification (e.g., different keys for sensitive vs. non-sensitive data).
package keymanager

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/jedarden/armor/internal/crypto"
)

// Key represents a named master encryption key.
type Key struct {
	Name string
	MEK  []byte
}

// RingKeyEntry represents a retired-but-valid MEK with its fingerprint.
type RingKeyEntry struct {
	MEK         []byte // The retired MEK (32 bytes)
	Fingerprint string // 16 hex chars (first 8 bytes of SHA-256(MEK))
}

// Route represents a prefix-to-key mapping.
type Route struct {
	Prefix  string
	KeyName string
}

// KeyManager manages multiple MEKs and routes object keys to the appropriate key.
type KeyManager struct {
	mu             sync.RWMutex
	keys           map[string]*Key           // name -> key
	rings          map[string][]RingKeyEntry // name -> ordered ring of retired keys
	routes         []Route                   // sorted by prefix length (longest first)
	defaultKeyName string
}

// New creates a new KeyManager with the given keys, routes, and retired key rings.
// ringKeys is a map from key name to concatenated ring MEKs (each 32 bytes).
func New(defaultMEK []byte, namedKeys map[string][]byte, routes []Route, ringKeys map[string][]byte) (*KeyManager, error) {
	km := &KeyManager{
		keys:           make(map[string]*Key),
		rings:          make(map[string][]RingKeyEntry),
		routes:         append([]Route(nil), routes...),
		defaultKeyName: "default",
	}

	// Add the default key
	if len(defaultMEK) != 32 {
		return nil, fmt.Errorf("default MEK must be 32 bytes, got %d", len(defaultMEK))
	}
	km.keys["default"] = &Key{Name: "default", MEK: append([]byte(nil), defaultMEK...)}

	// Add named keys
	for name, mek := range namedKeys {
		name = normalizeKeyName(name)
		if name == "" || name == "default" {
			return nil, fmt.Errorf("invalid named key %q", name)
		}
		if len(mek) != 32 {
			return nil, fmt.Errorf("MEK for key %q must be 32 bytes, got %d", name, len(mek))
		}
		if _, exists := km.keys[name]; exists {
			return nil, fmt.Errorf("duplicate key name %q", name)
		}
		km.keys[name] = &Key{Name: name, MEK: append([]byte(nil), mek...)}
	}

	// Initialize retired key rings
	for keyName, ringMEKs := range ringKeys {
		keyName = normalizeKeyName(keyName)
		if keyName == "" {
			return nil, fmt.Errorf("invalid ring key name")
		}

		// Verify the key exists (either as default or named key)
		if _, exists := km.keys[keyName]; !exists {
			return nil, fmt.Errorf("ring specified for unknown key %q", keyName)
		}

		// Parse ring MEKs (concatenated 32-byte MEKs)
		var ring []RingKeyEntry
		for i := 0; i < len(ringMEKs); i += 32 {
			if i+32 > len(ringMEKs) {
				return nil, fmt.Errorf("ring data for key %q is not a multiple of 32 bytes", keyName)
			}
			mek := ringMEKs[i : i+32]
			fp := crypto.MEKFingerprint(mek)
			ring = append(ring, RingKeyEntry{
				MEK:         append([]byte(nil), mek...),
				Fingerprint: fp,
			})
		}
		km.rings[keyName] = ring
	}

	// Validate routes reference existing keys
	for i := range km.routes {
		km.routes[i].KeyName = normalizeKeyName(km.routes[i].KeyName)
		if km.routes[i].Prefix == "*" {
			km.routes[i].Prefix = ""
		} else if strings.HasSuffix(km.routes[i].Prefix, "/*") {
			km.routes[i].Prefix = strings.TrimSuffix(km.routes[i].Prefix, "*")
		} else if strings.Contains(km.routes[i].Prefix, "*") {
			return nil, fmt.Errorf("invalid route prefix %q (wildcard must be at the end)", km.routes[i].Prefix)
		}
	}
	for _, route := range km.routes {
		if _, ok := km.keys[route.KeyName]; !ok {
			return nil, fmt.Errorf("route references unknown key %q", route.KeyName)
		}
	}

	// Sort routes by prefix length (longest first) for correct matching
	sort.Slice(km.routes, func(i, j int) bool {
		return len(km.routes[i].Prefix) > len(km.routes[j].Prefix)
	})

	return km, nil
}

// GetKey returns the key and key ID for the given object key.
// It routes based on the longest matching prefix.
// If no route matches, it returns the default key.
func (km *KeyManager) GetKey(objectKey string) (*Key, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	// Find the longest matching prefix
	for _, route := range km.routes {
		if strings.HasPrefix(objectKey, route.Prefix) {
			key, ok := km.keys[route.KeyName]
			if !ok {
				return nil, fmt.Errorf("key %q not found for prefix %q", route.KeyName, route.Prefix)
			}
			return key, nil
		}
	}

	// Return default key
	key, ok := km.keys[km.defaultKeyName]
	if !ok {
		return nil, fmt.Errorf("default key not found")
	}
	return key, nil
}

// GetKeyByID returns the key with the given name.
// This is used for decryption when the key ID is known from metadata.
func (km *KeyManager) GetKeyByID(keyName string) (*Key, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	// Empty key name means default key
	keyName = normalizeKeyName(keyName)
	if keyName == "" || keyName == "default" {
		key, ok := km.keys[km.defaultKeyName]
		if !ok {
			return nil, fmt.Errorf("default key not found")
		}
		return key, nil
	}

	key, ok := km.keys[keyName]
	if !ok {
		return nil, fmt.Errorf("key %q not found", keyName)
	}
	return key, nil
}

// GetMEK returns the MEK for the given object key.
// This is a convenience method that combines GetKey and returns the MEK.
func (km *KeyManager) GetMEK(objectKey string) ([]byte, string, error) {
	key, err := km.GetKey(objectKey)
	if err != nil {
		return nil, "", err
	}
	return key.MEK, key.Name, nil
}

// GetMEKByID returns the MEK for the given key ID.
func (km *KeyManager) GetMEKByID(keyName string) ([]byte, error) {
	key, err := km.GetKeyByID(keyName)
	if err != nil {
		return nil, err
	}
	return key.MEK, nil
}

// DefaultKey returns the default key.
func (km *KeyManager) DefaultKey() *Key {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.keys[km.defaultKeyName]
}

// UpdateDefaultKey updates the default MEK (used for key rotation).
func (km *KeyManager) UpdateDefaultKey(newMEK []byte) error {
	return km.UpdateKey("default", newMEK)
}

// UpdateKey updates an existing named MEK. It is used after a successful
// per-key rotation so new objects use the rotated key while existing objects
// remain decryptable during the rotation operation itself.
func (km *KeyManager) UpdateKey(keyName string, newMEK []byte) error {
	if len(newMEK) != 32 {
		return fmt.Errorf("MEK must be 32 bytes, got %d", len(newMEK))
	}

	keyName = normalizeKeyName(keyName)
	if keyName == "" {
		keyName = km.defaultKeyName
	}

	km.mu.Lock()
	defer km.mu.Unlock()
	if _, ok := km.keys[keyName]; !ok {
		return fmt.Errorf("key %q not found", keyName)
	}
	km.keys[keyName] = &Key{Name: keyName, MEK: append([]byte(nil), newMEK...)}
	return nil
}

// ListKeys returns the names of all configured keys.
func (km *KeyManager) ListKeys() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	names := make([]string, 0, len(km.keys))
	for name := range km.keys {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListRoutes returns all configured routes.
func (km *KeyManager) ListRoutes() []Route {
	km.mu.RLock()
	defer km.mu.RUnlock()

	routes := make([]Route, len(km.routes))
	copy(routes, km.routes)
	return routes
}

// GetMEKByFingerprint returns the MEK for a given key ID and fingerprint.
// It searches both the active key and the retired ring keys.
// Returns the MEK and true if found, nil and false otherwise.
func (km *KeyManager) GetMEKByFingerprint(keyID, fingerprint string) ([]byte, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keyID = normalizeKeyName(keyID)
	if keyID == "" {
		keyID = km.defaultKeyName
	}

	// Check active key first
	if key, exists := km.keys[keyID]; exists {
		if crypto.MEKFingerprint(key.MEK) == fingerprint {
			return key.MEK, true
		}
	}

	// Check ring keys
	if ring, exists := km.rings[keyID]; exists {
		for _, entry := range ring {
			if entry.Fingerprint == fingerprint {
				return entry.MEK, true
			}
		}
	}

	return nil, false
}

// Ring returns the ordered ring of retired-but-valid keys for a given key ID.
// Returns nil if the key has no ring configured.
func (km *KeyManager) Ring(keyID string) []RingKeyEntry {
	km.mu.RLock()
	defer km.mu.RUnlock()

	keyID = normalizeKeyName(keyID)
	if keyID == "" {
		keyID = km.defaultKeyName
	}

	if ring, exists := km.rings[keyID]; exists {
		// Return a copy to prevent mutation
		result := make([]RingKeyEntry, len(ring))
		copy(result, ring)
		return result
	}

	return nil
}

// ParseKeyRoutes parses a key routes string.
// Format: "prefix1=key1,prefix2=key2,*=default"
// The * prefix is a catch-all that maps to the default key.
func ParseKeyRoutes(routesStr string) ([]Route, error) {
	if routesStr == "" {
		return nil, nil
	}

	var routes []Route
	parts := strings.Split(routesStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid route format %q (expected prefix=keyname)", part)
		}

		prefix := strings.TrimSpace(kv[0])
		keyName := normalizeKeyName(kv[1])

		if prefix == "" || keyName == "" {
			return nil, fmt.Errorf("invalid route %q (empty prefix or key name)", part)
		}

		// A bare * is the catch-all route. A trailing /* is the documented
		// path-prefix notation, so normalize it to the prefix before the '*'.
		if prefix == "*" {
			prefix = ""
		} else if strings.HasSuffix(prefix, "/*") {
			prefix = strings.TrimSuffix(prefix, "*")
		} else if strings.Contains(prefix, "*") {
			return nil, fmt.Errorf("invalid route %q (wildcard must be at the end of the prefix)", part)
		}

		routes = append(routes, Route{
			Prefix:  prefix,
			KeyName: keyName,
		})
	}

	return routes, nil
}

// ParseNamedKeys extracts named keys from environment variables.
// It looks for variables matching the pattern ARMOR_MEK_<NAME>.
func ParseNamedKeys(envVars map[string]string) (map[string][]byte, error) {
	namedKeys := make(map[string][]byte)

	for name, value := range envVars {
		// Skip ARMOR_MEK (the default key)
		if name == "ARMOR_MEK" {
			continue
		}

		// Check for ARMOR_MEK_<NAME> pattern
		if strings.HasPrefix(name, "ARMOR_MEK_") {
			// Extract the key name (lowercase for consistency)
			keyName := normalizeKeyName(strings.TrimPrefix(name, "ARMOR_MEK_"))
			if keyName == "" || keyName == "default" {
				return nil, fmt.Errorf("invalid named key %q", name)
			}

			// Decode hex MEK
			mek, err := hex.DecodeString(value)
			if err != nil {
				return nil, fmt.Errorf("invalid hex for %s: %w", name, err)
			}

			if len(mek) != 32 {
				return nil, fmt.Errorf("%s must be 32 bytes, got %d", name, len(mek))
			}

			namedKeys[keyName] = mek
		}
	}

	return namedKeys, nil
}

func normalizeKeyName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
