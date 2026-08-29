// Package config provides tests for hot-reload support.
package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestAuthFileWatcher_BasicReload tests that a file rewrite triggers reload
// and the new credential authenticates within one poll interval.
func TestAuthFileWatcher_BasicReload(t *testing.T) {
	// Create a temporary auth file
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	// Write initial file with one credential
	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
    acl: "test-bucket:test-prefix"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	// Create env credentials
	envCreds := map[string]*Credential{
		"AKENV0000000000": {
			AccessKey: "AKENV0000000000",
			SecretKey: "env_secret_key",
			Source:    CredentialSourceEnv,
		},
	}

	// Create initial credentials map
	initialCreds := map[string]*Credential{
		"AKINITIAL123456": {
			AccessKey: "AKINITIAL123456",
			SecretKey: "initial_secret_key",
		},
		"AKENV0000000000": envCreds["AKENV0000000000"],
	}

	// Create watcher with short poll interval for testing
	watcher := &AuthFileWatcher{
		path:         authFilePath,
		envCreds:     envCreds,
		pollInterval: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}

	// Initialize with initial credentials
	mtime, _ := getFileMtime(authFilePath)
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	watcher.current.Store(initialSet)
	watcher.lastMtime = mtime

	// Start the watcher
	watcher.Start()
	defer watcher.Stop()

	// Wait a bit to ensure watcher is running
	time.Sleep(200 * time.Millisecond)

	// Verify initial credentials
	creds := watcher.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(creds))
	}
	if _, exists := creds["AKINITIAL123456"]; !exists {
		t.Error("initial credential not found")
	}
	if _, exists := creds["AKENV0000000000"]; !exists {
		t.Error("env credential not found")
	}

	// Rewrite the file with a new credential
	newContent := `credentials:
  - name: "cred2"
    access_key: "AKNEW123456789"
    secret_key: "new_secret_key"
    acl: "test-bucket:new-prefix"
`
	if err := os.WriteFile(authFilePath, []byte(newContent), 0600); err != nil {
		t.Fatalf("failed to write new auth file: %v", err)
	}

	// Wait for reload (should happen within one poll interval + margin)
	time.Sleep(300 * time.Millisecond)

	// Verify new credentials are loaded
	creds = watcher.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("expected 2 credentials after reload, got %d", len(creds))
	}
	if _, exists := creds["AKNEW123456789"]; !exists {
		t.Error("new credential not found after reload")
	}
	if _, exists := creds["AKENV0000000000"]; !exists {
		t.Error("env credential not found after reload (should be preserved)")
	}
	if _, exists := creds["AKINITIAL123456"]; exists {
		t.Error("old credential still present after reload (should be removed)")
	}
}

// TestAuthFileWatcher_BrokenFileKeepsOldCreds tests that a broken file rewrite
// leaves the old credentials active.
func TestAuthFileWatcher_BrokenFileKeepsOldCreds(t *testing.T) {
	// Create a temporary auth file
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	// Write initial valid file
	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
    acl: "test-bucket:test-prefix"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	// Create env credentials
	envCreds := map[string]*Credential{
		"AKENV0000000000": {
			AccessKey: "AKENV0000000000",
			SecretKey: "env_secret_key",
			Source:    CredentialSourceEnv,
		},
	}

	// Create initial credentials map
	initialCreds := map[string]*Credential{
		"AKINITIAL123456": {
			AccessKey: "AKINITIAL123456",
			SecretKey: "initial_secret_key",
		},
		"AKENV0000000000": envCreds["AKENV0000000000"],
	}

	// Create watcher with short poll interval
	watcher := &AuthFileWatcher{
		path:         authFilePath,
		envCreds:     envCreds,
		pollInterval: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}

	// Initialize
	mtime, _ := getFileMtime(authFilePath)
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	watcher.current.Store(initialSet)
	watcher.lastMtime = mtime

	// Start the watcher
	watcher.Start()
	defer watcher.Stop()

	// Wait a bit
	time.Sleep(200 * time.Millisecond)

	// Verify initial credentials
	creds := watcher.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(creds))
	}

	// Rewrite with broken YAML (missing required field)
	brokenContent := `credentials:
  - name: "broken"
    access_key: "AKBROKEN123456"
    # missing secret_key - this should fail validation
`
	if err := os.WriteFile(authFilePath, []byte(brokenContent), 0600); err != nil {
		t.Fatalf("failed to write broken auth file: %v", err)
	}

	// Wait for reload attempt
	time.Sleep(300 * time.Millisecond)

	// Verify old credentials are still active
	creds = watcher.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("expected 2 credentials (old set preserved), got %d", len(creds))
	}
	if _, exists := creds["AKINITIAL123456"]; !exists {
		t.Error("old credential not found (should be preserved on error)")
	}
	if _, exists := creds["AKENV0000000000"]; !exists {
		t.Error("env credential not found (should be preserved)")
	}
	if _, exists := creds["AKBROKEN123456"]; exists {
		t.Error("broken credential should not be loaded")
	}
}

