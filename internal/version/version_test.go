package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// TestTextRendering checks the one-line form `armor version` prints:
//
//	armor <version> (go<goversion>, <os>/<arch>)
//
// The regression this guards is the doubled "go" prefix: runtime.Version()
// already begins with "go", so the format string must not add another.
func TestTextRendering(t *testing.T) {
	got := Text("armor")

	want := fmt.Sprintf("armor %s (%s, %s/%s)", Effective(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Errorf("Text(\"armor\") = %q, want %q", got, want)
	}
	if strings.Contains(got, "gogo") {
		t.Errorf("Text(\"armor\") = %q contains a doubled go prefix", got)
	}
	if !strings.HasPrefix(got, "armor ") {
		t.Errorf("Text(\"armor\") = %q does not start with the app name", got)
	}
}

// TestTextRenderingHonorsAppNameAndBuildVersion proves the build-time Version
// variable flows into the text rendering (it is injected via ldflags in
// `make build` and CI).
func TestTextRenderingHonorsAppNameAndBuildVersion(t *testing.T) {
	orig := Version
	defer func() { Version = orig }()
	Version = "0.1.1969"

	if got := Text("restore-verifier"); !strings.HasPrefix(got, "restore-verifier 0.1.1969 (") {
		t.Errorf("Text(\"restore-verifier\") = %q, want prefix %q", got, "restore-verifier 0.1.1969 (")
	}
}

// TestJSONRendering checks the --json form: a single-line object carrying
// every field the text form prints (app, version, go, os, arch) plus the
// envelope format version, in that key order. The commit/dirty fields are
// optional: the test binary carries VCS stamping only when built inside a
// git checkout, so the assertion requires the fixed prefix and allows any
// well-formed tail behind it.
func TestJSONRendering(t *testing.T) {
	got, err := JSON("armor", DefaultFormatWriteVersion)
	if err != nil {
		t.Fatalf("JSON(\"armor\", %d) returned error: %v", DefaultFormatWriteVersion, err)
	}

	want := fmt.Sprintf(`{"app":"armor","version":%q,"go":%q,"os":%q,"arch":%q,"format_write_version":%d`,
		Effective(), runtime.Version(), runtime.GOOS, runtime.GOARCH, DefaultFormatWriteVersion)
	if !strings.HasPrefix(got, want) {
		t.Errorf("JSON(\"armor\", %d) =\n  %s\nwant prefix\n  %s", DefaultFormatWriteVersion, got, want)
	}
	tail := strings.TrimPrefix(got, want)
	switch {
	case tail == "}":
	case strings.HasPrefix(tail, `,"commit":`), strings.HasPrefix(tail, `,"dirty":`):
	default:
		t.Errorf("JSON(\"armor\", %d) = %q has unexpected tail %q after the fixed fields", DefaultFormatWriteVersion, got, tail)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("JSON output spans multiple lines: %q", got)
	}
	if strings.Contains(got, "gogo") {
		t.Errorf("JSON output = %q contains a doubled go prefix", got)
	}

	// Round-trip so consumers parsing the object get the same values.
	var info Info
	if err := json.Unmarshal([]byte(got), &info); err != nil {
		t.Fatalf("JSON output does not parse: %v", err)
	}
	if info.App != "armor" || info.Version != Effective() || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		t.Errorf("round-trip lost a text-form field: %+v", info)
	}
	if !strings.HasPrefix(info.Go, "go") {
		t.Errorf("round-trip go field = %q, want the runtime's go-prefixed version", info.Go)
	}
	if info.FormatWriteVersion != DefaultFormatWriteVersion {
		t.Errorf("round-trip format_write_version = %d, want %d", info.FormatWriteVersion, DefaultFormatWriteVersion)
	}
}

// TestJSONRenderingPassesThroughFormatWriteVersion checks the configured
// write format (ARMOR_FORMAT_VERSION=2, legacy) is reported, not just the
// default.
func TestJSONRenderingPassesThroughFormatWriteVersion(t *testing.T) {
	got, err := JSON("armor", 2)
	if err != nil {
		t.Fatalf("JSON(\"armor\", 2) returned error: %v", err)
	}
	if !strings.Contains(got, `"format_write_version":2`) {
		t.Errorf("JSON(\"armor\", 2) = %q, want format_write_version 2", got)
	}
}

