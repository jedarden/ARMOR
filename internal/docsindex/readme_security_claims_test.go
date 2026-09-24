package docsindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestREADMEEncryptionClaimsAreQualified guards the README's encryption
// claims against regressing to unconditional statements. ADR-005 records
// that version 1 envelopes reuse keystream between adjacent blocks, so the
// zero-knowledge claim is true only for objects written or migrated to
// envelope v2/v3. The README must scope the claim to those versions, state
// that legacy v1 objects require migration before it applies, and link the
// migration and verification workflow.
func TestREADMEEncryptionClaimsAreQualified(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)

	// Any line stating the zero-knowledge claim must carry the version
	// caveat: it has to name v1 (the legacy exception) and scope the
	// guarantee to v2 or v3. A claim with neither is the regression
	// ADR-005 warns about.
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "Zero-knowledge encryption") {
			continue
		}
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "v1") ||
			(!strings.Contains(lower, "v2") && !strings.Contains(lower, "v3")) {
			t.Errorf("README states a zero-knowledge claim without the version caveat: %q — the claim only holds for envelope v2/v3 objects (ADR-005)", line)
		}
	}

	// The README must distinguish the versions, gate the claim on
	// migration, and link the migration and verification workflow.
	required := []string{
		// The vulnerability record itself.
		"docs/adr/005-ctr-counter-stride-fix.md",
		// Legacy objects require migration before the claim applies.
		"armor migrate",
		"--target v3",
		// The migration workflow.
		"docs/research/migration/V3_Migration_Reference.md",
		// The verification workflow: offline audit and continuous restore
		// verification.
		"armor verify",
		"docs/restore-verifier-deployment-guide.md",
	}
	for _, want := range required {
		if !strings.Contains(text, want) {
			t.Errorf("README is missing required encryption-claim material %q", want)
		}
	}
}
