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
// This function first filters out comment lines, then checks if the line contains
// a colon (':'). It does not perform sophisticated validation of whether the colon
// is at a valid position or whether the key follows YAML naming conventions.
//
// Parameters:
//   - line: The line content to check
//
// Returns true if the line contains a colon and is not a comment, false otherwise.
//
// Examples:
//   - IsMappingKey("name: John") → true
//   - IsMappingKey("first name: John") → true
//   - IsMappingKey("just some text") → false
//   - IsMappingKey("") → false
//   - IsMappingKey("# This is a comment") → false (comment line)
//   - IsMappingKey("  # indented comment") → false (comment line)
//   - IsMappingKey("# TODO: fix this") → false (comment line even with colon)
func IsMappingKey(line string) bool {
	// Skip full-line comments
	if IsCommentLine(line) {
		return false
	}
	return strings.Contains(line, ":")
}

// ExtractKey extracts the mapping key text from a line before the colon.
//
// This function first strips inline comments from the line, then returns the text
// before the first colon. If no colon is found, it returns an empty string.
// The returned key includes leading and trailing whitespace as-is (no trimming
// is performed on the key itself).
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
//   - ExtractKey("key: value # comment") → "key: value " (comment stripped first)
//   - ExtractKey("url: http://example.com#anchor") → "url: http://example.com#anchor" (hash preserved)
func ExtractKey(line string) string {
	// Strip inline comments before processing
	stripped := StripInlineComment(line)

	colonPos := strings.Index(stripped, ":")
	if colonPos <= 0 {
		return ""
	}
	return stripped[:colonPos]
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
	// Check if this is a full-line comment
	trimmed := strings.TrimLeft(line, " \t")
	if len(trimmed) > 0 && trimmed[0] == '#' {
		// If it's just "#" or "# " with no actual comment content, strip it
		if len(trimmed) == 1 || (len(trimmed) > 1 && (trimmed[1] == ' ' || trimmed[1] == '\t')) {
			afterHash := strings.TrimLeft(trimmed[1:], " \t")
			if len(afterHash) == 0 {
				return "" // Empty comment, strip it completely
			}
		}
		// Full-line comment with content, preserve it
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
				// Values like hex colors, URL fragments, IDs have compact format
				afterHash := i + 1
				if afterHash < len(line) {
					nextChar := line[afterHash]
					// If next char is space/tab, definitely a comment
					if nextChar == ' ' || nextChar == '\t' {
						return line[:i]
					}
					// Check if this looks like a value (not a comment)
					// Values starting with # typically have alphanumeric chars immediately after
					// Common patterns:
					// - Hex colors: #FFF, #FF0000
					// - CSS IDs: #main-content, #sidebar
					// - Hashtags: #golang, #yaml
					// - Symbol refs: #define, #include
					isAlphanumeric := (nextChar >= '0' && nextChar <= '9') ||
						(nextChar >= 'A' && nextChar <= 'Z') ||
						(nextChar >= 'a' && nextChar <= 'z') ||
						nextChar == '_' || nextChar == '-'

					if isAlphanumeric {
						// Check how many alphanumeric/underscore/hyphen chars follow
						// If we have 1+ such chars, it's likely a value
						consumed := 1
						for consumed < 32 && afterHash+consumed < len(line) {
							c := line[afterHash+consumed]
							isValidValueChar := (c >= '0' && c <= '9') ||
								(c >= 'A' && c <= 'Z') ||
								(c >= 'a' && c <= 'z') ||
								c == '_' || c == '-' || c == '.'

							if !isValidValueChar {
								break
							}
							consumed++
						}
						// If we have 1+ chars that look like an ID/tag/hex, preserve it
						// This handles cases like #1, #a, #fff, etc.
						if consumed >= 1 {
							continue
						}
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

// IndentationContext tracks the indentation state for validating mapping keys.
//
// IndentationContext maintains a stack of indentation levels encountered while
// processing YAML content, allowing validation of proper nesting and indentation
// consistency for mapping keys.
type IndentationContext struct {
	parentLevels   []int // Stack of parent indentation levels
	spacesPerLevel int   // Expected spaces per indentation level
	lastLevel      int   // Last seen indentation level
	seenKeys       bool  // Whether we've seen any mapping keys yet
}

// NewIndentationContext creates a new indentation context.
//
// Parameters:
//   - spacesPerLevel: Expected number of spaces per indentation level (default: 2)
//
// Returns an IndentationContext ready to track indentation levels.
func NewIndentationContext(spacesPerLevel int) *IndentationContext {
	if spacesPerLevel <= 0 {
		spacesPerLevel = 2 // Default to 2 spaces
	}
	return &IndentationContext{
		parentLevels:   make([]int, 0),
		spacesPerLevel: spacesPerLevel,
		lastLevel:      -1,
		seenKeys:       false,
	}
}

// ValidateMappingKeyIndent validates if a mapping key has valid indentation.
//
// This function checks if a mapping key line has proper indentation relative to
// the current context. It validates:
// - Indentation is not mixed (tabs and spaces)
// - Indentation is consistent with parent context
// - Empty lines are skipped
//
// Parameters:
//   - line: The line content to validate
//   - isMappingKey: Whether the line contains a mapping key
//
// Returns true if the indentation is valid, false otherwise.
//
// Validation rules:
// - Top-level keys (indentation 0) are always valid
// - Nested keys must have indentation exactly one level deeper than parent
// - Empty lines are skipped (return true but don't update context)
// - Mixed indentation (tabs and spaces) is invalid
// - Indentation must be a multiple of expected spaces per level
func (ic *IndentationContext) ValidateMappingKeyIndent(line string, isMappingKey bool) bool {
	// Skip empty lines and comments
	if IsBlankLine(line) || IsCommentLine(line) {
		return true
	}

	if !isMappingKey {
		return true // Only validate mapping keys
	}

	info := CalculateIndentation(line, ic.spacesPerLevel)

	// Check for mixed indentation
	if info.IsMixed {
		return false
	}

	// Check if indentation is a multiple of expected unit (for space-based indentation)
	if info.IndentType == "space" && info.SpaceCount > 0 {
		if info.SpaceCount % ic.spacesPerLevel != 0 {
			return false
		}
	}

	currentLevel := info.Level

	// First key - establish baseline
	if !ic.seenKeys {
		ic.seenKeys = true
		ic.lastLevel = currentLevel

		// If this is a nested key (level > 0), add implicit level 0 as parent
		if currentLevel > 0 {
			ic.parentLevels = append(ic.parentLevels, 0)
		}

		return true
	}

	// Check if this is a valid transition
	if ic.isValidLevelTransition(currentLevel) {
		ic.updateContext(currentLevel)
		return true
	}

	return false
}

// isValidLevelTransition checks if moving to a new level is valid.
//
// Parameters:
//   - newLevel: The new indentation level
//
// Returns true if the transition is valid, false otherwise.
func (ic *IndentationContext) isValidLevelTransition(newLevel int) bool {
	if !ic.seenKeys {
		return true // First key is always valid
	}

	// Same level is always valid
	if newLevel == ic.lastLevel {
		return true
	}

	// Can only go one level deeper
	if newLevel == ic.lastLevel+1 {
		return true
	}

	// Can return to any previous parent level
	if newLevel < ic.lastLevel {
		// Check if this level exists in parent stack
		for _, level := range ic.parentLevels {
			if level == newLevel {
				return true
			}
		}
	}

	return false
}

// updateContext updates the context after a valid level transition.
//
// Parameters:
//   - newLevel: The new indentation level
func (ic *IndentationContext) updateContext(newLevel int) {
	if newLevel > ic.lastLevel {
		// Going deeper - add current level to parent stack
		ic.parentLevels = append(ic.parentLevels, ic.lastLevel)
	} else if newLevel < ic.lastLevel {
		// Coming back up - remove levels from parent stack
		for len(ic.parentLevels) > 0 && ic.parentLevels[len(ic.parentLevels)-1] != newLevel {
			ic.parentLevels = ic.parentLevels[:len(ic.parentLevels)-1]
		}
	}

	ic.lastLevel = newLevel
}

// GetCurrentLevel returns the current indentation level.
//
// Returns the last seen indentation level, or -1 if no keys have been seen.
func (ic *IndentationContext) GetCurrentLevel() int {
	return ic.lastLevel
}

// GetParentLevels returns the stack of parent indentation levels.
//
// Returns a copy of the parent levels stack.
func (ic *IndentationContext) GetParentLevels() []int {
	parentCopy := make([]int, len(ic.parentLevels))
	copy(parentCopy, ic.parentLevels)
	return parentCopy
}

// Reset resets the context to initial state.
func (ic *IndentationContext) Reset() {
	ic.parentLevels = make([]int, 0)
	ic.lastLevel = -1
	ic.seenKeys = false
}

// GetIndentationLevel extracts the indentation level from a line.
//
// This is a convenience function that calculates the indentation level
// for a line. It uses a default of 2 spaces per level for space-based
// indentation, and counts tabs directly for tab-based indentation.
//
// Parameters:
//   - line: The line content to analyze
//
// Returns the indentation level (0 for no indent, 1 for first level, etc.)
//
// Examples:
//   - GetIndentationLevel("key: value") → 0
//   - GetIndentationLevel("  key: value") → 1 (with 2 spaces per level)
//   - GetIndentationLevel("    key: value") → 2 (with 2 spaces per level)
//   - GetIndentationLevel("\tkey: value") → 1 (with tab-based indent)
func GetIndentationLevel(line string) int {
	info := CalculateIndentation(line, 2) // Default to 2 spaces per level
	return info.Level
}

// ValidateMappingKeyIndentLine validates mapping key indentation on a single line.
//
// This function combines indentation extraction and validation in one call.
// It checks if the line has proper indentation for a mapping key.
//
// Parameters:
//   - line: The line content to validate
//   - spacesPerLevel: Expected spaces per indentation level (0 to auto-detect)
//
// Returns true if the line has valid indentation for a mapping key, false otherwise.
//
// This function checks:
// - Line is not empty or a comment
// - Line contains a mapping key (has colon)
// - Indentation is not mixed
// - Indentation follows proper level structure
func ValidateMappingKeyIndentLine(line string, spacesPerLevel int) bool {
	if spacesPerLevel <= 0 {
		spacesPerLevel = 2 // Default to 2 spaces
	}

	// Skip empty lines and comments
	if IsBlankLine(line) || IsCommentLine(line) {
		return true
	}

	// Check if line contains a mapping key
	if !IsMappingKey(line) {
		return true // Only validate mapping keys
	}

	info := CalculateIndentation(line, spacesPerLevel)

	// Check for mixed indentation
	if info.IsMixed {
		return false
	}

	// Check if indentation is a multiple of expected unit
	if info.SpaceCount > 0 && info.SpaceCount%spacesPerLevel != 0 {
		return false
	}

	// For tab indentation, just check it's not mixed
	return true
}

// ValidateKeyIndentationSequence validates a sequence of mapping key lines.
//
// This function processes multiple lines and validates that the indentation
// sequence is consistent and follows proper nesting rules.
//
// Parameters:
//   - lines: Slice of lines to validate
//   - spacesPerLevel: Expected spaces per indentation level (0 to auto-detect)
//
// Returns true if all mapping keys have valid indentation, false otherwise.
//
// This function validates:
// - Each mapping key has proper indentation
// - Indentation transitions are valid (one level at a time)
// - Parent context is maintained throughout
func ValidateKeyIndentationSequence(lines []string, spacesPerLevel int) bool {
	context := NewIndentationContext(spacesPerLevel)

	for _, line := range lines {
		// Check if this line contains a mapping key
		isKey := IsMappingKey(line) && !IsCommentLine(line) && !IsBlankLine(line)

		if !context.ValidateMappingKeyIndent(line, isKey) {
			return false
		}
	}

	return true
}
