// Package config handles ARMOR configuration via environment variables and YAML files.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jedarden/armor/internal/acl"
	"gopkg.in/yaml.v3"
)

// AuthFile represents the YAML structure for ARMOR_AUTH_FILE.
// Schema is normative per plan §8.6.
type AuthFile struct {
	Credentials []FileCredential `yaml:"credentials"`
}

// FileCredential represents a single credential entry in the YAML file.
type FileCredential struct {
	Name      string `yaml:"name"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	ACL       string `yaml:"acl"`
}

// LoadAuthFile reads and parses the YAML credential file specified by
// ARMOR_AUTH_FILE. Returns nil if the env var is unset or empty.
// Validation errors name the entry index and field, never the values.
func LoadAuthFile() (*AuthFile, error) {
	path := os.Getenv("ARMOR_AUTH_FILE")
	if path == "" {
		return nil, nil
	}
	return loadAuthFileAtPath(path)
}

// loadAuthFileAtPath reads and parses the YAML credential file at the given path.
// Validation errors name the entry index and field, never the values.
func loadAuthFileAtPath(path string) (*AuthFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read ARMOR_AUTH_FILE: %s: %w", path, err)
	}

	var authFile AuthFile
	if err := yaml.Unmarshal(data, &authFile); err != nil {
		return nil, fmt.Errorf("failed to parse ARMOR_AUTH_FILE %q: %w", path, err)
	}

	// Validate entries
	if err := validateAuthFile(&authFile); err != nil {
		return nil, fmt.Errorf("ARMOR_AUTH_FILE validation failed: %w", err)
	}

	return &authFile, nil
}

// validateAuthFile validates the parsed auth file structure.
// Errors name the entry index and field, never the values.
func validateAuthFile(authFile *AuthFile) error {
	for i, cred := range authFile.Credentials {
		if cred.Name == "" {
			return fmt.Errorf("credentials[%d]: name is required", i)
		}
		if cred.AccessKey == "" {
			return fmt.Errorf("credentials[%d].name=%s: access_key is required", i, cred.Name)
		}
		if cred.SecretKey == "" {
			return fmt.Errorf("credentials[%d].name=%s: secret_key is required", i, cred.Name)
		}
		// ACL is optional, but if present, validate it
		if cred.ACL != "" {
			// Use the same ACL parser as env triplets for consistency
			_, err := parseACL(cred.ACL)
			if err != nil {
				return fmt.Errorf("credentials[%d].name=%s: acl: %w", i, cred.Name, err)
			}
		}
	}
	return nil
}

// MergeFileCredentials merges file credentials with existing env credentials.
// Env credentials win on name collision (logged at WARN level).
// Returns an error if parsing fails, but never partial state — either all
// file credentials are merged or none are.
func MergeFileCredentials(cfg *Config, authFile *AuthFile) error {
	if authFile == nil || len(authFile.Credentials) == 0 {
		return nil
	}

	// Create a temporary map to validate all file credentials before merging
	tempCreds := make(map[string]*Credential)
	var duplicateNames []string
	now := time.Now()

	for _, fileCred := range authFile.Credentials {
		// Check for name collision with existing credential
		if _, exists := cfg.Credentials[fileCred.AccessKey]; exists {
			// Env wins: log at WARN and skip
			slog.Warn("credential name collision - env credential takes precedence",
				"name", fileCred.Name,
				"access_key", fileCred.AccessKey,
				"source", "env")
			continue
		}

		// Check for duplicate names within the file itself
		if _, dupExists := tempCreds[fileCred.AccessKey]; dupExists {
			duplicateNames = append(duplicateNames, fileCred.Name)
			continue
		}

		// Parse ACL
		var acls []acl.ACLEntry
		var err error
		if fileCred.ACL != "" {
			acls, err = parseACL(fileCred.ACL)
			if err != nil {
				return fmt.Errorf("credentials[%d].name=%s: acl: %w", getIndex(authFile, fileCred.Name), fileCred.Name, err)
			}
		}

		tempCreds[fileCred.AccessKey] = &Credential{
			AccessKey: fileCred.AccessKey,
			SecretKey: fileCred.SecretKey,
			ACLs:      acls,
			Source:    CredentialSourceFile,
			LoadedAt:  now,
		}
	}

	// If we found duplicate names within the file, log them but don't fail
	for _, dupName := range duplicateNames {
		slog.Warn("duplicate credential name in file - skipping duplicates",
			"name", dupName,
			"source", "file")
	}

	// Merge all validated credentials
	for accessKey, cred := range tempCreds {
		cfg.Credentials[accessKey] = cred
	}

	return nil
}

// getIndex is a helper to find the index of a credential by name.
func getIndex(authFile *AuthFile, name string) int {
	for i, cred := range authFile.Credentials {
		if cred.Name == name {
			return i
		}
	}
	return -1
}
