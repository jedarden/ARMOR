package docsindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestADR005VersionClaimsAreCurrent guards ADR-005 and the code comments
// that describe the write format against regressing to presenting envelope
// v2 as the current write target. ADR-005's write mandate ("Version 2 is
// mandatory for all new objects") described the format landscape when v2 was
// the newest version; v3 has been the default write format since
// ARMOR_FORMAT_VERSION began defaulting to 3. The ADR must keep every v2
// write-target claim framed as history, state the current v3 boundary and
// the legacy migration split (v1 unsafe legacy, v2 safe legacy), and the
// code comments must not claim v2 is what new objects use.
func TestADR005VersionClaimsAreCurrent(t *testing.T) {
	root := repoRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "docs", "adr", "005-ctr-counter-stride-fix.md"))
	if err != nil {
		t.Fatal(err)
	}
	adr := string(raw)

	// Any ADR-005 line that asserts a normative claim about v2 (mandatory,
	// must use, should use) must qualify itself: either it also names v3,
	// or it carries an explicit historical marker. An unhedged line is the
	// drift this test exists for. (Quoting the original decision inside
	// quotes is fine — the qualifier travels on the same line.)
	markers := []string{
		"historical", "superseded", "no longer", "at the time",
		"described the state",
	}
	for _, line := range strings.Split(adr, "\n") {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "v2") && !strings.Contains(lower, "version 2") {
			continue
		}
		normative := strings.Contains(lower, "mandatory") ||
			strings.Contains(lower, "must use") ||
			strings.Contains(lower, "should use")
		if !normative {
			continue
		}
		qualified := strings.Contains(lower, "v3") || strings.Contains(lower, "version 3")
		for _, m := range markers {
			if strings.Contains(lower, m) {
				qualified = true
				break
			}
		}
		if !qualified {
			t.Errorf("ADR-005 line makes an unqualified v2 write-target claim: %q — name v3 or mark the claim historical", line)
		}
	}

	// The ADR must carry the current-format material: the version-boundary
	// section, the v3 spec, the migration runbook, and the legacy boundary
	// (v1 excluded from migration by default; v2 selectable but not the
	// write default).
	required := []string{
		"## Current format state",
		"../format/envelope-v3.md",
		"../runbooks/format-migration.md",
		"ARMOR_FORMAT_VERSION",
		"armor migrate --target v3",
		"include=v1,v2",
	}
	for _, want := range required {
		if !strings.Contains(adr, want) {
			t.Errorf("ADR-005 is missing required current-format material %q", want)
		}
	}

	// Code comments must agree with the ADR: the stale v2-era claims must
	// stay out, and the current claim must be present.
	envelope, err := os.ReadFile(filepath.Join(root, "internal", "crypto", "envelope.go"))
	if err != nil {
		t.Fatal(err)
	}
	envelopeText := string(envelope)
	for _, stale := range []string{
		"All new objects should use Version2",
		"Defaults to Version2 for security",
	} {
		if strings.Contains(envelopeText, stale) {
			t.Errorf("internal/crypto/envelope.go still carries the stale v2 write-target claim %q — v3 is the current default write format", stale)
		}
	}

	config, err := os.ReadFile(filepath.Join(root, "internal", "config", "config.go"))
	if err != nil {
		t.Fatal(err)
	}
	configText := string(config)
	for _, stale := range []string{
		"2 (default) or 3 (future)",
	} {
		if strings.Contains(configText, stale) {
			t.Errorf("internal/config/config.go still carries the stale format-version claim %q — 3 is the default, 2 is the legacy escape hatch", stale)
		}
	}
	if !strings.Contains(configText, "3 (current, the default)") {
		t.Error("internal/config/config.go does not state the current format-version default (\"3 (current, the default)\")")
	}

	// The documentation index's ADR-005 row must not present the ADR as a
	// plain current decision: the row has to say the v2 write mandate was
	// superseded by envelope v3.
	index, err := os.ReadFile(filepath.Join(root, "docs", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(index), "\n") {
		if !strings.Contains(line, "adr/005-ctr-counter-stride-fix.md") {
			continue
		}
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "v3") {
			t.Errorf("docs/README.md ADR-005 row does not note the v3 supersession: %q", line)
		}
	}
}
