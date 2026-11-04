// settings_test.go holds `armor config show`'s table contract: completeness
// against the variables the package actually reads, the redaction policy for
// secret material, and the source labels.
package config

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
)

// Planted secret values for the redaction tests, built from repeating
// low-entropy fragments so no fixture looks like real credential material,
// while staying distinctive enough that their appearance in any row value is
// an unambiguous leak.
var (
	plantedMEK      = append([]byte{0xee}, bytesOf(0x11, 31)...)
	plantedRing     = bytesOf(0x22, 32)
	plantedNamed    = bytesOf(0x33, 32)
	plantedMEKHex   = hex.EncodeToString(plantedMEK)
	plantedRingHex  = hex.EncodeToString(plantedRing)
	plantedNamedHex = hex.EncodeToString(plantedNamed)
	plantedSecret   = "planted-show-secret-value"
	plantedToken    = "planted-show-token-value"
)

// bytesOf returns n bytes of the given value.
func bytesOf(value byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = value
	}
	return out
}

// showEnv is a complete, loadable B2 deployment carrying one of every
// secret-bearing variable with a planted value, plus a named key with a ring
// and a second named credential, so the dynamic rows are exercised too.
// ARMOR_WRITER_ID is pinned because the hostname fallback varies by host and
// the row value must stay deterministic.
func showEnv() []string {
	return []string{
		"ARMOR_LISTEN", "127.0.0.1:9100",
		"ARMOR_B2_REGION", "us-east-005",
		"ARMOR_B2_ACCESS_KEY_ID", "show-key-id",
		"ARMOR_B2_SECRET_ACCESS_KEY", plantedSecret,
		"ARMOR_BUCKET", "show-bucket",
		"ARMOR_MEK", plantedMEKHex,
		"ARMOR_MEK_ARCHIVE", plantedNamedHex,
		"ARMOR_MEK_RING", plantedRingHex,
		"ARMOR_KEY_ROUTES", "data/*=archive,*=default",
		"ARMOR_AUTH_ACCESS_KEY", "show-access",
		"ARMOR_AUTH_SECRET_KEY", plantedSecret,
		"ARMOR_AUTH_TENANT_ACCESS_KEY", "show-tenant-access",
		"ARMOR_AUTH_TENANT_SECRET_KEY", plantedSecret,
		"ARMOR_ADMIN_TOKEN", plantedToken,
		"ARMOR_WRITER_ID", "show-writer",
	}
}

// loadForShow runs Load under showEnv and fails the test if it does not load.
func loadForShow(t *testing.T) *Config {
	t.Helper()
	setEnv(t, showEnv()...)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() under the show fixture: %v", err)
	}
	return cfg
}

// showRows loads the fixture configuration and indexes its rows by name.
func showRows(t *testing.T) map[string]SettingRow {
	t.Helper()
	rows := loadForShow(t).Settings()
	byName := make(map[string]SettingRow, len(rows))
	for _, row := range rows {
		byName[row.Name] = row
	}
	return byName
}

