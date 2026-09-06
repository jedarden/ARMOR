package config

import (
	"os"
	"testing"
)

// setManifestPrefixEnv pins the two variables that determine the effective
// manifest prefix, restoring whatever was there on cleanup. An empty value
// unsets the variable so the default applies; passing "" for ARMOR_PREFIX also
// evicts a value leaked in by an earlier test, which minimalEnv does not cover.
func setManifestPrefixEnv(t *testing.T, armorPrefix, manifestPrefix string) {
	t.Helper()

	const prefixKey = "ARMOR_PREFIX"
	const manifestKey = "ARMOR_MANIFEST_PREFIX"

	prevPrefix, hadPrefix := os.LookupEnv(prefixKey)
	prevManifest, hadManifest := os.LookupEnv(manifestKey)

	if armorPrefix == "" {
		os.Unsetenv(prefixKey)
	} else {
		if err := os.Setenv(prefixKey, armorPrefix); err != nil {
			t.Fatalf("Setenv(%s): %v", prefixKey, err)
		}
	}
	if manifestPrefix == "" {
		os.Unsetenv(manifestKey)
	} else {
		if err := os.Setenv(manifestKey, manifestPrefix); err != nil {
			t.Fatalf("Setenv(%s): %v", manifestKey, err)
		}
	}

	t.Cleanup(func() {
		if hadPrefix {
			os.Setenv(prefixKey, prevPrefix)
		} else {
			os.Unsetenv(prefixKey)
		}
		if hadManifest {
			os.Setenv(manifestKey, prevManifest)
		} else {
			os.Unsetenv(manifestKey)
		}
	})
}

// TestManifestPrefixComposedWithTenantPrefix verifies that ARMOR_MANIFEST_PREFIX
// is resolved relative to the ADR-001 tenant prefix rather than taken as a
// bucket-root path. Manifest deltas are internal bookkeeping: taken literally
// they land at <bucket>/.armor/manifest/ regardless of ARMOR_PREFIX, so every
// instance sharing the bucket loads every other tenant's deltas into its index
// and a B2 key scoped to namePrefix <tenant>/ is denied the write.
func TestManifestPrefixComposedWithTenantPrefix(t *testing.T) {
	tests := []struct {
		name           string
		armorPrefix    string
		manifestPrefix string // "" leaves ARMOR_MANIFEST_PREFIX unset
		want           string
	}{
		{
			name:           "no tenant prefix leaves the manifest at the bucket root",
			armorPrefix:    "",
			manifestPrefix: "",
			want:           ".armor/manifest",
		},
		{
			name:           "tenant prefix scopes the default manifest prefix",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "",
			want:           "needle-ledger/.armor/manifest",
		},
		{
			name:           "tenant prefix is normalized before composing",
			armorPrefix:    "needle-ledger",
			manifestPrefix: "",
			want:           "needle-ledger/.armor/manifest",
		},
		{
			name:           "custom manifest prefix is relative to the tenant prefix",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: ".custom/manifest",
			want:           "needle-ledger/.custom/manifest",
		},
		{
			name:           "custom manifest prefix without a tenant prefix is literal",
			armorPrefix:    "",
			manifestPrefix: ".custom/manifest",
			want:           ".custom/manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalEnv()...)
			setManifestPrefixEnv(t, tt.armorPrefix, tt.manifestPrefix)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.ManifestPrefix != tt.want {
				t.Errorf("ManifestPrefix = %q, want %q", cfg.ManifestPrefix, tt.want)
			}
		})
	}
}
