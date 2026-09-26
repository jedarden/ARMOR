package config

import (
	"os"
	"path"
	"strings"
	"testing"
)

// ADR-001 "Internal Namespaces" makes ARMOR_MANIFEST_PREFIX relative to
// ARMOR_PREFIX: it may relocate a tenant's manifests anywhere inside that
// tenant's namespace but must never escape it — not into a sibling tenant's
// space, not above the tenant prefix, and (when no prefix is in force) not
// above the bucket root. TestManifestPrefixConfinedToTenantNamespace pins the
// headline cases; this suite is the full adversarial matrix the invariant
// rests on: every ARMOR_PREFIX shape crossed with every adversarial
// ARMOR_MANIFEST_PREFIX shape, asserting for each row either a hard refusal at
// Load (never a silent rewrite) or a composed prefix that stays confined.
//
// Composition itself is pinned as raw concatenation — trailing slashes,
// doubled slashes and "./" detours ride through verbatim rather than being
// cleaned away — because instances must agree byte for byte on where a
// tenant's deltas live: two instances that normalized the same input
// differently would silently stop seeing each other's deltas.

// escapeMatrixPrefixes is the ARMOR_PREFIX dimension. normalized is what
// normalizePrefix must yield for the raw value (asserted against cfg.Prefix
// below, so a normalization change fails loudly here instead of quietly
// invalidating the expectations built on these strings).
var escapeMatrixPrefixes = []struct {
	name       string
	raw        string
	normalized string
}{
	{name: "no-prefix", raw: "", normalized: ""},
	{name: "bare", raw: "p", normalized: "p/"},
	{name: "trailing-slash", raw: "p/", normalized: "p/"},
	{name: "leading-slash", raw: "/lead", normalized: "lead/"},
	{name: "nested", raw: "deep/nest/", normalized: "deep/nest/"},
}

// manifestUnset marks the row that leaves ARMOR_MANIFEST_PREFIX unset so the
// default applies.
var manifestUnset = struct{}{}

// escapeMatrixValues is the ARMOR_MANIFEST_PREFIX dimension. rejected rows are
// the escape attempts: absolute-looking values (any leading slash at all) and
// values whose cleaned form climbs out of the namespace. The remaining rows
// are accepted and must compose verbatim, however odd they look — confinement
// is the invariant, not prettiness.
var escapeMatrixValues = []struct {
	name     string
	value    any // string, or manifestUnset to leave the variable unset
	rejected bool
}{
	// -- escape attempts: hard-fail at Load, never clamp or rewrite --
	{name: "short-absolute", value: "/x", rejected: true},
	{name: "double-leading-slash", value: "//x", rejected: true},
	{name: "bare-root-absolute", value: "/", rejected: true},
	{name: "full-absolute", value: "/var/armor/manifest", rejected: true},
	{name: "bare-dotdot", value: "..", rejected: true},
	{name: "direct-climb", value: "../x", rejected: true},
	{name: "climb-after-detour", value: "x/../../y", rejected: true},
	{name: "dot-noise-climb", value: "a/./../..", rejected: true},

	// -- accepted: relocation within the namespace --
	{name: "unset-defaults", value: manifestUnset},
	{name: "explicit-empty-is-unset", value: ""},
	{name: "explicit-default", value: ".armor/manifest"},
	{name: "trailing-slash", value: ".armor/manifest/"},
	{name: "double-trailing-slash", value: ".armor/manifest//"},
	{name: "plain-relocation", value: "deeper/manifest"},
	{name: "relocation-trailing-slash", value: "deeper/manifest/"},
	{name: "dot-is-namespace-root", value: "."},
	{name: "dot-relative", value: "./manifest"},
	{name: "detour-cleans-to-root", value: "sub/.."},
	{name: "dotdot-leading-name", value: "..hidden"},
	{name: "internal-doubled-slash", value: "a//b"},
	{name: "leading-dot-slash-doubled", value: ".//x"},
}

