package config

import (
	"os"
	"strings"
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

// TestManifestPrefixConfinedToTenantNamespace verifies the confinement half of
// the contract: ARMOR_MANIFEST_PREFIX may relocate the manifests anywhere
// within the tenant namespace, but a value that would climb out with "../" —
// which the filesystem backend resolves physically out of
// <FSPath>/<bucket>/<prefix>/ — or escape into an absolute path is rejected at
// Load rather than composed.
func TestManifestPrefixConfinedToTenantNamespace(t *testing.T) {
	tests := []struct {
		name           string
		armorPrefix    string
		manifestPrefix string // "" leaves ARMOR_MANIFEST_PREFIX unset
		wantErr        bool
		want           string // asserted when wantErr is false
	}{
		{
			name:           "parent traversal escapes the tenant namespace",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "../escape",
			wantErr:        true,
		},
		{
			name:           "nested traversal escapes after cleaning",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "sub/../../escape",
			wantErr:        true,
		},
		{
			name:           "bare dotdot escapes",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "..",
			wantErr:        true,
		},
		{
			name:           "traversal escapes the bucket root too",
			armorPrefix:    "",
			manifestPrefix: "../escape",
			wantErr:        true,
		},
		{
			name:           "absolute path does not compose onto a tenant prefix",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "/var/armor/manifest",
			wantErr:        true,
		},
		{
			name:           "absolute path is rejected without a tenant prefix",
			armorPrefix:    "",
			manifestPrefix: "/var/armor/manifest",
			wantErr:        true,
		},
		{
			name:           "relocation within the namespace is allowed",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "deeper/manifest",
			want:           "needle-ledger/deeper/manifest",
		},
		{
			name:           "in-namespace detour that cleans back is allowed",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "sub/../manifest",
			want:           "needle-ledger/sub/../manifest",
		},
		{
			name:           "dot-relative form is allowed",
			armorPrefix:    "needle-ledger/",
			manifestPrefix: "./manifest",
			want:           "needle-ledger/./manifest",
		},
		{
			name:           "relocation without a tenant prefix is literal",
			armorPrefix:    "",
			manifestPrefix: "deeper/manifest",
			want:           "deeper/manifest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, minimalEnv()...)
			setManifestPrefixEnv(t, tt.armorPrefix, tt.manifestPrefix)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Load() accepted ManifestPrefix %q, want rejection", tt.manifestPrefix)
				}
				if !strings.Contains(err.Error(), "ARMOR_MANIFEST_PREFIX") {
					t.Errorf("Load() error %v does not name ARMOR_MANIFEST_PREFIX", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error: %v", err)
			}
			if cfg.ManifestPrefix != tt.want {
				t.Errorf("ManifestPrefix = %q, want %q", cfg.ManifestPrefix, tt.want)
			}
		})
	}
}
