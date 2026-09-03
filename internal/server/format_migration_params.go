// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"fmt"
	"strconv"
	"strings"
)

// parseMigrationTarget normalizes the migration target parameter and validates
// it against the server's configured write version. The target is always
// explicit: an empty targetStr means "the configured write version", and any
// value that is present must name exactly that version, so the endpoint can
// never be driven toward a format the server does not itself write. Both the
// "v3" and "3" spellings are accepted.
func parseMigrationTarget(targetStr string, writeVersion uint8) (uint8, error) {
	if strings.TrimSpace(targetStr) == "" {
		return writeVersion, nil
	}
	target, err := parseVersionParam(targetStr)
	if err != nil {
		return 0, fmt.Errorf("invalid target version %q: expected v%d (or %d)", targetStr, writeVersion, writeVersion)
	}
	if target != writeVersion {
		return 0, fmt.Errorf("target version v%d does not match configured write version v%d", target, writeVersion)
	}
	return target, nil
}

// parseMigrationIncludeVersions normalizes the migration include (source
// version) list. Entries accept the "v1" or "1" spellings and must be strictly
// older than the target — include=v3 for a V3 target is rejected, as the plan's
// migration API contract requires. An empty list defaults to the previous major
// version (v2 for a V3 target, v1 for a V2 target). Duplicates are collapsed
// preserving first-occurrence order.
func parseMigrationIncludeVersions(includeStr string, writeVersion uint8) ([]string, error) {
	if strings.TrimSpace(includeStr) == "" {
		// The caller has already rejected write versions below v2, so the
		// default always names a real predecessor format.
		return []string{strconv.FormatUint(uint64(writeVersion-1), 10)}, nil
	}

	var (
		versions []string
		seen     = make(map[string]bool)
	)
	for _, part := range strings.Split(includeStr, ",") {
		version, err := parseVersionParam(part)
		if err != nil {
			return nil, fmt.Errorf("invalid include version %q: expected v1, v2, ... (comma-separated)", part)
		}
		if version >= writeVersion {
			return nil, fmt.Errorf("include version v%d is not compatible with target version v%d (source versions must be older than the target)", version, writeVersion)
		}
		v := strconv.FormatUint(uint64(version), 10)
		if seen[v] {
			continue
		}
		seen[v] = true
		versions = append(versions, v)
	}
	return versions, nil
}

// parseVersionParam parses a format-version parameter in either the "v2" or
// "2" form, tolerating surrounding whitespace and case. The result is bounded
// to [1, 255] so it can never wrap when held in a uint8.
func parseVersionParam(s string) (uint8, error) {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "v")
	version, err := strconv.Atoi(s)
	if err != nil || version < 1 || version > 255 {
		return 0, fmt.Errorf("not a format version: %q", s)
	}
	return uint8(version), nil
}