// setEscapeMatrixEnv pins ARMOR_PREFIX and, when set is true, ARMOR_MANIFEST_PREFIX
// to the given values. A set-but-empty ARMOR_MANIFEST_PREFIX is a distinct
// operator input from an unset one even though getEnv treats them alike — the
// explicit-empty-is-unset row exists to pin that equivalence — so the helper
// takes an explicit set flag instead of overloading the empty string the way
// setManifestPrefixEnv does.
func setEscapeMatrixEnv(t *testing.T, armorPrefix, manifestPrefix string, manifestSet bool) {
	t.Helper()

	const prefixKey = "ARMOR_PREFIX"
	const manifestKey = "ARMOR_MANIFEST_PREFIX"

	prevPrefix, hadPrefix := os.LookupEnv(prefixKey)
	prevManifest, hadManifest := os.LookupEnv(manifestKey)

	if armorPrefix == "" {
		os.Unsetenv(prefixKey)
	} else if err := os.Setenv(prefixKey, armorPrefix); err != nil {
		t.Fatalf("Setenv(%s): %v", prefixKey, err)
	}
	if manifestSet {
		if err := os.Setenv(manifestKey, manifestPrefix); err != nil {
			t.Fatalf("Setenv(%s): %v", manifestKey, err)
		}
	} else {
		os.Unsetenv(manifestKey)
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

// assertManifestPrefixConfined is the invariant the matrix asserts for accepted
// values: the composed prefix stays inside the tenant namespace both as a
// literal string (what the writer/loader concatenate keys onto) and after
// Clean (what the filesystem backend's Join resolves it to). An empty tenant
// prefix means the bucket root is the namespace, so confinement there means
// only "does not climb above the root".
func assertManifestPrefixConfined(t *testing.T, tenantPrefix, composed string) {
	t.Helper()

	if !strings.HasPrefix(composed, tenantPrefix) {
		t.Errorf("composed manifest prefix %q escapes the tenant namespace %q", composed, tenantPrefix)
	}
	cleaned := path.Clean(composed)
	if path.IsAbs(cleaned) {
		t.Errorf("cleaned manifest prefix %q is absolute", cleaned)
		return
	}
	if tenantPrefix == "" {
		if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
			t.Errorf("cleaned manifest prefix %q climbs out of the bucket root", cleaned)
		}
		return
	}
	if root := strings.TrimSuffix(tenantPrefix, "/"); cleaned != root && !strings.HasPrefix(cleaned, tenantPrefix) {
		t.Errorf("cleaned manifest prefix %q escapes the tenant namespace %q", cleaned, tenantPrefix)
	}
}

// TestManifestPrefixEscapeMatrix crosses every ARMOR_PREFIX shape with every
// adversarial ARMOR_MANIFEST_PREFIX shape. Escape attempts must be refused at
// Load with a nil config — a hard fail, never a silent rewrite — and accepted
// values must compose onto the normalized tenant prefix verbatim and stay
// confined.
func TestManifestPrefixEscapeMatrix(t *testing.T) {
	for _, scope := range escapeMatrixPrefixes {
		for _, mv := range escapeMatrixValues {
			t.Run(scope.name+"/"+mv.name, func(t *testing.T) {
				setEnv(t, minimalEnv()...)

				var rawValue string
				manifestSet := true
				switch v := mv.value.(type) {
				case string:
					rawValue = v
				case struct{}:
					manifestSet = false
				default:
					t.Fatalf("unhandled value type %T", mv.value)
				}
				setEscapeMatrixEnv(t, scope.raw, rawValue, manifestSet)

				cfg, err := Load()
				if mv.rejected {
					if err == nil {
						t.Fatalf("Load() accepted ARMOR_MANIFEST_PREFIX=%q, want rejection", rawValue)
					}
					if cfg != nil {
						t.Errorf("Load() returned a config alongside the rejection; ManifestPrefix = %q", cfg.ManifestPrefix)
					}
					if !strings.Contains(err.Error(), "ARMOR_MANIFEST_PREFIX") {
						t.Errorf("Load() error %v does not name ARMOR_MANIFEST_PREFIX", err)
					}
					if !strings.Contains(err.Error(), rawValue) {
						t.Errorf("Load() error %v does not echo the refused value %q", err, rawValue)
					}
					return
				}

				if err != nil {
					t.Fatalf("Load() error: %v", err)
				}

				// The tenant prefix must be normalized exactly as this matrix
				// assumes, or the expectations below are built on sand.
				if cfg.Prefix != scope.normalized {
					t.Fatalf("cfg.Prefix = %q, want %q", cfg.Prefix, scope.normalized)
				}

				// Composition is raw concatenation: unset and empty fall back
				// to the default, everything else rides through verbatim.
				eff := rawValue
				if !manifestSet || eff == "" {
					eff = ".armor/manifest"
				}
				want := scope.normalized + eff
				if cfg.ManifestPrefix != want {
					t.Errorf("ManifestPrefix = %q, want %q (raw composition, no rewrite)", cfg.ManifestPrefix, want)
				}

				assertManifestPrefixConfined(t, scope.normalized, cfg.ManifestPrefix)
			})
		}
	}
}
