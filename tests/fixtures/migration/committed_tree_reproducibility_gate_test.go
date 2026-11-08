// Clean-clone reproducibility gate over the WHOLE committed fixture tree
// (armor-8a72436e).
//
// TestCommittedRegenerationSetMatchesGenerator pins generated_fixtures/ only;
// nothing proved that the fixture directories outside it (contradictory/,
// v1_multipart/, v1_single_put/, v2_multipart/, v2_single_put/) still match
// what the generator emits -- nine of them had silently drifted at HEAD
// before the 2026-09-23 reconciliation. This gate runs the committed
// standalone_generator.go main() the way an operator regeneration run does
// (`go run standalone_generator.go <dir>`) and holds the committed tree
// against that output:
//
//   - every fixture directory the generator emits in a category the tree
//     carries must exist in the tree (a generated dir missing from the tree
//     fails), and
//   - every committed fixture directory must be produced by the generator (a
//     committed dir the generator no longer produces fails), and
//   - every matched directory pair -- plus generated_fixtures/manifest.json
//     -- must be byte-identical; the failure names the first differing path
//     and its byte offset.
//
// malformed/ and edge_cases/ are the documented exception: the generator
// emits both, but no fixture from either category has ever been committed
// (README.md, "The committed set"), so their absence from the tree is not
// drift. If either category ever lands, move it out of
// uncommittedCategories in the same change.
//
// Determinism is a precondition proven elsewhere, not assumed here:
// TestRegenerationSetIsDeterministic and TestMalformedSetIsDeterministic pin
// it at unit scale. If a nondeterministic field ever appears in generator
// output, fix the generator -- this gate does no normalisation, or it would
// silently bless whatever the last run produced.

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// generatorSource is the committed generator that must reproduce the tree,
// relative to this package directory (the working directory of every test in
// the package).
const generatorSource = "standalone_generator.go"

const legacyCiphertextHashesFile = "legacy_ciphertext_hashes.json"

type legacyCiphertextHash struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type legacyCiphertextHashes struct {
	Generator string                          `json:"generator"`
	Fixtures  map[string]legacyCiphertextHash `json:"fixtures"`
}

func loadLegacyCiphertextHashes(t *testing.T) legacyCiphertextHashes {
	t.Helper()
	raw, err := os.ReadFile(legacyCiphertextHashesFile)
	if err != nil {
		t.Fatalf("read %s: %v", legacyCiphertextHashesFile, err)
	}
	var pins legacyCiphertextHashes
	if err := json.Unmarshal(raw, &pins); err != nil {
		t.Fatalf("parse %s: %v", legacyCiphertextHashesFile, err)
	}
	if pins.Generator != generatorSource {
		t.Fatalf("%s generator = %q, want %q", legacyCiphertextHashesFile, pins.Generator, generatorSource)
	}
	if len(pins.Fixtures) != 6 {
		t.Fatalf("%s pins %d fixtures, want 6", legacyCiphertextHashesFile, len(pins.Fixtures))
	}
	return pins
}

func verifyLegacyCiphertextHashes(t *testing.T, generatedRoot string, pins legacyCiphertextHashes) error {
	t.Helper()
	for rel, pin := range pins.Fixtures {
		data, err := os.ReadFile(filepath.Join(generatedRoot, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("read generated %s: %w", rel, err)
		}
		if got, want := int64(len(data)), pin.Bytes; got != want {
			return fmt.Errorf("%s has %d bytes, want %d", rel, got, want)
		}
		sum := sha256.Sum256(data)
		if got, want := hex.EncodeToString(sum[:]), pin.SHA256; got != want {
			return fmt.Errorf("%s sha256 = %s, want %s", rel, got, want)
		}
	}
	return nil
}

func isPinnedLegacyCiphertext(rel string, pins legacyCiphertextHashes) bool {
	_, ok := pins.Fixtures[filepath.ToSlash(rel)]
	return ok
}

// uncommittedCategories holds the generator's categories whose absence from
// the committed tree is documented policy, not drift.
var uncommittedCategories = map[string]bool{
	"edge_cases": true,
	"malformed":  true,
}

// runGeneratorMain executes the committed generator's main() into a fresh
// directory, via `go run` of the source file itself, and returns the
// directory. Running the file -- not a re-called copy of its call sequence --
// is what makes this a gate over the generator as committed.
func runGeneratorMain(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(generatorSource); err != nil {
		t.Fatalf("generator source %s not found: %v (tests must run from the %s package directory)",
			generatorSource, err, filepath.Dir(generatorSource))
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("no go toolchain found to run the generator: %v", err)
	}
	outDir := t.TempDir()
	cmd := exec.Command(goBin, "run", generatorSource, outDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run %s failed: %v\n%s", generatorSource, err, out)
	}
	return outDir
}

// fixtureDirs returns every fixture directory under root as slash-separated
// <category>/<variant> paths. A fixture directory is one directly holding a
// metadata.json -- the shape of every generator-written and committed
// fixture, and the discriminator that keeps source-only directories
// (canonical/) out of the comparison.
func fixtureDirs(t *testing.T, root string) map[string]bool {
	t.Helper()
	dirs := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "metadata.json")); statErr == nil {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			dirs[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return dirs
}

// relFiles returns every regular file under root as sorted slash-separated
// relative paths.
func relFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(files)
	return files
}