// TestSettingsCoversEveryKnownVariable pins completeness: every non-family
// entry of knownEnvVars — the same list the unknown-variable warning and the
// configuration-reference parity hold — must appear as a row, in every
// deployment. A variable added to Load without a row here ships a show table
// that silently omits it.
func TestSettingsCoversEveryKnownVariable(t *testing.T) {
	names := rowNames(loadForShow(t).Settings())
	have := make(map[string]bool, len(names))
	for _, name := range names {
		have[name] = true
	}

	var missing []string
	for _, known := range knownEnvVars {
		if strings.HasSuffix(known, "_") {
			// Family prefix: its members are the dynamic per-key and
			// per-credential rows (ARMOR_MEK_<NAME>, ARMOR_AUTH_<NAME>_*),
			// which have no fixed name to pin.
			continue
		}
		if !have[known] {
			missing = append(missing, known)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Settings() rows omit known variables %q; every variable the package reads needs a row", missing)
	}
}

// TestSettingsRedactsSecretMaterial plants a distinctive value through every
// secret-bearing variable and asserts none of them reaches a row value —
// only presence indicators ("<set>") and fingerprints may appear.
func TestSettingsRedactsSecretMaterial(t *testing.T) {
	byName := showRows(t)
	if len(byName) == 0 {
		t.Fatal("Settings() returned no rows")
	}

	secrets := []string{plantedMEKHex, plantedRingHex, plantedNamedHex, plantedSecret, plantedToken}
	for name, row := range byName {
		for _, secret := range secrets {
			if strings.Contains(row.Value, secret) {
				t.Errorf("row %s carries planted secret material; the policy is presence or fingerprint only", name)
			}
		}
		// The unpublished identifiers must be fingerprinted, not passed
		// through: bucket names and access-key IDs identify the target.
		for _, identifier := range []string{"show-bucket", "show-key-id", "show-access", "show-tenant-access"} {
			if strings.Contains(row.Value, identifier) {
				t.Errorf("row %s carries the unpublished identifier %q in the clear; it must render as a fingerprint", name, identifier)
			}
		}
	}

	// Key material renders as its fingerprint — the same one `armor check`'s
	// fingerprint probe prints, so an operator can match the two outputs.
	if got, want := byName["ARMOR_MEK"].Value, crypto.MEKFingerprint(plantedMEK); got != want {
		t.Errorf("ARMOR_MEK row value = %q, want the MEK fingerprint %q", got, want)
	}
	if got := byName["ARMOR_MEK"].Source; got != SettingSourceEnv {
		t.Errorf("ARMOR_MEK row source = %q, want %q", got, SettingSourceEnv)
	}
	if got, want := byName["ARMOR_MEK_ARCHIVE"].Value, crypto.MEKFingerprint(plantedNamed); got != want {
		t.Errorf("ARMOR_MEK_ARCHIVE row value = %q, want the named key's fingerprint %q", got, want)
	}
	if got, want := byName["ARMOR_MEK_RING"].Value, crypto.MEKFingerprint(plantedRing); got != want {
		t.Errorf("ARMOR_MEK_RING row value = %q, want the retired key's fingerprint %q", got, want)
	}
	for _, name := range []string{"ARMOR_ADMIN_TOKEN", "ARMOR_AUTH_SECRET_KEY", "ARMOR_B2_SECRET_ACCESS_KEY"} {
		if got := byName[name].Value; got != "<set>" {
			t.Errorf("%s row value = %q, want <set>", name, got)
		}
	}
}

// TestSettingsSourceLabels pins the source column: env for a variable the
// environment set, default for one left unset, derived for the composed B2
// endpoint, and skipped for variables the configured backend never reads.
func TestSettingsSourceLabels(t *testing.T) {
	byName := showRows(t)

	if got := byName["ARMOR_LISTEN"].Source; got != SettingSourceEnv {
		t.Errorf("ARMOR_LISTEN source = %q, want env (the fixture sets it)", got)
	}
	if got := byName["ARMOR_LISTEN"].Value; got != "127.0.0.1:9100" {
		t.Errorf("ARMOR_LISTEN value = %q, want the fixture value", got)
	}
	// The fixture leaves the endpoint unset, so Load composes it from the
	// region — a value no single variable supplied.
	if got := byName["ARMOR_B2_ENDPOINT"].Source; got != SettingSourceDerived {
		t.Errorf("ARMOR_B2_ENDPOINT source = %q, want derived (composed from the region)", got)
	}
	if got := byName["ARMOR_ADMIN_LISTEN"].Source; got != SettingSourceDefault {
		t.Errorf("ARMOR_ADMIN_LISTEN source = %q, want default (unset in the fixture)", got)
	}
	if got := byName["ARMOR_FS_PATH"].Source; got != SettingSourceSkipped {
		t.Errorf("ARMOR_FS_PATH source = %q, want skipped (ARMOR_BACKEND=b2 never reads it)", got)
	}

	// An explicit endpoint override flips the source to env.
	setEnv(t, "ARMOR_B2_ENDPOINT", "https://b2.example.com")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() with an explicit endpoint: %v", err)
	}
	for _, row := range cfg.Settings() {
		if row.Name == "ARMOR_B2_ENDPOINT" && row.Source != SettingSourceEnv {
			t.Errorf("ARMOR_B2_ENDPOINT source with an override = %q, want env", row.Source)
		}
	}
}

