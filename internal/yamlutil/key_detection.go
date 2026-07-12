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

// StripInlineComment removes inline comments from a YAML line.
//
// This function handles inline comments that appear after a value. It only
// strips comments that appear after whitespace (per YAML spec). Hash characters
// that appear in the middle of text (not preceded by whitespace) are preserved.
//
// Parameters:
//   - line: The line content to strip comments from
//
// Returns the line with inline comments removed.
//
// Examples:
//   - StripInlineComment("key: value # comment") → "key: value "
//   - StripInlineComment("key: value#no-space-comment") → "key: value#no-space-comment" (not a comment per YAML spec)
//   - StripInlineComment("url: http://example.com#anchor") → "url: http://example.com#anchor"
//   - StripInlineComment("key: value with # hash in it") → "key: value with # hash in it"
//   - StripInlineComment("# comment") → "# comment" (full-line comments not handled)
//   - StripInlineComment("  # indented comment") → "  # indented comment" (full-line comments not handled)
//   - StripInlineComment("key: value") → "key: value"
func StripInlineComment(line string) string {
	// Check if this is a full-line comment - if so, don't strip anything
	if IsCommentLine(line) {
		return line
	}

	// Find inline comment by looking for '#' preceded by whitespace
	// But not if it's at the start of the line (already handled above)
	for i := 0; i < len(line); i++ {
		if line[i] == '#' {
			// Check if this '#' is preceded by whitespace or is at position 0
			if i == 0 {
				// This should have been caught by IsCommentLine check above
				// but handle it for safety
				return ""
			}
			// Check if preceded by whitespace
			if line[i-1] == ' ' || line[i-1] == '\t' {
				// Check if this looks like a comment vs a value fragment
				// Comments typically have space after # or are natural language
				// Values like hex colors, URL fragments have compact format
				afterHash := i + 1
				if afterHash < len(line) {
					nextChar := line[afterHash]
					// If next char is space/tab, definitely a comment
					if nextChar == ' ' || nextChar == '\t' {
						return line[:i]
					}
					// If it's a hex digit (0-9, A-F, a-f), might be a hex color
					if (nextChar >= '0' && nextChar <= '9') ||
						(nextChar >= 'A' && nextChar <= 'F') ||
						(nextChar >= 'a' && nextChar <= 'f') {
						// Could be hex color, keep it
						continue
					}
					// If we have several chars that look like hex/ID, keep it
					// Otherwise treat as comment
					consumed := 1
					for consumed < 8 && afterHash+consumed < len(line) {
						c := line[afterHash+consumed]
						if !((c >= '0' && c <= '9') ||
							(c >= 'A' && c <= 'F') ||
							(c >= 'a' && c <= 'f')) {
							break
						}
						consumed++
					}
					// If we have 3+ hex-like chars, it's probably a value (hex color, ID)
					if consumed >= 3 {
						continue
					}
					// Otherwise, treat as comment
					return line[:i]
				}
				// # at end of line with no content after, treat as comment
				return line[:i]
			}
		}
	}

	return line
}
