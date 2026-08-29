// Package config tests credential file loading and merging.
package config

import (
	"os"
	"path/filepath"
	"testing"

)

// TestLoadAuthFile_Unset tests that LoadAuthFile returns nil when ARMOR_AUTH_FILE is unset.
func TestLoadAuthFile_Unset(t *testing.T) {
	// Ensure env var is unset
	os.Unsetenv("ARMOR_AUTH_FILE")

	authFile, err := LoadAuthFile()
	if err != nil {
		t.Fatalf("LoadAuthFile() with unset env var should return nil, got error: %v", err)
	}
	if authFile != nil {
		t.Fatalf("LoadAuthFile() with unset env var should return nil, got %+v", authFile)
	}
}

// TestLoadAuthFile_Empty tests that an empty ARMOR_AUTH_FILE is treated as unset.
func TestLoadAuthFile_Empty(t *testing.T) {
	os.Setenv("ARMOR_AUTH_FILE", "")
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	authFile, err := LoadAuthFile()
	if err != nil {
		t.Fatalf("LoadAuthFile() with empty env var should return nil, got error: %v", err)
	}
	if authFile != nil {
		t.Fatalf("LoadAuthFile() with empty env var should return nil, got %+v", authFile)
	}
}

// TestLoadAuthFile_NotFound tests that a non-existent file returns an error.
func TestLoadAuthFile_NotFound(t *testing.T) {
	os.Setenv("ARMOR_AUTH_FILE", "/nonexistent/path/credentials.yaml")
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	_, err := LoadAuthFile()
	if err == nil {
		t.Fatal("LoadAuthFile() with non-existent file should return error")
	}
	// Error should mention the path
	if msg := err.Error(); filepath.Base(msg) != "credentials.yaml" {
		t.Errorf("Error should mention the filename, got: %v", err)
	}
}

// TestLoadAuthFile_InvalidYAML tests that invalid YAML returns an error.
func TestLoadAuthFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "credentials.yaml")
	if err := os.WriteFile(tmpFile, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ARMOR_AUTH_FILE", tmpFile)
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	_, err := LoadAuthFile()
	if err == nil {
		t.Fatal("LoadAuthFile() with invalid YAML should return error")
	}
}

// TestLoadAuthFile_ValidStructure tests parsing a valid auth file.
func TestLoadAuthFile_ValidStructure(t *testing.T) {
	yamlContent := `credentials:
  - name: TEST_READER
    access_key: test-reader-key
    secret_key: test-reader-secret
    acl: "mybucket:readonly/*:get+list"
  - name: TEST_WRITER
    access_key: test-writer-key
    secret_key: test-writer-secret
    acl: "mybucket:uploads/*:put+list"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "credentials.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ARMOR_AUTH_FILE", tmpFile)
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	authFile, err := LoadAuthFile()
	if err != nil {
		t.Fatalf("LoadAuthFile() failed: %v", err)
	}

	if len(authFile.Credentials) != 2 {
		t.Fatalf("Expected 2 credentials, got %d", len(authFile.Credentials))
	}

	// Verify first credential
	if authFile.Credentials[0].Name != "TEST_READER" {
		t.Errorf("Expected name TEST_READER, got %s", authFile.Credentials[0].Name)
	}
	if authFile.Credentials[0].AccessKey != "test-reader-key" {
		t.Errorf("Expected access_key test-reader-key, got %s", authFile.Credentials[0].AccessKey)
	}
	if authFile.Credentials[0].SecretKey != "test-reader-secret" {
		t.Errorf("Expected secret_key test-reader-secret, got %s", authFile.Credentials[0].SecretKey)
	}
	if authFile.Credentials[0].ACL != "mybucket:readonly/*:get+list" {
		t.Errorf("Expected acl mybucket:readonly/*:get+list, got %s", authFile.Credentials[0].ACL)
	}

	// Verify second credential
	if authFile.Credentials[1].Name != "TEST_WRITER" {
		t.Errorf("Expected name TEST_WRITER, got %s", authFile.Credentials[1].Name)
	}
}

// TestLoadAuthFile_NoACL tests that ACL is optional.
func TestLoadAuthFile_NoACL(t *testing.T) {
	yamlContent := `credentials:
  - name: NO_ACL
    access_key: no-acl-key
    secret_key: no-acl-secret
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "credentials.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ARMOR_AUTH_FILE", tmpFile)
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	authFile, err := LoadAuthFile()
	if err != nil {
		t.Fatalf("LoadAuthFile() failed: %v", err)
	}

	if len(authFile.Credentials) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(authFile.Credentials))
	}
	if authFile.Credentials[0].ACL != "" {
		t.Errorf("Expected empty ACL, got %s", authFile.Credentials[0].ACL)
	}
}

