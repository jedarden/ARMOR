// Package version provides version information for ARMOR binaries.
package version

import (
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags:
//
//	-X github.com/jedarden/armor/internal/version.Version=<version>
//
// It defaults to "dev" when not set (e.g., during development).
var Version = "dev"

// Print prints version information in the format:
//
//	armor <version> (go<version>, <os>/<arch>)
//
// runtime.Version already carries the "go" prefix ("go1.25.0"), so it is
// printed as-is; adding another prefix produced "gogo1.25.0".
func Print(appName string) {
	fmt.Printf("%s %s (%s, %s/%s)\n", appName, Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
