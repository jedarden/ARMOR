// Package version provides version information for ARMOR binaries.
package version

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags:
//
//	-X github.com/jedarden/armor/internal/version.Version=<version>
//
// It defaults to "dev" when not set (e.g., during development).
var Version = "dev"

// DefaultFormatWriteVersion is the envelope format version written for new
// objects when ARMOR_FORMAT_VERSION is unset. It mirrors the default applied
// by internal/config; the version subcommand reports it without calling
// config.Load, which requires server credentials.
const DefaultFormatWriteVersion = 3

// Text renders version information in the format:
//
//	armor <version> (go<version>, <os>/<arch>)
//
// runtime.Version already carries the "go" prefix ("go1.25.0"), so it is
// printed as-is; adding another prefix produced "gogo1.25.0".
func Text(appName string) string {
	return fmt.Sprintf("%s %s (%s, %s/%s)", appName, Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// Print prints the text rendering of version information to stdout.
func Print(appName string) {
	fmt.Println(Text(appName))
}

// Info is the machine-readable rendering of the version output: every field
// the text form prints (app, version, go, os, arch) plus the envelope format
// version the README documents as reported by `armor version`.
type Info struct {
	App                string `json:"app"`
	Version            string `json:"version"`
	Go                 string `json:"go"`
	OS                 string `json:"os"`
	Arch               string `json:"arch"`
	FormatWriteVersion int    `json:"format_write_version"`
}

// JSON renders version information as a single-line JSON object.
func JSON(appName string, formatWriteVersion int) (string, error) {
	b, err := json.Marshal(Info{
		App:                appName,
		Version:            Version,
		Go:                 runtime.Version(),
		OS:                 runtime.GOOS,
		Arch:               runtime.GOARCH,
		FormatWriteVersion: formatWriteVersion,
	})
	if err != nil {
		return "", err
	}
	return string(b), nil
}