// TestValidateAuthFile_MissingName tests validation error for missing name.
func TestValidateAuthFile_MissingName(t *testing.T) {
	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "",
				AccessKey: "test-key",
				SecretKey: "test-secret",
			},
		},
	}

	err := validateAuthFile(authFile)
	if err == nil {
		t.Fatal("validateAuthFile() with missing name should return error")
	}

	expected := "credentials[0]: name is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

// TestValidateAuthFile_MissingAccessKey tests validation error for missing access_key.
func TestValidateAuthFile_MissingAccessKey(t *testing.T) {
	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "TEST",
				AccessKey: "",
				SecretKey: "test-secret",
			},
		},
	}

	err := validateAuthFile(authFile)
	if err == nil {
		t.Fatal("validateAuthFile() with missing access_key should return error")
	}

	expected := "credentials[0].name=TEST: access_key is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

// TestValidateAuthFile_MissingSecretKey tests validation error for missing secret_key.
func TestValidateAuthFile_MissingSecretKey(t *testing.T) {
	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "TEST",
				AccessKey: "test-key",
				SecretKey: "",
			},
		},
	}

	err := validateAuthFile(authFile)
	if err == nil {
		t.Fatal("validateAuthFile() with missing secret_key should return error")
	}

	expected := "credentials[0].name=TEST: secret_key is required"
	if err.Error() != expected {
		t.Errorf("Expected error '%s', got '%s'", expected, err.Error())
	}
}

// TestValidateAuthFile_InvalidACL tests validation error for invalid ACL.
func TestValidateAuthFile_InvalidACL(t *testing.T) {
	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "TEST",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				ACL:       "invalid acl format",
			},
		},
	}

	err := validateAuthFile(authFile)
	if err == nil {
		t.Fatal("validateAuthFile() with invalid ACL should return error")
	}

	// Error should mention the credential and field
	if err == nil {
		t.Fatal("Expected error for invalid ACL")
	}
}

// TestValidateAuthFile_ValidACL tests that valid ACLs pass validation.
func TestValidateAuthFile_ValidACL(t *testing.T) {
	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "TEST",
				AccessKey: "test-key",
				SecretKey: "test-secret",
				ACL:       "mybucket:prefix/*:get+put+list",
			},
		},
	}

	err := validateAuthFile(authFile)
	if err != nil {
		t.Fatalf("validateAuthFile() with valid ACL failed: %v", err)
	}
}

