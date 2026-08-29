// Package version provides version information for ARMOR binaries.
package version

import (
	"fmt"
	"runtime"
)

// Version is set at build time via ldflags: -X main.version=<version>
// It defaults to "dev" when not set (e.g., during development).
var Version = "dev"

// Print prints version information in the format:
//   armor <version> (go<version>, <os>/<arch>)
func Print(appName string) {
	fmt.Printf("%s %s (go%s, %s/%s)\n", appName, Version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