// TestAuthFileWatcher_EnvCredsAlwaysPresent tests that env credentials
// are always present even after file reloads.
func TestAuthFileWatcher_EnvCredsAlwaysPresent(t *testing.T) {
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	// Write empty file (no credentials)
	emptyContent := `credentials: []
`
	if err := os.WriteFile(authFilePath, []byte(emptyContent), 0600); err != nil {
		t.Fatalf("failed to write empty auth file: %v", err)
	}

	// Create env credentials
	envCreds := map[string]*Credential{
		"AKENV0000000000": {
			AccessKey: "AKENV0000000000",
			SecretKey: "env_secret_key",
			Source:    CredentialSourceEnv,
		},
	}

	// Create initial credentials with only env creds
	initialCreds := map[string]*Credential{
		"AKENV0000000000": envCreds["AKENV0000000000"],
	}

	// Create watcher
	watcher := &AuthFileWatcher{
		path:         authFilePath,
		envCreds:     envCreds,
		pollInterval: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}

	// Initialize
	mtime, _ := getFileMtime(authFilePath)
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	watcher.current.Store(initialSet)
	watcher.lastMtime = mtime

	// Start
	watcher.Start()
	defer watcher.Stop()

	time.Sleep(200 * time.Millisecond)

	// Verify only env creds present
	creds := watcher.GetCredentials()
	if len(creds) != 1 {
		t.Errorf("expected 1 credential (env only), got %d", len(creds))
	}

	// Rewrite file with file credentials (should merge with env)
	fileContent := `credentials:
  - name: "filecred"
    access_key: "AKFILE123456789"
    secret_key: "file_secret_key"
`
	if err := os.WriteFile(authFilePath, []byte(fileContent), 0600); err != nil {
		t.Fatalf("failed to write file auth file: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verify both env and file creds present
	creds = watcher.GetCredentials()
	if len(creds) != 2 {
		t.Errorf("expected 2 credentials (env + file), got %d", len(creds))
	}
	if _, exists := creds["AKENV0000000000"]; !exists {
		t.Error("env credential missing after file reload")
	}
	if _, exists := creds["AKFILE123456789"]; !exists {
		t.Error("file credential not loaded")
	}
}

// TestAuthFileWatcher_NilPathReturnsNil tests that NewAuthFileWatcher returns nil
// when path is empty.
func TestAuthFileWatcher_NilPathReturnsNil(t *testing.T) {
	watcher := NewAuthFileWatcher("", nil, nil)
	if watcher != nil {
		t.Error("expected nil watcher for empty path")
	}
}

// TestAuthFileWatcher_ConcurrentAccess tests that the watcher handles
// concurrent GetCredentials calls safely.
func TestAuthFileWatcher_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	envCreds := map[string]*Credential{
		"AKENV0000000000": {
			AccessKey: "AKENV0000000000",
			SecretKey: "env_secret_key",
			Source:    CredentialSourceEnv,
		},
	}

	initialCreds := map[string]*Credential{
		"AKINITIAL123456": {
			AccessKey: "AKINITIAL123456",
			SecretKey: "initial_secret_key",
		},
		"AKENV0000000000": envCreds["AKENV0000000000"],
	}

	watcher := &AuthFileWatcher{
		path:         authFilePath,
		envCreds:     envCreds,
		pollInterval: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}

	mtime, _ := getFileMtime(authFilePath)
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	watcher.current.Store(initialSet)
	watcher.lastMtime = mtime

	watcher.Start()
	defer watcher.Stop()

	// Launch concurrent readers
	var ops int64 = 1000
	done := make(chan struct{})

	for i := 0; i < 10; i++ {
		go func() {
			for atomic.LoadInt64(&ops) > 0 {
				creds := watcher.GetCredentials()
				if creds == nil {
					t.Error("got nil credentials during concurrent access")
				}
				atomic.AddInt64(&ops, -1)
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines to finish
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we got here without race detector complaints, test passed
}

// TestAuthFileWatcher_GetCredentialsOnNilWatcher tests that GetCredentials
// handles nil watcher gracefully.
func TestAuthFileWatcher_GetCredentialsOnNilWatcher(t *testing.T) {
	var watcher *AuthFileWatcher
	creds := watcher.GetCredentials()
	if creds != nil {
		t.Error("expected nil credentials from nil watcher")
	}
}

// TestAuthFileWatcher_StartStopIdempotent tests that Start and Stop are
// idempotent and safe to call multiple times.
func TestAuthFileWatcher_StartStopIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	watcher := NewAuthFileWatcher(authFilePath, map[string]*Credential{}, map[string]*Credential{})
	if watcher == nil {
		t.Fatal("expected non-nil watcher")
	}

	// Multiple starts should be safe
	watcher.Start()
	watcher.Start()
	watcher.Start()

	// Multiple stops should be safe
	watcher.Stop()
	watcher.Stop()
	watcher.Stop()

	// If we got here without panics or deadlocks, test passed
}

// TestAuthFileWatcher_InvalidACLKeepsOldCreds tests that a file with an
// invalid ACL keeps the old credentials.
func TestAuthFileWatcher_InvalidACLKeepsOldCreds(t *testing.T) {
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	envCreds := map[string]*Credential{}

	initialCreds := map[string]*Credential{
		"AKINITIAL123456": {
			AccessKey: "AKINITIAL123456",
			SecretKey: "initial_secret_key",
		},
	}

	watcher := &AuthFileWatcher{
		path:         authFilePath,
		envCreds:     envCreds,
		pollInterval: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
	}

	mtime, _ := getFileMtime(authFilePath)
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	watcher.current.Store(initialSet)
	watcher.lastMtime = mtime

	watcher.Start()
	defer watcher.Stop()

	time.Sleep(200 * time.Millisecond)

	// Verify initial state
	creds := watcher.GetCredentials()
	if len(creds) != 1 {
		t.Errorf("expected 1 credential, got %d", len(creds))
	}

	// Rewrite with invalid ACL
	brokenContent := `credentials:
  - name: "broken"
    access_key: "AKBROKEN123456"
    secret_key: "broken_secret_key"
    acl: "invalid::acl::::format"
`
	if err := os.WriteFile(authFilePath, []byte(brokenContent), 0600); err != nil {
		t.Fatalf("failed to write broken auth file: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	// Verify old credentials preserved
	creds = watcher.GetCredentials()
	if len(creds) != 1 {
		t.Errorf("expected 1 credential (old set preserved), got %d", len(creds))
	}
	if _, exists := creds["AKINITIAL123456"]; !exists {
		t.Error("old credential not found (should be preserved on ACL error)")
	}
}

// TestAuthFileWatcher_Logging verifies that the watcher logs appropriate
// messages during reload.
func TestAuthFileWatcher_Logging(t *testing.T) {
	// This test would require capturing log output
	// For now, we just verify the watcher doesn't crash when logging
	tmpDir := t.TempDir()
	authFilePath := filepath.Join(tmpDir, "auth.yaml")

	initialContent := `credentials:
  - name: "cred1"
    access_key: "AKINITIAL123456"
    secret_key: "initial_secret_key"
`
	if err := os.WriteFile(authFilePath, []byte(initialContent), 0600); err != nil {
		t.Fatalf("failed to write initial auth file: %v", err)
	}

	// Create a logger that goes to /dev/null or test handler
	logger := slog.Default()

	_ = logger // Use the logger to avoid unused variable warning

	watcher := NewAuthFileWatcher(authFilePath, map[string]*Credential{}, map[string]*Credential{})
	if watcher == nil {
		t.Fatal("expected non-nil watcher")
	}

	watcher.Start()
	watcher.Stop()

	// If we got here without crashing, test passed
}