// TestSettingsSkipsB2RowsUnderFilesystem pins the mirror case: a filesystem
// deployment reads none of the B2 variables, and the table says so instead of
// implying the environment's values took effect.
func TestSettingsSkipsB2RowsUnderFilesystem(t *testing.T) {
	setEnv(t,
		"ARMOR_BACKEND", "filesystem",
		"ARMOR_FS_PATH", t.TempDir(),
		"ARMOR_BUCKET", "show-bucket",
		"ARMOR_MEK", plantedMEKHex,
		"ARMOR_ALLOW_NO_CREDENTIALS", "true",
		// Planted but never read under this backend: the row must not claim
		// the value had any effect.
		"ARMOR_B2_REGION", "us-east-005",
		"ARMOR_CF_DOMAIN", "cf.example.com",
	)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() under the filesystem fixture: %v", err)
	}
	byName := make(map[string]SettingRow)
	for _, row := range cfg.Settings() {
		byName[row.Name] = row
	}
	for _, name := range []string{
		"ARMOR_B2_REGION", "ARMOR_B2_ENDPOINT", "ARMOR_B2_ACCESS_KEY_ID",
		"ARMOR_B2_SECRET_ACCESS_KEY", "ARMOR_CF_DOMAIN", "ARMOR_FS_PATH",
		"ARMOR_SECONDARY_B2_KEY",
	} {
		row, ok := byName[name]
		if !ok {
			t.Errorf("Settings() omitted static row %q under the filesystem backend", name)
			continue
		}
		if name == "ARMOR_FS_PATH" {
			if row.Source != SettingSourceEnv {
				t.Errorf("ARMOR_FS_PATH source under filesystem = %q, want env", row.Source)
			}
			continue
		}
		if row.Source != SettingSourceSkipped {
			t.Errorf("%s source under filesystem = %q, want skipped", name, row.Source)
		}
		if row.Value != "" {
			t.Errorf("%s value under filesystem = %q, want empty (unread)", name, row.Value)
		}
	}
}

// TestSettingsDeterministic pins the table's shape: two loads of the same
// environment produce identical tables (no map-iteration order leaking into
// row order) and no row name repeats.
func TestSettingsDeterministic(t *testing.T) {
	first := loadForShow(t).Settings()

	setEnv(t, showEnv()...)
	again, err := Load()
	if err != nil {
		t.Fatalf("Load() second pass: %v", err)
	}
	second := again.Settings()

	if len(first) != len(second) {
		t.Fatalf("two loads produced different table sizes: %d vs %d", len(first), len(second))
	}
	seen := make(map[string]bool, len(first))
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("row %d differs between loads: %+v vs %+v", i, first[i], second[i])
		}
		if seen[first[i].Name] {
			t.Errorf("row name %q appears more than once", first[i].Name)
		}
		seen[first[i].Name] = true
	}
}

// TestSettingsCredentialRows carries the per-credential rows: keyed by access
// -key fingerprint, reporting the credential's own source (env or file) and
// ACL count, never the access key or secret itself.
func TestSettingsCredentialRows(t *testing.T) {
	var credRows []SettingRow
	for _, row := range loadForShow(t).Settings() {
		if strings.HasPrefix(row.Name, "credential/") {
			credRows = append(credRows, row)
		}
	}
	if len(credRows) != 2 {
		t.Fatalf("credential rows = %d, want 2 (default env credential + named tenant credential)", len(credRows))
	}
	for _, row := range credRows {
		if row.Source != string(CredentialSourceEnv) {
			t.Errorf("credential row %s source = %q, want %q", row.Name, row.Source, CredentialSourceEnv)
		}
		if !strings.Contains(row.Value, "secret <set>") {
			t.Errorf("credential row %s value = %q, want it to report the secret as <set>", row.Name, row.Value)
		}
	}
}

// rowNames returns the table's row names in order.
func rowNames(rows []SettingRow) []string {
	names := make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
	}
	return names
}
