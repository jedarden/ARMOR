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
)

// Key represents a named master encryption key.
type Key struct {
	Name string
	MEK  []byte
}

// Route represents a prefix-to-key mapping.
type Route struct {
	Prefix  string
	KeyName string
}

// KeyManager manages multiple MEKs and routes object keys to the appropriate key.
type KeyManager struct {
	mu     sync.RWMutex
	keys   map[string]*Key    // name -> key
	routes []Route            // sorted by prefix length (longest first)
	defaultKeyName string
}

// New creates a new KeyManager with the given keys and routes.
func New(defaultMEK []byte, namedKeys map[string][]byte, routes []Route) (*KeyManager, error) {
	km := &KeyManager{
		keys:   make(map[string]*Key),
		routes: routes,
		defaultKeyName: "default",
	}

	// Add the default key
	if len(defaultMEK) != 32 {
		return nil, fmt.Errorf("default MEK must be 32 bytes, got %d", len(defaultMEK))
	}
	km.keys["default"] = &Key{Name: "default", MEK: defaultMEK}

	// Add named keys
	for name, mek := range namedKeys {
		if len(mek) != 32 {
			return nil, fmt.Errorf("MEK for key %q must be 32 bytes, got %d", name, len(mek))
		}
		km.keys[name] = &Key{Name: name, MEK: mek}
	}

	// Validate routes reference existing keys
	for _, route := range routes {
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
	if len(newMEK) != 32 {
		return fmt.Errorf("MEK must be 32 bytes, got %d", len(newMEK))
	}

	km.mu.Lock()
	defer km.mu.Unlock()
	km.keys[km.defaultKeyName] = &Key{Name: "default", MEK: newMEK}
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
		keyName := strings.TrimSpace(kv[1])

		if prefix == "" || keyName == "" {
			return nil, fmt.Errorf("invalid route %q (empty prefix or key name)", part)
		}

		// Handle wildcard - it maps to default key
		if prefix == "*" {
			prefix = ""
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
			keyName := strings.TrimPrefix(name, "ARMOR_MEK_")
			keyName = strings.ToLower(keyName)

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
