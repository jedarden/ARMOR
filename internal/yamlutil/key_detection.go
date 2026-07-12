// Package yamlutil provides basic colon-based key detection functions.
//
// These functions provide simple YAML mapping key detection based on colon presence.
// They are designed for basic use cases where sophisticated key validation is not required.
package yamlutil

import (
	"strings"
)

// IsMappingKey checks if a line contains a YAML mapping key based on colon presence.
//
// This is a basic function that simply checks if a line contains a colon (':').
// It does not perform sophisticated validation of whether the colon is at a valid
// position or whether the key follows YAML naming conventions.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line contains a colon, false otherwise.
//
// Examples:
//   - IsMappingKey("name: John") → true
//   - IsMappingKey("first name: John") → true
//   - IsMappingKey("just some text") → false
//   - IsMappingKey("") → false
func IsMappingKey(line string) bool {
	return strings.Contains(line, ":")
}

// ExtractKey extracts the mapping key text from a line before the colon.
//
// This is a basic function that returns the text before the first colon in a line.
// If no colon is found, it returns an empty string. The returned key includes leading
// and trailing whitespace as-is (no trimming is performed).
//
// Parameters:
//   - line: The line content to extract the key from
//
// Returns the text before the colon, or empty string if no colon is found.
//
// Examples:
//   - ExtractKey("name: John") → "name"
//   - ExtractKey("first name: John") → "first name"
//   - ExtractKey("just some text") → ""
//   - ExtractKey("") → ""
//   - ExtractKey(":") → ""
//   - ExtractKey("  key  : value") → "  key  "
func ExtractKey(line string) string {
	colonPos := strings.Index(line, ":")
	if colonPos <= 0 {
		return ""
	}
	return line[:colonPos]
}
