// Package config handles ARMOR configuration via environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// KeyRoute represents a prefix-to-key mapping for multi-key support.
type KeyRoute struct {
	Prefix  string
	KeyName string
}

// Config holds all ARMOR configuration.
type Config struct {
	// Server configuration
	Listen      string
	AdminListen string

	// B2 backend configuration
	B2Region          string
	B2Endpoint        string
	B2AccessKeyID     string
	B2SecretAccessKey string
	Bucket            string

	// Cloudflare download configuration
	CFDomain string

	// Encryption configuration
	MEK       []byte
	BlockSize int

	// Multi-key configuration
	NamedKeys map[string][]byte // Named MEKs (key name -> MEK)
	KeyRoutes []KeyRoute        // Prefix to key name mappings

	// Authentication credentials for ARMOR clients
	AuthAccessKey string
	AuthSecretKey string

	// Writer ID for provenance chain
	WriterID string

	// Cache configuration
	CacheMaxEntries int
	CacheTTL        int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		Listen:      getEnv("ARMOR_LISTEN", "0.0.0.0:9000"),
		AdminListen: getEnv("ARMOR_ADMIN_LISTEN", "127.0.0.1:9001"),
	}

	// Required B2 configuration
	cfg.B2Region = os.Getenv("ARMOR_B2_REGION")
	if cfg.B2Region == "" {
		return nil, fmt.Errorf("ARMOR_B2_REGION is required")
	}

	cfg.B2Endpoint = os.Getenv("ARMOR_B2_ENDPOINT")
	if cfg.B2Endpoint == "" {
		cfg.B2Endpoint = fmt.Sprintf("https://s3.%s.backblazeb2.com", cfg.B2Region)
	}

	cfg.B2AccessKeyID = os.Getenv("ARMOR_B2_ACCESS_KEY_ID")
	if cfg.B2AccessKeyID == "" {
		return nil, fmt.Errorf("ARMOR_B2_ACCESS_KEY_ID is required")
	}

	cfg.B2SecretAccessKey = os.Getenv("ARMOR_B2_SECRET_ACCESS_KEY")
	if cfg.B2SecretAccessKey == "" {
		return nil, fmt.Errorf("ARMOR_B2_SECRET_ACCESS_KEY is required")
	}

	cfg.Bucket = os.Getenv("ARMOR_BUCKET")
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("ARMOR_BUCKET is required")
	}

	// Cloudflare domain (required)
	cfg.CFDomain = os.Getenv("ARMOR_CF_DOMAIN")
	if cfg.CFDomain == "" {
		return nil, fmt.Errorf("ARMOR_CF_DOMAIN is required")
	}

	// Master encryption key (required)
	mekHex := os.Getenv("ARMOR_MEK")
	if mekHex == "" {
		return nil, fmt.Errorf("ARMOR_MEK is required")
	}
	var err error
	cfg.MEK, err = hex.DecodeString(mekHex)
	if err != nil {
		return nil, fmt.Errorf("ARMOR_MEK must be hex-encoded: %w", err)
	}
	if len(cfg.MEK) != 32 {
		return nil, fmt.Errorf("ARMOR_MEK must be 32 bytes (64 hex chars), got %d bytes", len(cfg.MEK))
	}

	// Block size (default 64KB)
	cfg.BlockSize = getEnvInt("ARMOR_BLOCK_SIZE", 65536)
	if cfg.BlockSize < 4096 || (cfg.BlockSize&(cfg.BlockSize-1)) != 0 {
		return nil, fmt.Errorf("ARMOR_BLOCK_SIZE must be a power of 2 >= 4096")
	}

	// Auth credentials (generate random if not provided)
	cfg.AuthAccessKey = os.Getenv("ARMOR_AUTH_ACCESS_KEY")
	if cfg.AuthAccessKey == "" {
		cfg.AuthAccessKey = generateRandomKey(16)
	}
	cfg.AuthSecretKey = os.Getenv("ARMOR_AUTH_SECRET_KEY")
	if cfg.AuthSecretKey == "" {
		cfg.AuthSecretKey = generateRandomKey(32)
	}

	// Writer ID (default to hostname)
	cfg.WriterID = os.Getenv("ARMOR_WRITER_ID")
	if cfg.WriterID == "" {
		cfg.WriterID, _ = os.Hostname()
		if cfg.WriterID == "" {
			cfg.WriterID = "armor-unknown"
		}
	}

	// Cache configuration
	cfg.CacheMaxEntries = getEnvInt("ARMOR_CACHE_MAX_ENTRIES", 10000)
	cfg.CacheTTL = getEnvInt("ARMOR_CACHE_TTL", 300)

	// Load named keys (ARMOR_MEK_<NAME>)
	cfg.NamedKeys = make(map[string][]byte)
	for _, env := range os.Environ() {
		// Look for ARMOR_MEK_<NAME> pattern
		if strings.HasPrefix(env, "ARMOR_MEK_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) != 2 {
				continue
			}
			name := strings.TrimPrefix(parts[0], "ARMOR_MEK_")
			name = strings.ToLower(name)
			if name == "" {
				continue
			}
			// Decode hex MEK
			mek, err := hex.DecodeString(parts[1])
			if err != nil {
				return nil, fmt.Errorf("ARMOR_MEK_%s must be hex-encoded: %w", name, err)
			}
			if len(mek) != 32 {
				return nil, fmt.Errorf("ARMOR_MEK_%s must be 32 bytes (64 hex chars), got %d bytes", name, len(mek))
			}
			cfg.NamedKeys[name] = mek
		}
	}

	// Load key routes (ARMOR_KEY_ROUTES)
	if routesStr := os.Getenv("ARMOR_KEY_ROUTES"); routesStr != "" {
		routes, err := parseKeyRoutes(routesStr)
		if err != nil {
			return nil, fmt.Errorf("ARMOR_KEY_ROUTES: %w", err)
		}
		cfg.KeyRoutes = routes
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate random key: %v", err))
	}
	return hex.EncodeToString(b)
}

// parseKeyRoutes parses a key routes string.
// Format: "prefix1=key1,prefix2=key2,*=default"
// The * prefix is a catch-all that maps to the default key.
func parseKeyRoutes(routesStr string) ([]KeyRoute, error) {
	if routesStr == "" {
		return nil, nil
	}

	var routes []KeyRoute
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

		routes = append(routes, KeyRoute{
			Prefix:  prefix,
			KeyName: keyName,
		})
	}

	return routes, nil
}
