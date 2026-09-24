// Consistency harness binding the golden-outcomes manifest to the committed
// fixture tree (armor-73cb73d8).
//
// v3-golden-outcomes.json once claimed 32 on-disk fixture directories while
// the tree carried 16: the number had been taken from goldenFixtureMatrix's
// row list (internal/server/format_migration_fixture_matrix_test.go), not
// from disk. These tests pin the manifest's summary to the tree so the two
// can only move together:
//
//   - TestGoldenManifestOnDiskDirsMatchTree  the manifest's summary.on_disk_dirs
//     and the per-entry disk_path fields must equal exactly the committed
//     fixture directories (<category>/<variant> holding a metadata.json,
//     canonical/ excluded -- the same definition loadGoldenFixtures in
//     internal/server uses); the summary counts must be derivable from those
//     lists; and every planning-only row (no disk_path) must carry an explicit
//     disposition ("covered" naming a committed directory, or "deferred" with
//     a one-line reason stating it is not required for Phase 8.11 acceptance).
//   - TestGoldenManifestYAMLMatchesJSON  the .yml twin stays semantically
//     identical to the .json (both formats are committed; only their bytes
//     differ).
//
// The manifest is consumed as recorded data; nothing here recomputes
// migration outcomes (see generated_fixtures_validation_test.go for the
// expectation-discipline note).

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const (
	goldenManifestJSONPath = "v3-golden-outcomes.json"
	goldenManifestYAMLPath = "v3-golden-outcomes.yml"
)

type goldenManifestEntry struct {
	DiskPath          string `json:"disk_path"`
	ExpectedOutcome   string `json:"expected_outcome"`
	PlanningOnly      *bool  `json:"planning_only"`
	Disposition       string `json:"disposition"`
	CoveredBy         string `json:"covered_by"`
	DispositionReason string `json:"disposition_reason"`
}

type goldenManifestSummary struct {
	TotalEntries        int `json:"total_entries"`
	OnDiskFixtureDirs   int `json:"on_disk_fixture_dirs"`
	PlanningOnlyEntries int `json:"planning_only_entries"`

	OnDiskDirs             []string `json:"on_disk_dirs"`
	OnDiskExpectedOutcomes struct {
		Success int `json:"success"`
		Failure int `json:"failure"`
	} `json:"on_disk_expected_outcomes"`

	PlanningOnlyNote         string         `json:"planning_only_note"`
	PlanningOnlyDispositions map[string]int `json:"planning_only_dispositions"`
}

type goldenManifest struct {
	Version     string                         `json:"version"`
	Generated   string                         `json:"generated"`
	Description string                         `json:"description"`
	Fixtures    map[string]goldenManifestEntry `json:"fixtures"`
	Summary     goldenManifestSummary          `json:"summary"`
}

func loadGoldenManifest(t *testing.T) goldenManifest {
	t.Helper()
	raw, err := os.ReadFile(goldenManifestJSONPath)
	if err != nil {
		t.Fatalf("read %s: %v", goldenManifestJSONPath, err)
	}
	var m goldenManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parse %s: %v", goldenManifestJSONPath, err)
	}
	if len(m.Fixtures) == 0 {
		t.Fatalf("%s carries no fixtures", goldenManifestJSONPath)
	}
	return m
}

