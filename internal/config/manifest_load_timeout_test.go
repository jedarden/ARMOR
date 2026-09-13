package config

import (
	"os"
	"testing"
)

// TestManifestLoadTimeoutDefault pins the default bound on the startup
// manifest load: 480s sits under the 600s startupProbe budget shipped in the
// deployment manifests (periodSeconds 10 x failureThreshold 60), so a slow
// cold load times out into the non-fatal empty-index fallback before the
// probe kills the container and crash-loops the pod.
func TestManifestLoadTimeoutDefault(t *testing.T) {
	setEnv(t, minimalEnv()...)
	// Unset so the default applies even where the operator env leaks in.
	os.Unsetenv("ARMOR_MANIFEST_LOAD_TIMEOUT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ManifestLoadTimeout != 480 {
		t.Errorf("ManifestLoadTimeout default = %d, want 480", cfg.ManifestLoadTimeout)
	}
}

// TestManifestLoadTimeoutOverride covers the operator override and the
// explicit opt-out: <= 0 disables the bound, restoring the unbounded load.
func TestManifestLoadTimeoutOverride(t *testing.T) {
	setEnv(t, minimalEnv()...)

	os.Setenv("ARMOR_MANIFEST_LOAD_TIMEOUT", "90")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.ManifestLoadTimeout != 90 {
		t.Errorf("ManifestLoadTimeout = %d, want 90", cfg.ManifestLoadTimeout)
	}

	os.Setenv("ARMOR_MANIFEST_LOAD_TIMEOUT", "0")
	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() error with 0: %v", err)
	}
	if cfg.ManifestLoadTimeout != 0 {
		t.Errorf("ManifestLoadTimeout = %d, want 0 (unbounded)", cfg.ManifestLoadTimeout)
	}
}
