package version

import (
	"encoding/json"
	"fmt"
	"runtime"
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

	want := fmt.Sprintf("armor %s (%s, %s/%s)", Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
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
// envelope format version, in that key order.
func TestJSONRendering(t *testing.T) {
	got, err := JSON("armor", DefaultFormatWriteVersion)
	if err != nil {
		t.Fatalf("JSON(\"armor\", %d) returned error: %v", DefaultFormatWriteVersion, err)
	}

	want := fmt.Sprintf(`{"app":"armor","version":%q,"go":%q,"os":%q,"arch":%q,"format_write_version":%d}`,
		Version, runtime.Version(), runtime.GOOS, runtime.GOARCH, DefaultFormatWriteVersion)
	if got != want {
		t.Errorf("JSON(\"armor\", %d) =\n  %s\nwant\n  %s", DefaultFormatWriteVersion, got, want)
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
	if info.App != "armor" || info.Version != Version || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
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
