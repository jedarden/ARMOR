// Package version provides version information for ARMOR binaries.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is set at build time via ldflags:
//
//	-X github.com/jedarden/armor/internal/version.Version=<version>
//
// It defaults to "dev" when not set (e.g., during development). Reporting
// surfaces read Effective, which falls back to the module version the go
// toolchain records at link time — so `go install
// github.com/jedarden/armor/cmd/armor@v0.1.x` builds, which cannot apply
// -ldflags, report the installed version instead of "dev".
var Version = "dev"

// buildInfo is the BuildInfo source Effective reads when Version is unset.
// It is a variable so tests can inject a BuildInfo without building a real
// module.
var buildInfo = debug.ReadBuildInfo

// DefaultFormatWriteVersion is the envelope format version written for new
// objects when ARMOR_FORMAT_VERSION is unset. It mirrors the default applied
// by internal/config; the version subcommand reports it without calling
// config.Load, which requires server credentials.
const DefaultFormatWriteVersion = 3

// Effective returns the version the binary reports everywhere it prints one:
// the ldflags-injected value when the build set one, otherwise the main
// module version recorded by the toolchain. A development build (no ldflags,
// Main.Version "(devel)" or empty) still reports "dev".
func Effective() string {
	if Version != "dev" {
		return Version
	}
	bi, ok := buildInfo()
	if !ok {
		return "dev"
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return strings.TrimPrefix(v, "v")
	}
	return "dev"
}

// vcsStatus returns the VCS state the go command stamped into the binary.
// Both values are absent for proxy downloads (go install module@version
// builds from the module zip) and for builds outside a VCS checkout.
func vcsStatus() (commit string, dirty bool) {
	bi, ok := buildInfo()
	if !ok {
		return "", false
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return commit, dirty
}

// Text renders version information in the format:
//
//	armor <version> (go<version>, <os>/<arch>)
//
// runtime.Version already carries the "go" prefix ("go1.25.0"), so it is
// printed as-is; adding another prefix produced "gogo1.25.0".
func Text(appName string) string {
	return fmt.Sprintf("%s %s (%s, %s/%s)", appName, Effective(), runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Print prints the text rendering of version information to stdout.
func Print(appName string) {
	fmt.Println(Text(appName))
}

// Info is the machine-readable rendering of the version output: every field
// the text form prints (app, version, go, os, arch) plus the envelope format
// version the README documents as reported by `armor version`. Commit and
// Dirty carry the VCS state of the build when the go command stamped one and
// are omitted otherwise.
type Info struct {
	App                string `json:"app"`
	Version            string `json:"version"`
	Go                 string `json:"go"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	FormatWriteVersion int    `json:"format_write_version"`
	Commit             string `json:"commit,omitempty"`
	Dirty              bool   `json:"dirty,omitempty"`
}

// JSON renders version information as a single-line JSON object.
func JSON(appName string, formatWriteVersion int) (string, error) {
	commit, dirty := vcsStatus()
	b, err := json.Marshal(Info{
		App:                appName,
		Version:            Effective(),
		Go:                 runtime.Version(),
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		FormatWriteVersion: formatWriteVersion,
		Commit:             commit,
		Dirty:              dirty,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
