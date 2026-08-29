// Package middleware provides HTTP middleware for ARMOR.
package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/version"
)

// ServerHeader returns middleware that adds a Server: ARMOR/<version> header to all responses.
// This header is used by the drift check and fleet console to identify ARMOR versions.
func ServerHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add Server header to response
		w.Header().Set("Server", fmt.Sprintf("ARMOR/%s", version.Version))
		next.ServeHTTP(w, r)
	})
}

// VersionHandler returns an HTTP handler that returns version information.
// The endpoint requires no authentication and returns JSON with version, format_write_version, and go.
func VersionHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Extract Go version (strip "go" prefix for cleaner JSON)
		goVersion := strings.TrimPrefix(runtime.Version(), "go")

		fmt.Fprintf(w, `{"version":"%s","format_write_version":%d,"go":"%s"}`,
			version.Version, cfg.FormatWriteVersion, goVersion)
	}
}