// firstByteDiff returns the offset of the first byte at which a and b differ
// (for equal prefixes of different lengths, the shorter length), and whether
// they differ at all.
func firstByteDiff(a, b []byte) (int, bool) {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i, true
		}
	}
	if len(a) != len(b) {
		return n, true
	}
	return -1, false
}

// requireSameBytes byte-compares one generated/committed file pair and fails
// on the first difference, naming the path and its byte offset.
func requireSameBytes(t *testing.T, generatedRoot, committedRoot, rel string) {
	t.Helper()
	generated, err := os.ReadFile(filepath.Join(generatedRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("generated %s: %v", rel, err)
	}
	committed, err := os.ReadFile(filepath.Join(committedRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("committed %s: %v", rel, err)
	}
	if offset, differs := firstByteDiff(generated, committed); differs {
		t.Fatalf("%s differs from generator output at byte offset %d (generated %d bytes, committed %d bytes); regenerate and commit instead of editing either side by hand",
			rel, offset, len(generated), len(committed))
	}
}

// requireSameFixtureDir holds one generated/committed fixture-directory pair
// to exactly equal file sets and bytes, first difference fatal.
func requireSameFixtureDir(t *testing.T, generatedRoot, committedRoot, rel string) {
	t.Helper()
	pins := loadLegacyCiphertextHashes(t)
	generated := relFiles(t, filepath.Join(generatedRoot, filepath.FromSlash(rel)))
	committed := relFiles(t, filepath.Join(committedRoot, filepath.FromSlash(rel)))
	for _, file := range generated {
		if !slices.Contains(committed, file) {
			if isPinnedLegacyCiphertext(rel+"/"+file, pins) {
				continue
			}
			t.Fatalf("%s/%s: generated file is missing from the committed tree", rel, file)
		}
	}
	for _, file := range committed {
		if !slices.Contains(generated, file) {
			t.Fatalf("%s/%s: committed file is not produced by the generator", rel, file)
		}
	}
	for _, file := range committed {
		requireSameBytes(t, generatedRoot, committedRoot, rel+"/"+file)
	}
}

// TestCommittedFixtureTreeMatchesGenerator is the clean-clone gate: the
// committed fixture tree must be exactly the output of the committed
// generator, in both directions and byte for byte.
func TestCommittedFixtureTreeMatchesGenerator(t *testing.T) {
	generated := runGeneratorMain(t)
	pins := loadLegacyCiphertextHashes(t)
	if err := verifyLegacyCiphertextHashes(t, generated, pins); err != nil {
		t.Fatal(err)
	}

	generatedDirs := fixtureDirs(t, generated)
	committedDirs := fixtureDirs(t, ".")
	if len(committedDirs) == 0 {
		t.Fatal("no committed fixture directories found; the gate has nothing to compare (tests must run from the tests/fixtures/migration package directory)")
	}
	if len(generatedDirs) == 0 {
		t.Fatalf("the generator run produced no fixture directories in %s", generated)
	}

	// Set drift, generated -> committed.
	for _, rel := range slices.Sorted(maps.Keys(generatedDirs)) {
		if uncommittedCategories[strings.SplitN(rel, "/", 2)[0]] {
			continue
		}
		if !committedDirs[rel] {
			t.Fatalf("generated fixture %s is missing from the committed tree; regenerate and commit it", rel)
		}
	}
	// Set drift, committed -> generated.
	for _, rel := range slices.Sorted(maps.Keys(committedDirs)) {
		if !generatedDirs[rel] {
			t.Fatalf("committed fixture %s is not produced by the generator; remove the stale directory or restore the generator call that emits it", rel)
		}
	}

	// Byte drift, one directory pair at a time, first difference fatal.
	for _, rel := range slices.Sorted(maps.Keys(committedDirs)) {
		requireSameFixtureDir(t, generated, ".", rel)
	}
	requireSameBytes(t, generated, ".", "generated_fixtures/manifest.json")
}

// TestPinnedLegacyCiphertextsAreGeneratedNotCommitted keeps the large
// ciphertexts out of the repository while the fixture-tree gate above still
// checks every generated byte against a repository-pinned digest.
func TestPinnedLegacyCiphertextsAreGeneratedNotCommitted(t *testing.T) {
	pins := loadLegacyCiphertextHashes(t)
	for rel := range pins.Fixtures {
		if _, err := os.Stat(filepath.FromSlash(rel)); err == nil {
			t.Fatalf("%s is committed; keep this large ciphertext test-time generated", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", rel, err)
		}
	}
}

// TestPinnedLegacyCiphertextTamperIsDetected proves that changing one
// generated ciphertext byte cannot pass the pinned golden-vector check.
func TestPinnedLegacyCiphertextTamperIsDetected(t *testing.T) {
	generated := runGeneratorMain(t)
	pins := loadLegacyCiphertextHashes(t)
	const rel = "v1_multipart/uniform_parts/stored_ciphertext.bin"
	path := filepath.Join(generated, filepath.FromSlash(rel))
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	data[len(data)/2] ^= 1
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("tamper %s: %v", rel, err)
	}
	if err := verifyLegacyCiphertextHashes(t, generated, pins); err == nil {
		t.Fatal("pinned legacy ciphertext verification accepted a one-byte mutation")
	}
}