// TestMergeFileCredentials_BasicMerge tests basic credential merging.
func TestMergeFileCredentials_BasicMerge(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{
			"env-key": {
				AccessKey: "env-key",
				SecretKey: "env-secret",
				ACLs:      nil,
			},
		},
	}

	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "FILE_CRED",
				AccessKey: "file-key",
				SecretKey: "file-secret",
				ACL:       "mybucket:file/*:get",
			},
		},
	}

	err := MergeFileCredentials(cfg, authFile)
	if err != nil {
		t.Fatalf("MergeFileCredentials() failed: %v", err)
	}

	// Should have both credentials
	if len(cfg.Credentials) != 2 {
		t.Fatalf("Expected 2 credentials after merge, got %d", len(cfg.Credentials))
	}

	// Check env credential is still present
	if cfg.Credentials["env-key"] == nil {
		t.Error("Env credential was lost during merge")
	}

	// Check file credential was added
	if cfg.Credentials["file-key"] == nil {
		t.Error("File credential was not added")
	}

	// Verify file credential details
	fileCred := cfg.Credentials["file-key"]
	if fileCred.AccessKey != "file-key" {
		t.Errorf("Expected access_key file-key, got %s", fileCred.AccessKey)
	}
	if fileCred.SecretKey != "file-secret" {
		t.Errorf("Expected secret_key file-secret, got %s", fileCred.SecretKey)
	}
	if len(fileCred.ACLs) != 1 {
		t.Errorf("Expected 1 ACL entry, got %d", len(fileCred.ACLs))
	}
	if len(fileCred.ACLs) > 0 {
		entry := fileCred.ACLs[0]
		if entry.Bucket != "mybucket" {
			t.Errorf("Expected bucket mybucket, got %s", entry.Bucket)
		}
		if entry.Prefix != "file/" {
			t.Errorf("Expected prefix file/, got %s", entry.Prefix)
		}
	}
}

// TestMergeFileCredentials_NameCollision tests that env wins on name collision.
func TestMergeFileCredentials_NameCollision(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{
			"shared-key": {
				AccessKey: "shared-key",
				SecretKey: "env-secret",
				ACLs:      nil,
			},
		},
	}

	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "FILE_CRED",
				AccessKey: "shared-key", // Same access key as env credential
				SecretKey: "file-secret",
				ACL:       "mybucket:file/*:get",
			},
		},
	}

	err := MergeFileCredentials(cfg, authFile)
	if err != nil {
		t.Fatalf("MergeFileCredentials() failed: %v", err)
	}

	// Should still have only 1 credential (env wins)
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential after collision, got %d", len(cfg.Credentials))
	}

	// Verify env secret is kept
	cred := cfg.Credentials["shared-key"]
	if cred.SecretKey != "env-secret" {
		t.Errorf("Expected env-secret to be preserved, got %s", cred.SecretKey)
	}
}

// TestMergeFileCredentials_DuplicateInFile tests handling of duplicate names within the file.
func TestMergeFileCredentials_DuplicateInFile(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{},
	}

	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "FIRST",
				AccessKey: "dup-key",
				SecretKey: "first-secret",
				ACL:       "bucket1:*",
			},
			{
				Name:      "SECOND",
				AccessKey: "dup-key", // Duplicate access key
				SecretKey: "second-secret",
				ACL:       "bucket2:*",
			},
		},
	}

	err := MergeFileCredentials(cfg, authFile)
	if err != nil {
		t.Fatalf("MergeFileCredentials() failed: %v", err)
	}

	// Should have only 1 credential (first one wins, second is skipped)
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential after duplicate handling, got %d", len(cfg.Credentials))
	}

	// Verify first secret is kept
	cred := cfg.Credentials["dup-key"]
	if cred.SecretKey != "first-secret" {
		t.Errorf("Expected first-secret to be preserved, got %s", cred.SecretKey)
	}
}

// TestMergeFileCredentials_NilAuthFile tests that nil auth file is handled gracefully.
func TestMergeFileCredentials_NilAuthFile(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{
			"env-key": {
				AccessKey: "env-key",
				SecretKey: "env-secret",
			},
		},
	}

	err := MergeFileCredentials(cfg, nil)
	if err != nil {
		t.Fatalf("MergeFileCredentials() with nil authFile failed: %v", err)
	}

	// Should still have only env credential
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(cfg.Credentials))
	}
}

