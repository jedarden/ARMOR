// Package config provides hot-reload support for ARMOR_AUTH_FILE.
package config

import (
	"github.com/jedarden/armor/internal/acl"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// CredentialSet is a snapshot of credentials that can be atomically swapped.
type CredentialSet struct {
	credentials map[string]*Credential
	mtime       time.Time
}

// AuthFileWatcher monitors ARMOR_AUTH_FILE for changes and reloads credentials.
// Polls the file's mtime every 10 seconds (no fsnotify dependency).
// On change, parses into a new credential set and swaps atomically using
// atomic.Pointer — in-flight requests keep the old set.
// Parse errors keep the previous set and log at ERROR with the reason.
// Successful reloads log names added/removed (never keys).
// Env-defined credentials are re-merged each time.
type AuthFileWatcher struct {
	path         string
	envCreds     map[string]*Credential // Base env credentials (re-merged on reload)
	current      atomic.Pointer[CredentialSet]
	lastMtime    time.Time
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
}

// NewAuthFileWatcher creates a new watcher for the given auth file path.
// If path is empty, returns nil (file not configured).
func NewAuthFileWatcher(path string, initialCreds map[string]*Credential, envCreds map[string]*Credential) *AuthFileWatcher {
	if path == "" {
		return nil
	}

	// Get initial mtime
	mtime, err := getFileMtime(path)
	if err != nil {
		slog.Error("failed to get initial mtime for ARMOR_AUTH_FILE",
			"path", path,
			"error", err)
		// Continue anyway — we'll retry on next poll
		mtime = time.Time{}
	}

	w := &AuthFileWatcher{
		path:         path,
		envCreds:     envCreds,
		pollInterval: 10 * time.Second,
		stopCh:       make(chan struct{}),
	}

	// Store initial credential set
	initialSet := &CredentialSet{
		credentials: initialCreds,
		mtime:       mtime,
	}
	w.current.Store(initialSet)
	w.lastMtime = mtime

	return w
}

// Start begins the polling goroutine.
// Returns immediately; goroutine runs in background.
func (w *AuthFileWatcher) Start() {
	if w == nil {
		return
	}

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.checkAndReload()
			case <-w.stopCh:
				return
			}
		}
	}()

	slog.Info("started ARMOR_AUTH_FILE hot-reload watcher",
		"path", w.path,
		"interval", w.pollInterval)
}

// Stop gracefully shuts down the watcher.
// Blocks until the polling goroutine exits.
func (w *AuthFileWatcher) Stop() {
	if w == nil {
		return
	}

	close(w.stopCh)
	w.wg.Wait()

	slog.Info("stopped ARMOR_AUTH_FILE hot-reload watcher", "path", w.path)
}

// GetCredentials returns a snapshot of the current credential map.
// The returned map is safe for concurrent reads but must not be modified.
func (w *AuthFileWatcher) GetCredentials() map[string]*Credential {
	if w == nil {
		return nil
	}

	cs := w.current.Load()
	if cs == nil {
		return nil
	}

	return cs.credentials
}

// checkAndReload checks the file mtime and reloads if changed.
func (w *AuthFileWatcher) checkAndReload() {
	w.mu.Lock()
	defer w.mu.Unlock()

	mtime, err := getFileMtime(w.path)
	if err != nil {
		slog.Error("failed to get mtime for ARMOR_AUTH_FILE",
			"path", w.path,
			"error", err)
		return
	}

	// No change
	if mtime.Equal(w.lastMtime) || mtime.Before(w.lastMtime) {
		return
	}

	// File changed — reload
	slog.Info("ARMOR_AUTH_FILE mtime changed, reloading",
		"path", w.path,
		"old_mtime", w.lastMtime,
		"new_mtime", mtime)

	w.reload(mtime)
}

// reload loads and swaps in a new credential set.
// On error, keeps the old set and logs at ERROR.
func (w *AuthFileWatcher) reload(newMtime time.Time) {
	// Load and parse the file
	authFile, err := LoadAuthFile()
	if err != nil {
		slog.Error("failed to reload ARMOR_AUTH_FILE, keeping previous credentials",
			"path", w.path,
			"error", err)
		return
	}

	// Validate that we got a non-nil file
	if authFile == nil {
		slog.Error("ARMOR_AUTH_FILE returned nil after successful load, keeping previous credentials",
			"path", w.path)
		return
	}

	// Build new credential set by merging file creds with env creds
	newCreds := w.buildNewCredentialSet(authFile)

	// Compute diff for logging (names only, never keys)
	oldSet := w.current.Load()
	oldNames := w.getCredentialNames(oldSet)
	newNames := w.getCredentialNamesFromMap(newCreds)

	added, removed := diffNames(oldNames, newNames)

	// Swap atomically
	newSet := &CredentialSet{
		credentials: newCreds,
		mtime:       newMtime,
	}
	w.current.Store(newSet)
	w.lastMtime = newMtime

	// Log success with diff
	slog.Info("reloaded ARMOR_AUTH_FILE successfully",
		"path", w.path,
		"added_count", len(added),
		"removed_count", len(removed),
		"total_count", len(newCreds),
		"added_names", added,
		"removed_names", removed)
}

// buildNewCredentialSet creates a new credential map by merging file and env creds.
// Env credentials win on name collision.
func (w *AuthFileWatcher) buildNewCredentialSet(authFile *AuthFile) map[string]*Credential {
	// Start with a copy of env credentials
	newCreds := make(map[string]*Credential)
	for ak, c := range w.envCreds {
		newCreds[ak] = c
	}

	// Merge file credentials
	for _, fileCred := range authFile.Credentials {
		// Env wins on collision
		if _, exists := newCreds[fileCred.AccessKey]; exists {
			slog.Warn("credential name collision during reload - env credential takes precedence",
				"name", fileCred.Name,
				"access_key", fileCred.AccessKey,
				"source", "env")
			continue
		}

		// Parse ACL
		var acls []acl.ACLEntry
		var err error
		if fileCred.ACL != "" {
			acls, err = parseACL(fileCred.ACL)
			if err != nil {
				slog.Error("failed to parse ACL for credential in file, skipping",
					"name", fileCred.Name,
					"error", err)
				continue
			}
		}

		newCreds[fileCred.AccessKey] = &Credential{
			AccessKey: fileCred.AccessKey,
			SecretKey: fileCred.SecretKey,
			ACLs:      acls,
		}
	}

	return newCreds
}

// getCredentialNames extracts credential names from a CredentialSet.
func (w *AuthFileWatcher) getCredentialNames(cs *CredentialSet) map[string]string {
	if cs == nil {
		return nil
	}

	names := make(map[string]string)
	for _, cred := range cs.credentials {
		// Use access key as name since we don't store the friendly name
		names[cred.AccessKey] = cred.AccessKey
	}
	return names
}

// getCredentialNamesFromMap extracts credential names from a credential map.
func (w *AuthFileWatcher) getCredentialNamesFromMap(creds map[string]*Credential) map[string]string {
	names := make(map[string]string)
	for accessKey := range creds {
		names[accessKey] = accessKey
	}
	return names
}

// diffNames computes added and removed name sets.
func diffNames(old, new map[string]string) (added, removed []string) {
	for name := range new {
		if _, exists := old[name]; !exists {
			added = append(added, name)
		}
	}
	for name := range old {
		if _, exists := new[name]; !exists {
			removed = append(removed, name)
		}
	}
	return
}

// getFileMtime returns the modification time of the file at path.
func getFileMtime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