// committedFixtureDirs returns every committed fixture directory as
// "<category>/<variant>", using the same definition as loadGoldenFixtures in
// internal/server: a direct child directory of the fixture root (canonical/
// excluded) whose direct-child directories hold a metadata.json. Test and
// manifest files at the root are not directories and never match.
func committedFixtureDirs(t *testing.T) []string {
	t.Helper()
	categories, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read fixture root: %v", err)
	}
	var dirs []string
	for _, cat := range categories {
		if !cat.IsDir() || cat.Name() == "canonical" || strings.HasPrefix(cat.Name(), ".") {
			continue
		}
		variants, err := os.ReadDir(cat.Name())
		if err != nil {
			t.Fatalf("read %s: %v", cat.Name(), err)
		}
		for _, variant := range variants {
			if !variant.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join(cat.Name(), variant.Name(), "metadata.json")); err != nil {
				continue // not a fixture directory
			}
			dirs = append(dirs, cat.Name()+"/"+variant.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

// onlyIn returns the elements of a that do not appear in b, in a's order.
func onlyIn(a, b []string) []string {
	var out []string
	for _, v := range a {
		if !slices.Contains(b, v) {
			out = append(out, v)
		}
	}
	return out
}

func TestGoldenManifestOnDiskDirsMatchTree(t *testing.T) {
	m := loadGoldenManifest(t)
	dirs := committedFixtureDirs(t)

	// Walk the manifest: collect its on-disk claims and validate each entry's
	// shape before comparing sets, so failures name the offending entry.
	manifestDirs := make([]string, 0, len(m.Fixtures))
	byDir := make(map[string]string, len(m.Fixtures))
	planningOnly := make([]string, 0, len(m.Fixtures))
	for name, e := range m.Fixtures {
		if e.DiskPath == "" {
			if e.PlanningOnly == nil || !*e.PlanningOnly {
				t.Errorf("entry %s has no disk_path but planning_only is not true", name)
			}
			planningOnly = append(planningOnly, name)
			continue
		}
		if e.PlanningOnly != nil && *e.PlanningOnly {
			t.Errorf("entry %s has disk_path %q but is marked planning_only", name, e.DiskPath)
		}
		if prev, dup := byDir[e.DiskPath]; dup {
			t.Errorf("entries %s and %s both claim disk_path %q", prev, name, e.DiskPath)
			continue
		}
		byDir[e.DiskPath] = name
		manifestDirs = append(manifestDirs, e.DiskPath)
		if _, err := os.Stat(filepath.Join(e.DiskPath, "metadata.json")); err != nil {
			t.Errorf("entry %s names disk_path %q, which is not a fixture directory on disk", name, e.DiskPath)
		}
		switch e.ExpectedOutcome {
		case "success", "failure":
		default:
			t.Errorf("entry %s (on disk): expected_outcome must be \"success\" or \"failure\", got %q", name, e.ExpectedOutcome)
		}
	}
	sort.Strings(manifestDirs)

	t.Run("on_disk_entries_equal_tree", func(t *testing.T) {
		if extra := onlyIn(manifestDirs, dirs); len(extra) > 0 {
			t.Errorf("manifest claims on-disk fixture dirs absent from the tree: %s", strings.Join(extra, ", "))
		}
		if missing := onlyIn(dirs, manifestDirs); len(missing) > 0 {
			t.Errorf("committed fixture dirs with no manifest entry carrying disk_path: %s", strings.Join(missing, ", "))
		}
	})

	t.Run("summary_on_disk_dirs_equal_tree", func(t *testing.T) {
		got := append([]string(nil), m.Summary.OnDiskDirs...)
		sort.Strings(got)
		if joined, want := strings.Join(got, "\n"), strings.Join(dirs, "\n"); joined != want {
			t.Fatalf("summary.on_disk_dirs differs from the committed tree\nonly in manifest: %s\nonly on disk: %s",
				strings.Join(onlyIn(got, dirs), ", "), strings.Join(onlyIn(dirs, got), ", "))
		}
	})

	t.Run("summary_counts_derived_from_lists", func(t *testing.T) {
		if m.Summary.TotalEntries != len(m.Fixtures) {
			t.Errorf("summary.total_entries = %d, manifest carries %d entries", m.Summary.TotalEntries, len(m.Fixtures))
		}
		if m.Summary.OnDiskFixtureDirs != len(dirs) {
			t.Errorf("summary.on_disk_fixture_dirs = %d, tree carries %d fixture dirs", m.Summary.OnDiskFixtureDirs, len(dirs))
		}
		if m.Summary.PlanningOnlyEntries != len(planningOnly) {
			t.Errorf("summary.planning_only_entries = %d, manifest carries %d rows without disk_path",
				m.Summary.PlanningOnlyEntries, len(planningOnly))
		}
		if total := m.Summary.OnDiskFixtureDirs + m.Summary.PlanningOnlyEntries; total != m.Summary.TotalEntries {
			t.Errorf("summary splits do not add up: %d on disk + %d planning-only != %d total",
				m.Summary.OnDiskFixtureDirs, m.Summary.PlanningOnlyEntries, m.Summary.TotalEntries)
		}
	})

	t.Run("summary_expected_outcomes_match_entries", func(t *testing.T) {
		success, failure := 0, 0
		for name, e := range m.Fixtures {
			if e.DiskPath == "" {
				continue
			}
			switch e.ExpectedOutcome {
			case "success":
				success++
			case "failure":
				failure++
			default:
				t.Errorf("entry %s (on disk): unparsable expected_outcome %q", name, e.ExpectedOutcome)
			}
		}
		if m.Summary.OnDiskExpectedOutcomes.Success != success || m.Summary.OnDiskExpectedOutcomes.Failure != failure {
			t.Errorf("summary.on_disk_expected_outcomes = {%d success, %d failure}, on-disk entries tally {%d, %d}",
				m.Summary.OnDiskExpectedOutcomes.Success, m.Summary.OnDiskExpectedOutcomes.Failure, success, failure)
		}
	})

	t.Run("planning_only_rows_have_dispositions", func(t *testing.T) {
		tallied := map[string]int{"covered": 0, "deferred": 0}
		for _, name := range planningOnly {
			e := m.Fixtures[name]
			switch e.Disposition {
			case "covered":
				tallied["covered"]++
				if e.CoveredBy == "" {
					t.Errorf("planning-only row %s: disposition \"covered\" needs covered_by", name)
					continue
				}
				if !slices.Contains(dirs, e.CoveredBy) {
					t.Errorf("planning-only row %s: covered_by names %q, which is not a committed fixture dir", name, e.CoveredBy)
				}
			case "deferred":
				tallied["deferred"]++
			default:
				t.Errorf("planning-only row %s: disposition must be \"covered\" or \"deferred\", got %q", name, e.Disposition)
				continue
			}
			if strings.TrimSpace(e.DispositionReason) == "" {
				t.Errorf("planning-only row %s: disposition_reason is empty", name)
			}
			if e.Disposition == "deferred" && !strings.Contains(e.DispositionReason, "Phase 8.11") {
				t.Errorf("planning-only row %s: deferred reason must state why the row is not required for Phase 8.11 acceptance", name)
			}
		}
		for _, kind := range []string{"covered", "deferred"} {
			if m.Summary.PlanningOnlyDispositions[kind] != tallied[kind] {
				t.Errorf("summary.planning_only_dispositions[%s] = %d, entries tally %d",
					kind, m.Summary.PlanningOnlyDispositions[kind], tallied[kind])
			}
		}
	})

	t.Run("note_names_every_planning_only_row", func(t *testing.T) {
		for _, name := range planningOnly {
			if !strings.Contains(m.Summary.PlanningOnlyNote, name) {
				t.Errorf("summary.planning_only_note does not name planning-only row %s", name)
			}
		}
	})
}

// TestGoldenManifestYAMLMatchesJSON fails if the committed .yml twin drifts
// from the .json. Both documents are normalized through encoding/json before
// comparison so int/float representation differences cannot false-positive.
func TestGoldenManifestYAMLMatchesJSON(t *testing.T) {
	jsonRaw, err := os.ReadFile(goldenManifestJSONPath)
	if err != nil {
		t.Fatalf("read %s: %v", goldenManifestJSONPath, err)
	}
	yamlRaw, err := os.ReadFile(goldenManifestYAMLPath)
	if err != nil {
		t.Fatalf("read %s: %v", goldenManifestYAMLPath, err)
	}

	var jsonDoc, yamlDoc any
	if err := json.Unmarshal(jsonRaw, &jsonDoc); err != nil {
		t.Fatalf("parse %s: %v", goldenManifestJSONPath, err)
	}
	if err := yaml.Unmarshal(yamlRaw, &yamlDoc); err != nil {
		t.Fatalf("parse %s: %v", goldenManifestYAMLPath, err)
	}
	jsonBytes, err := json.Marshal(jsonDoc)
	if err != nil {
		t.Fatal(err)
	}
	yamlBytes, err := json.Marshal(yamlDoc)
	if err != nil {
		t.Fatalf("normalize yaml document: %v", err)
	}

	if bytes.Equal(jsonBytes, yamlBytes) {
		return
	}

	// Name the divergence instead of dumping two full documents.
	var jsonNorm, yamlNorm map[string]any
	if err := json.Unmarshal(jsonBytes, &jsonNorm); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(yamlBytes, &yamlNorm); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "generated", "description", "fixtures", "summary"} {
		jv, yv := jsonNorm[key], yamlNorm[key]
		jb, _ := json.Marshal(jv)
		yb, _ := json.Marshal(yv)
		if !bytes.Equal(jb, yb) {
			if key == "fixtures" {
				t.Fatalf("%s drifted from %s in fixtures: %v",
					goldenManifestYAMLPath, goldenManifestJSONPath, firstDifferingKey(jv, yv))
			}
			t.Fatalf("%s drifted from %s in %q:\njson: %s\nyaml: %s",
				goldenManifestYAMLPath, goldenManifestJSONPath, key, truncate(string(jb)), truncate(string(yb)))
		}
	}
	t.Fatalf("%s drifted from %s", goldenManifestYAMLPath, goldenManifestJSONPath)
}

// firstDifferingKey returns the fixture names whose canonical JSON differs,
// plus a marker for names present on only one side.
func firstDifferingKey(jsonVal, yamlVal any) []string {
	jm, _ := jsonVal.(map[string]any)
	ym, _ := yamlVal.(map[string]any)
	var diffs []string
	for name := range jm {
		jb, _ := json.Marshal(jm[name])
		yb, _ := json.Marshal(ym[name])
		if !bytes.Equal(jb, yb) {
			diffs = append(diffs, name)
		}
	}
	for name := range ym {
		if _, ok := jm[name]; !ok {
			diffs = append(diffs, name+" (yaml only)")
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 5 {
		return append(diffs[:5], fmt.Sprintf("... and %d more", len(diffs)-5))
	}
	return diffs
}

func truncate(s string) string {
	const max = 400
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