// TestMergeFileCredentials_EmptyAuthFile tests that empty auth file is handled gracefully.
func TestMergeFileCredentials_EmptyAuthFile(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{
			"env-key": {
				AccessKey: "env-key",
				SecretKey: "env-secret",
			},
		},
	}

	authFile := &AuthFile{
		Credentials: []FileCredential{},
	}

	err := MergeFileCredentials(cfg, authFile)
	if err != nil {
		t.Fatalf("MergeFileCredentials() with empty authFile failed: %v", err)
	}

	// Should still have only env credential
	if len(cfg.Credentials) != 1 {
		t.Fatalf("Expected 1 credential, got %d", len(cfg.Credentials))
	}
}

// TestMergeFileCredentials_ACLParsing tests that ACLs are parsed correctly during merge.
func TestMergeFileCredentials_ACLParsing(t *testing.T) {
	cfg := &Config{
		Credentials: map[string]*Credential{},
	}

	authFile := &AuthFile{
		Credentials: []FileCredential{
			{
				Name:      "MULTI_ACL",
				AccessKey: "multi-key",
				SecretKey: "multi-secret",
				ACL:       "bucket1:prefix1/*:get+list,bucket2:prefix2/*:put+delete",
			},
		},
	}

	err := MergeFileCredentials(cfg, authFile)
	if err != nil {
		t.Fatalf("MergeFileCredentials() failed: %v", err)
	}

	cred := cfg.Credentials["multi-key"]
	if len(cred.ACLs) != 2 {
		t.Fatalf("Expected 2 ACL entries, got %d", len(cred.ACLs))
	}

	// Verify first entry
	if cred.ACLs[0].Bucket != "bucket1" {
		t.Errorf("Expected bucket1, got %s", cred.ACLs[0].Bucket)
	}
	if cred.ACLs[0].Prefix != "prefix1/" {
		t.Errorf("Expected prefix1/, got %s", cred.ACLs[0].Prefix)
	}

	// Verify actions
	if cred.ACLs[0].Actions == nil {
		t.Error("Expected actions to be set, got nil")
	} else {
		if !cred.ACLs[0].Actions["get"] {
			t.Error("Expected get action to be set")
		}
		if !cred.ACLs[0].Actions["list"] {
			t.Error("Expected list action to be set")
		}
	}
}

// TestIntegration_LoadAndMerge tests the full integration of loading and merging.
func TestIntegration_LoadAndMerge(t *testing.T) {
	// Create a temporary YAML file
	yamlContent := `credentials:
  - name: INTEGRATION_READER
    access_key: integration-reader-key
    secret_key: integration-reader-secret
    acl: "mybucket:integration/*:get"
  - name: INTEGRATION_WRITER
    access_key: integration-writer-key
    secret_key: integration-writer-secret
    acl: "mybucket:integration/*:put+list"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "credentials.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	os.Setenv("ARMOR_AUTH_FILE", tmpFile)
	defer os.Unsetenv("ARMOR_AUTH_FILE")

	// Load the file
	authFile, err := LoadAuthFile()
	if err != nil {
		t.Fatalf("LoadAuthFile() failed: %v", err)
	}

	// Create config with env credentials
	cfg := &Config{
		Credentials: map[string]*Credential{
			"env-key": {
				AccessKey: "env-key",
				SecretKey: "env-secret",
				ACLs:      nil,
			},
		},
	}

	// Merge
	if err := MergeFileCredentials(cfg, authFile); err != nil {
		t.Fatalf("MergeFileCredentials() failed: %v", err)
	}

	// Verify we have 3 credentials total
	if len(cfg.Credentials) != 3 {
		t.Fatalf("Expected 3 credentials, got %d", len(cfg.Credentials))
	}

	// Verify specific credentials exist
	requiredKeys := []string{"env-key", "integration-reader-key", "integration-writer-key"}
	for _, key := range requiredKeys {
		if cfg.Credentials[key] == nil {
			t.Errorf("Missing credential for key %s", key)
		}
	}
}