// withBuildInfo swaps the BuildInfo source for the duration of f so tests can
// inject the exact BuildInfo a go-install or VCS-stamped build would carry.
func withBuildInfo(t *testing.T, bi *debug.BuildInfo, ok bool, f func()) {
	t.Helper()
	origInfo, origVersion := buildInfo, Version
	defer func() { buildInfo, Version = origInfo, origVersion }()
	buildInfo = func() (*debug.BuildInfo, bool) { return bi, ok }
	Version = "dev"
	f()
}

// TestEffectiveFallsBackToBuildInfo covers the motivating case: a binary
// built by `go install github.com/jedarden/armor/cmd/armor@v0.1.1970` carries
// no ldflags version, so Effective falls back to the module version the
// toolchain records, stripped of its "v" to match the VERSION file format.
func TestEffectiveFallsBackToBuildInfo(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1970"}}, true, func() {
		if got := Effective(); got != "0.1.1970" {
			t.Errorf("Effective() = %q, want %q", got, "0.1.1970")
		}
		if got, want := Text("armor"), "armor 0.1.1970 ("; !strings.HasPrefix(got, want) {
			t.Errorf("Text(\"armor\") = %q, want prefix %q", got, want)
		}
		got, err := JSON("armor", DefaultFormatWriteVersion)
		if err != nil {
			t.Fatalf("JSON(\"armor\", %d) returned error: %v", DefaultFormatWriteVersion, err)
		}
		if !strings.Contains(got, `"version":"0.1.1970"`) {
			t.Errorf("JSON output = %q, want version 0.1.1970", got)
		}
	})
}

// TestEffectiveIgnoresDevelAndMissingBuildInfo pins the development-build
// behavior: a plain `go build` in a module directory records "(devel)", some
// contexts record no version at all, and a binary compiled without build
// info returns ok=false. None of these may be reported as a version.
func TestEffectiveIgnoresDevelAndMissingBuildInfo(t *testing.T) {
	cases := []struct {
		name string
		bi   *debug.BuildInfo
		ok   bool
	}{
		{"devel marker", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, true},
		{"empty version", &debug.BuildInfo{Main: debug.Module{Version: ""}}, true},
		{"no build info", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withBuildInfo(t, tc.bi, tc.ok, func() {
				if got := Effective(); got != "dev" {
					t.Errorf("Effective() = %q, want %q", got, "dev")
				}
			})
		})
	}
}

// TestEffectiveKeepsLdflagsPrecedence proves an ldflags-injected version
// wins: `make build` and CI set Version through -X and the fallback must
// never override it even when build info carries a module version.
func TestEffectiveKeepsLdflagsPrecedence(t *testing.T) {
	withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1970"}}, true, func() {
		Version = "0.1.1969"
		if got := Effective(); got != "0.1.1969" {
			t.Errorf("Effective() = %q, want the ldflags value %q", got, "0.1.1969")
		}
	})
}

// TestJSONSurfacesVCSInfo checks the optional vcs.revision / vcs.modified
// reporting: present settings reach the JSON object as commit/dirty, and a
// build without VCS stamping (proxy downloads build from the module zip)
// omits both keys.
func TestJSONSurfacesVCSInfo(t *testing.T) {
	t.Run("stamped", func(t *testing.T) {
		bi := &debug.BuildInfo{
			Main: debug.Module{Version: "v0.1.1970"},
			Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "3f39c3f9abc"},
				{Key: "vcs.modified", Value: "true"},
			},
		}
		withBuildInfo(t, bi, true, func() {
			got, err := JSON("armor", DefaultFormatWriteVersion)
			if err != nil {
				t.Fatalf("JSON(\"armor\", %d) returned error: %v", DefaultFormatWriteVersion, err)
			}
			if !strings.Contains(got, `"commit":"3f39c3f9abc"`) {
				t.Errorf("JSON output = %q, want commit 3f39c3f9abc", got)
			}
			if !strings.Contains(got, `"dirty":true`) {
				t.Errorf("JSON output = %q, want dirty true", got)
			}
		})
	})
	t.Run("unstamped", func(t *testing.T) {
		withBuildInfo(t, &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1970"}}, true, func() {
			got, err := JSON("armor", DefaultFormatWriteVersion)
			if err != nil {
				t.Fatalf("JSON(\"armor\", %d) returned error: %v", DefaultFormatWriteVersion, err)
			}
			if strings.Contains(got, "commit") || strings.Contains(got, "dirty") {
				t.Errorf("JSON output = %q, want no commit/dirty keys without VCS stamping", got)
			}
		})
	})
}
